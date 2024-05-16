package adb

import (
	"fmt"
	"strconv"

	"github.com/prife/goadb/wire"
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
func (c *Adb) Dial() (*wire.Conn, error) {
	return c.server.Dial()
}

// Starts the adb server if it’s not running.
func (c *Adb) StartServer() error {
	return c.server.Start()
}

func (c *Adb) Device(descriptor DeviceDescriptor) *Device {
	return &Device{
		server:         c.server,
		descriptor:     descriptor,
		deviceListFunc: c.ListDevices,
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
//	adb connect
func (c *Adb) Connect(host string, port int) error {
	_, err := roundTripSingleResponse(c.server, fmt.Sprintf("host:connect:%s:%d", host, port))
	if err != nil {
		return fmt.Errorf("Connect: %w", err)
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
