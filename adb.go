package adb

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/prife/goadb/wire"
)

const (
	CommandTimeoutShortDefault = time.Second * 2
	CommandTimeoutLongDefault  = time.Second * 30
)

// Adb communicates with host services on the adb server.
// Eg.
//
//	client := adb.New()
//	client.ListDevices()
//
// See list of services at https://android.googlesource.com/platform/system/core/+/master/adb/SERVICES.TXT.
// TODO(z): Finish implementing host services.
type Adb struct {
	server server
}

// New creates a new Adb client that uses the default ServerConfig.
func New() (*Adb, error) {
	return NewWithConfig(ServerConfig{})
}

func NewWithConfig(config ServerConfig) (*Adb, error) {
	server, err := newServer(config)
	if err != nil {
		return nil, err
	}
	return &Adb{server}, nil
}

// Dial establishes a connection with the adb server.
func (c *Adb) Dial() (wire.IConn, error) {
	return c.server.Dial()
}

// Starts the adb server if it’s not running.
func (c *Adb) StartServer() error {
	return c.server.Start()
}

func (c *Adb) Device(descriptor DeviceDescriptor) *Device {
	return &Device{
		server:          c.server,
		descriptor:      descriptor,
		deviceListFunc:  c.ListDevices,
		CmdTimeoutShort: CommandTimeoutShortDefault,
		CmdTimeoutLong:  CommandTimeoutLongDefault,
	}
}

func (c *Adb) NewDeviceWatcher() *DeviceWatcher {
	return newDeviceWatcher(c.server)
}

// ServerVersion asks the ADB server for its internal version number.
func (c *Adb) ServerVersion() (int, error) {
	resp, err := roundTripSingleResponse(c.server, "host:version")
	if err != nil {
		return 0, fmt.Errorf("GetServerVersion: %w", err)
	}

	version, err := c.parseServerVersion(resp)
	if err != nil {
		return 0, fmt.Errorf("GetServerVersion: %w", err)
	}
	return version, nil
}

func (c *Adb) HostFeatures() (map[string]bool, error) {
	resp, err := roundTripSingleResponse(c.server, "host:host-features")
	if err != nil {
		return nil, err
	}
	return featuresStrToMap(string(resp)), nil
}

// KillServer tells the server to quit immediately.
// Corresponds to the command:
//
//	adb kill-server
func (c *Adb) KillServer() error {
	conn, err := c.server.Dial()
	if err != nil {
		return fmt.Errorf("KillServer: %w", err)
	}
	defer conn.Close()

	if err = conn.SendMessage([]byte("host:kill")); err != nil {
		return fmt.Errorf("KillServer: %w", err)
	}
	return nil
}

// ListDeviceSerials returns the serial numbers of all attached devices.
// Corresponds to the command:
//
//	adb devices
func (c *Adb) ListDeviceSerials() ([]string, error) {
	resp, err := roundTripSingleResponse(c.server, "host:devices")
	if err != nil {
		return nil, fmt.Errorf("ListDeviceSerials: %w", err)
	}

	devices, err := parseDeviceList(string(resp), parseDeviceShort)
	if err != nil {
		return nil, fmt.Errorf("ListDeviceSerials: %w", err)
	}

	serials := make([]string, len(devices))
	for i, dev := range devices {
		serials[i] = dev.Serial
	}
	return serials, nil
}

// ListDevices returns the list of connected devices.
// Corresponds to the command:
//
//	adb devices -l
func (c *Adb) ListDevices() ([]*DeviceInfo, error) {
	resp, err := roundTripSingleResponse(c.server, "host:devices-l")
	if err != nil {
		return nil, fmt.Errorf("ListDevices: %w", err)
	}

	devices, err := parseDeviceList(string(resp), parseDeviceLongE)
	if err != nil {
		return nil, fmt.Errorf("ListDevices: %w", err)
	}
	return devices, nil
}

// Connect connect to a device via TCP/IP
// Corresponds to the command:
//
//	adb connect ip:port
func (c *Adb) Connect(addr string) error {
	// connect may slow in internet, set 5 second timeout
	_, err := roundTripSingleResponseTimeout(c.server, "host:connect:"+addr, time.Second*5)
	if err != nil {
		return fmt.Errorf("Connect: %w", err)
	}
	return nil
}

func (c *Adb) DisconnectAll() error {
	_, err := roundTripSingleResponse(c.server, "host:disconnect:")
	if err != nil {
		return fmt.Errorf("disconnect: %w", err)
	}
	return nil
}

func (c *Adb) Disconnect(addr string) error {
	_, err := roundTripSingleResponse(c.server, "host:disconnect:"+addr)
	if err != nil {
		return fmt.Errorf("disconnect: %w", err)
	}
	return nil
}

func (c *Adb) ListForward() ([]ForwardEntry, error) {
	resp, err := roundTripSingleResponse(c.server, "host:list-forward")
	if err != nil {
		return nil, err
	}
	return parseForwardList(resp), nil
}

// RemoveAllForward
// --->
// 00000000  30 30 31 34 68 6f 73 74  3a 6b 69 6c 6c 66 6f 72  |0014host:killfor|
// 00000010  77 61 72 64 2d 61 6c 6c                           |ward-all|
// <---
// 00000000  4f 4b 41 59 4f 4b 41 59                           |OKAYOKAY|
func (c *Adb) RemoveAllForward() (err error) {
	conn, err := c.server.Dial()
	if err != nil {
		return
	}
	defer conn.Close()

	// 这里没有使用 roundTripSingleResponse，因为它返回 OKAY之后，后面还跟 OKAY
	req := "host:killforward-all"
	if err = conn.SendMessage([]byte(req)); err != nil {
		return err
	}

	if _, err = readStatusWithTimeout(conn, req, CommandTimeoutShortDefault); err != nil {
		return fmt.Errorf("'%s' failed: %w", req, err)
	}
	return nil
}

func (c *Adb) parseServerVersion(versionRaw []byte) (int, error) {
	versionStr := string(versionRaw)
	version, err := strconv.ParseInt(versionStr, 16, 32)
	if err != nil {
		return 0, fmt.Errorf("error parsing server version: %s", versionStr)
	}
	return int(version), nil
}

func featuresStrToMap(attr string) (features map[string]bool) {
	lists := strings.Split(attr, ",")
	if len(lists) == 0 {
		return
	}
	features = make(map[string]bool)
	for _, f := range lists {
		features[f] = true
	}
	return
}

type ForwardEntry struct {
	Serial string
	Local  string
	Remote string
}

func parseForwardList(resp []byte) []ForwardEntry {
	lines := bytes.Split(resp, []byte("\n"))
	deviceForward := make([]ForwardEntry, 0, len(lines))

	for i := range lines {
		line := bytes.TrimSpace(lines[i])
		if len(line) == 0 {
			continue
		}
		fields := bytes.Fields(line)
		deviceForward = append(deviceForward, ForwardEntry{Serial: string(fields[0]), Local: string(fields[1]), Remote: string(fields[2])})
	}
	return deviceForward
}
