package wire

import (
	"fmt"
	"io"
	"regexp"
	"sync"

	"github.com/prife/goadb/internal/errors"
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

// IsAdbServerErrorMatching returns true if err is an *Err with code AdbError and for which
// predicate returns true when passed Details.ServerMsg.
func IsAdbServerErrorMatching(err error, predicate func(string) bool) bool {
	if err, ok := err.(*errors.Err); ok && err.Code == errors.AdbError {
		return predicate(err.Details.(ErrorResponseDetails).ServerMsg)
	}
	return false
}

func errIncompleteMessage(description string, actual int, expected int) error {
	return &errors.Err{
		Code:    errors.ConnectionResetError,
		Message: fmt.Sprintf("incomplete %s: read %d bytes, expecting %d", description, actual, expected),
		Details: struct {
			ActualReadBytes int
			ExpectedBytes   int
		}{
			ActualReadBytes: actual,
			ExpectedBytes:   expected,
		},
	}
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
