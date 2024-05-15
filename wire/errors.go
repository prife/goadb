package wire

import (
	"errors"
)

var (
	ErrAssertion = errors.New("AssertionError")
	ErrParse     = errors.New("ParseError")
	// ErrServerNotAvailable the server was not available on the requested port.
	ErrServerNotAvailable = errors.New("ServerNotAvailable")
	// ErrNetwork general network error communicating with the server.
	ErrNetwork = errors.New("Network")
	// ErrConnectionReset the connection to the server was reset in the middle of an operation. Server probably died.
	ErrConnectionReset = errors.New("ConnectionReset")
	// ErrAdb The server returned an error message, but we couldn't parse it.
	ErrAdb = errors.New("AdbError")
	// ErrDeviceNotFound the server returned a "device not found" error.
	ErrDeviceNotFound = errors.New("DeviceNotFound")
	// ErrFileNoExist tried to perform an operation on a path that doesn't exist on the device.
	ErrFileNoExist = errors.New("FileNoExist")
)
