package adb

import (
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prife/goadb/wire"
)

// MtimeOfClose should be passed to OpenWrite to set the file modification time to the time the Close
// method is called.
var MtimeOfClose = time.Time{}

// Device communicates with a specific Android device.
// To get an instance, call Device() on an Adb.
type Device struct {
	server     server
	descriptor DeviceDescriptor

	// Used to get device info.
	deviceListFunc func() ([]*DeviceInfo, error)
	deviceFeatures map[string]bool
}

const (
	FeatureShell2                    = "shell_v2"
	FeatureCmd                       = "cmd"
	FeatureStat2                     = "stat_v2"
	FeatureLs2                       = "ls_v2"
	FeatureLibusb                    = "libusb"
	FeaturePushSync                  = "push_sync"
	FeatureApex                      = "apex"
	FeatureFixedPushMkdir            = "fixed_push_mkdir"
	FeatureAbb                       = "abb"
	FeatureFixedPushSymlinkTimestamp = "fixed_push_symlink_timestamp"
	FeatureAbbExec                   = "abb_exec"
	FeatureRemountShell              = "remount_shell"
	//track_app
	//sendrecv_v2
	//sendrecv_v2_brotli
	//sendrecv_v2_lz4
	//sendrecv_v2_zstd
	//sendrecv_v2_dry_run_send
	//openscreen_mdns
	//push_sync
)

func (c *Device) String() string {
	return c.descriptor.String()
}

func (c *Device) Serial() (string, error) {
	attr, err := c.getAttribute("get-serialno")
	return attr, wrapClientError(err, c, "Serial")
}

func (c *Device) DevicePath() (string, error) {
	attr, err := c.getAttribute("get-devpath")
	return attr, wrapClientError(err, c, "DevicePath")
}

func (c *Device) DeviceFeatures() (features map[string]bool, err error) {
	attr, err := c.getAttribute("features")
	if err != nil {
		return nil, wrapClientError(err, c, "features")
	}
	features = featuresStrToMap(attr)
	return
}

func (c *Device) State() (DeviceState, error) {
	attr, err := c.getAttribute("get-state")
	if err != nil {
		if strings.Contains(err.Error(), "unauthorized") {
			return StateUnauthorized, nil
		}
		return StateInvalid, wrapClientError(err, c, "State")
	}
	state, err := parseDeviceState(attr)
	return state, wrapClientError(err, c, "State")
}

func (c *Device) DeviceInfo() (*DeviceInfo, error) {
	// Adb doesn't actually provide a way to get this for an individual device,
	// so we have to just list devices and find ourselves.

	serial, err := c.Serial()
	if err != nil {
		return nil, wrapClientError(err, c, "GetDeviceInfo(GetSerial)")
	}

	devices, err := c.deviceListFunc()
	if err != nil {
		return nil, wrapClientError(err, c, "DeviceInfo(ListDevices)")
	}

	for _, deviceInfo := range devices {
		if deviceInfo.Serial == serial {
			return deviceInfo, nil
		}
	}

	err = fmt.Errorf("%w: device list doesn't contain serial %s", wire.ErrDeviceNotFound, serial)
	return nil, wrapClientError(err, c, "DeviceInfo")
}

// Forward create a tcp connection to remote addr in android device
// forward [--no-rebind] LOCAL REMOTE
// forward socket connection using:
//
//	tcp:<port> (<local> may be "tcp:0" to pick any open port)
//	localabstract:<unix domain socket name>
//	localreserved:<unix domain socket name>
//	localfilesystem:<unix domain socket name>
//	dev:<character device name>
//	jdwp:<process pid> (remote only)
//	vsock:<CID>:<port> (remote only)
//	acceptfd:<fd> (listen only)
func (c *Device) ForwardPort(port int) (net.Conn, error) {
	return c.Forward("tcp:" + strconv.Itoa(port))
}

func (c *Device) ForwardAbstract(name string) (net.Conn, error) {
	return c.Forward("localabstract:" + name)
}

func (c *Device) Forward(addr string) (net.Conn, error) {
	conn, err := c.dialDevice()
	if err != nil {
		return nil, wrapClientError(err, c, "forward")
	}
	if err = conn.SendMessage([]byte(addr)); err != nil {
		conn.Close()
		return nil, wrapClientError(err, c, "forward")
	}
	if _, err = conn.ReadStatus(addr); err != nil {
		conn.Close()
		return nil, wrapClientError(err, c, "forward")
	}

	return conn.(*wire.Conn), wrapClientError(err, c, "forward")
}

// Remount, from the official adb command’s docs:
//
//	Ask adbd to remount the device's filesystem in read-write mode,
//	instead of read-only. This is usually necessary before performing
//	an "adb sync" or "adb push" request.
//	This request may not succeed on certain builds which do not allow
//	that.
//
// Source: https://android.googlesource.com/platform/system/core/+/master/adb/SERVICES.TXT
func (c *Device) Remount() (string, error) {
	conn, err := c.dialDevice()
	if err != nil {
		return "", wrapClientError(err, c, "Remount")
	}
	defer conn.Close()

	resp, err := conn.RoundTripSingleResponse([]byte("remount"))
	return string(resp), wrapClientError(err, c, "Remount")
}

func (c *Device) ListDirEntries(path string) (*DirEntries, error) {
	conn, err := c.getSyncConn()
	if err != nil {
		return nil, wrapClientError(err, c, "ListDirEntries(%s)", path)
	}

	entries, err := conn.List(path)
	return entries, wrapClientError(err, c, "ListDirEntries(%s)", path)
}

func (c *Device) Stat(path string) (*DirEntry, error) {
	conn, err := c.getSyncConn()
	if err != nil {
		return nil, wrapClientError(err, c, "Stat(%s)", path)
	}
	defer conn.Close()

	entry, err := conn.Stat(path)
	return entry, wrapClientError(err, c, "Stat(%s)", path)
}

func (c *Device) OpenRead(path string) (io.ReadCloser, error) {
	conn, err := c.getSyncConn()
	if err != nil {
		return nil, wrapClientError(err, c, "OpenRead(%s)", path)
	}

	reader, err := conn.Recv(path)
	return reader, wrapClientError(err, c, "OpenRead(%s)", path)
}

// OpenWrite opens the file at path on the device, creating it with the permissions specified
// by perms if necessary, and returns a writer that writes to the file.
// The files modification time will be set to mtime when the WriterCloser is closed. The zero value
// is TimeOfClose, which will use the time the Close method is called as the modification time.
func (c *Device) OpenWrite(path string, perms os.FileMode, mtime time.Time) (io.WriteCloser, error) {
	conn, err := c.getSyncConn()
	if err != nil {
		return nil, wrapClientError(err, c, "OpenWrite(%s)", path)
	}

	writer, err := conn.Send(path, perms, mtime)
	return writer, wrapClientError(err, c, "OpenWrite(%s)", path)
}

// getAttribute returns the first message returned by the server by running
// <host-prefix>:<attr>, where host-prefix is determined from the DeviceDescriptor.
func (c *Device) getAttribute(attr string) (string, error) {
	resp, err := roundTripSingleResponse(c.server,
		fmt.Sprintf("%s:%s", c.descriptor.getHostPrefix(), attr))
	if err != nil {
		return "", err
	}
	return string(resp), nil
}

func (c *Device) getSyncConn() (*FileService, error) {
	conn, err := c.dialDevice()
	if err != nil {
		return nil, err
	}

	// Switch the connection to sync mode.
	if err := conn.SendMessage([]byte("sync:")); err != nil {
		return nil, err
	}
	if _, err := conn.ReadStatus("sync"); err != nil {
		return nil, err
	}

	// FIXME: refactor in soon
	return &FileService{SyncConn: conn.(*wire.Conn).NewSyncConn()}, nil
}

func (c *Device) NewFileService() (*FileService, error) {
	return c.getSyncConn()
}

// dialDevice switches the connection to communicate directly with the device
// by requesting the transport defined by the DeviceDescriptor.
func (c *Device) dialDevice() (wire.IConn, error) {
	conn, err := c.server.Dial()
	if err != nil {
		return nil, err
	}

	req := fmt.Sprintf("host:%s", c.descriptor.getTransportDescriptor())
	if err = conn.SendMessage([]byte(req)); err != nil {
		conn.Close()
		return nil, fmt.Errorf("error connecting to device '%s': %w", c.descriptor, err)
	}

	if _, err = conn.ReadStatus(req); err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}

// prepareCommandLine validates the command and argument strings, quotes
// arguments if required, and joins them into a valid adb command string.
func prepareCommandLine(cmd string, args ...string) (string, error) {
	if isBlank(cmd) {
		return "", fmt.Errorf("%w: command cannot be empty", wire.ErrAssertion)
	}

	for i, arg := range args {
		if strings.ContainsRune(arg, '"') {
			return "", fmt.Errorf("%w: arg at index %d contains an invalid double quote: %s", wire.ErrParse, i, arg)
		}
		if containsWhitespace(arg) {
			args[i] = fmt.Sprintf("\"%s\"", arg)
		}
	}

	// Prepend the command to the args array.
	if len(args) > 0 {
		cmd = fmt.Sprintf("%s %s", cmd, strings.Join(args, " "))
	}

	return cmd, nil
}
