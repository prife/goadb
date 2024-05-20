package wire

import (
	"fmt"
	"io"
	"strconv"
)

// TODO(zach): All EOF errors returned from networoking calls should use ConnectionResetError.

// StatusCodes are returned by the server. If the code indicates failure, the
// next message will be the error.
const (
	StatusSuccess  string = "OKAY"
	StatusFailure         = "FAIL"
	StatusSyncData        = "DATA"
	StatusSyncDone        = "DONE"
	StatusNone            = ""
)

func isFailureStatus(status string) bool {
	return status == StatusFailure
}

type StatusReader interface {
	// Reads a 4-byte status string and returns it.
	// If the status string is StatusFailure, reads the error message from the server
	// and returns it as an AdbError.
	ReadStatus(req string) (string, error)
}

// Scanner reads tokens from a server.
// See Conn for more details.
type Scanner interface {
	io.Closer
	StatusReader
	ReadMessage() ([]byte, error)
	ReadUntilEof() ([]byte, error)
	// NewSyncScanner() SyncScanner
}

// Reads the status, and if failure, reads the message and returns it as an error.
// If the status is success, doesn't read the message.
// req is just used to populate the AdbError, and can be nil.
func readStatusFailureAsError(r io.Reader, buf []byte, req string) (string, error) {
	// read 4 bytes
	if len(buf) < 4 {
		buf = make([]byte, 4)
	}
	n, err := io.ReadFull(r, buf[:4])
	if err == io.ErrUnexpectedEOF {
		return "", fmt.Errorf("error reading status for %s: %w", req, errIncompleteMessage(req, n, 4))
	} else if err != nil {
		return "", fmt.Errorf("error reading status for %s: %w", req, err)
	}

	status := string(buf[:4])
	if isFailureStatus(status) {
		msg, err := readMessage(r, buf)
		if err != nil {
			return "", fmt.Errorf("server returned error for %s, but couldn't read the error message, %w", req, err)
		}

		return "", adbServerError(req, string(msg))
	}

	return status, nil
}

// func (s *realScanner) NewSyncScanner() SyncScanner {
// 	return NewSyncScanner(s.reader)
// }

// readMessage reads a 4-byte hex string from r, then reads length bytes and returns them.
func readMessage(r io.Reader, buf []byte) ([]byte, error) {
	// read 4 bytes
	if len(buf) < 4 {
		buf = make([]byte, 4)
	}
	length, err := readHexLength(r, buf)
	if err != nil {
		return nil, err
	}

	// read length buf
	if int(length) > len(buf) {
		buf = make([]byte, length)
	}
	n, err := io.ReadFull(r, buf[:length])
	if err == io.ErrUnexpectedEOF {
		return buf[:n], errIncompleteMessage("message data", n, int(length))
	} else if err != nil {
		return buf[:n], fmt.Errorf("error reading message data: %w", err)
	}
	return buf[:n], nil
}

// readHexLength reads the next 4 bytes from r as an ASCII hex-encoded length and parses them into an int.
func readHexLength(r io.Reader, buf []byte) (int, error) {
	n, err := io.ReadFull(r, buf)
	if err != nil {
		return 0, errIncompleteMessage("length", n, 4)
	}

	length, err := strconv.ParseInt(string(buf[:n]), 16, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: could not parse hex length:%v", ErrAssertion, buf[:n])
	}
	return int(length), nil
}
