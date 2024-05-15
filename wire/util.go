package wire

import (
	// "errors"
	"fmt"
	"io"
	"regexp"
	"sync"
)

// ErrorResponseDetails is an error message returned by the server for a particular request.
type ErrorResponseDetails struct {
	Request   string
	ServerMsg string
}

// deviceNotFoundMessagePattern matches all possible error messages returned by adb servers to
// report that a matching device was not found. Used to set the DeviceNotFound error code on
// error values.
//
// Old servers send "device not found", and newer ones "device 'serial' not found".
var deviceNotFoundMessagePattern = regexp.MustCompile(`device( '.*')? not found`)

func adbServerError(request string, serverMsg string) error {
	if deviceNotFoundMessagePattern.MatchString(serverMsg) {
		return fmt.Errorf("%w: request %s, server error: %s", ErrDeviceNotFound, request, serverMsg)
	}
	return fmt.Errorf("%w: request %s, server error: %s", ErrAdb, request, serverMsg)
}

func errIncompleteMessage(description string, actual int, expected int) error {
	return fmt.Errorf("%w: incomplete %s: read %d bytes, expecting %d", ErrConnectionReset, description, actual, expected)
}

// MultiCloseable wraps c in a ReadWriteCloser that can be safely closed multiple times.
func MultiCloseable(c io.ReadWriteCloser) io.ReadWriteCloser {
	return &multiCloseable{ReadWriteCloser: c}
}

type multiCloseable struct {
	io.ReadWriteCloser
	closeOnce sync.Once
	err       error
}

func (c *multiCloseable) Close() error {
	c.closeOnce.Do(func() {
		c.err = c.ReadWriteCloser.Close()
	})
	return c.err
}
