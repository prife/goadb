package wire

import (
	"fmt"
	"io"
	"net"
	"strconv"
)

const (
	// The official implementation of adb imposes an undocumented 255-byte limit
	// on messages.
	MaxMessageLength = 255
	// Chunks cannot be longer than 64k.
	SyncMaxChunkSize = 64 * 1024

	// StatusCodes are returned by the server. If the code indicates failure, the next message will be the error.
	StatusSuccess string = "OKAY"
	StatusFailure string = "FAIL"
)

type StatusReader interface {
	// Reads a 4-byte status string and returns it.
	// If the status string is StatusFailure, reads the error message from the server
	// and returns it as an AdbError.
	ReadStatus(req string) (string, error)
}

// Sender sends messages to the server.
type Sender interface {
	SendMessage(msg []byte) error
}

// Scanner reads tokens from a server.
type Scanner interface {
	io.Closer
	StatusReader
	ReadMessage() ([]byte, error)
	ReadUntilEof() ([]byte, error)
	// NewSyncScanner() SyncScanner
}

type IConn interface {
	net.Conn
	Sender
	Scanner
	RoundTripSingleResponse(req []byte) (resp []byte, err error)
}

// Conn is a normal connection to an adb server.
// For most cases, usage looks something like:
//
//	conn := wire.Dial()
//	conn.SendMessage(data)
//	conn.ReadStatus() == StatusSuccess || StatusFailure
//	conn.ReadMessage()
//	conn.Close()
//
// For some messages, the server will return more than one message (but still a single
// status). Generally, after calling ReadStatus once, you should call ReadMessage until
// it returns an io.EOF error. Note: the protocol docs seem to suggest that connections will be
// kept open for multiple commands, but this is not the case. The official client closes
// a connection immediately after its read the response, in most cases. The docs might be
// referring to the connection between the adb server and the device, but I haven't confirmed
// that.
// For most commands, the server will close the connection after sending the response.
// You should still always call Close() when you're done with the connection.
type Conn struct {
	net.Conn
	rbuf []byte
}

func NewConn(conn net.Conn) *Conn {
	return &Conn{
		Conn: conn,
		rbuf: make([]byte, 4),
	}
}

var _ Sender = &Conn{}
var _ Scanner = &Conn{}

// NewSyncConn returns connection that can operate in sync mode.
// The connection must already have been switched (by sending the sync command
// to a specific device), or the return connection will return an error.
func (c *Conn) NewSyncConn() *SyncConn {
	return &SyncConn{c.Conn, make([]byte, 8), make([]byte, 8)}
}

func (s *Conn) SendMessage(msg []byte) error {
	if len(msg) > MaxMessageLength {
		return fmt.Errorf("message length exceeds maximum:%d", len(msg))
	}

	// FIXME: when message is very large, if cost heavy
	lengthAndMsg := fmt.Sprintf("%04x%s", len(msg), msg)
	_, err := s.Write([]byte(lengthAndMsg))
	return err
}

// RoundTripSingleResponse sends a message to the server, and reads a single
// message response. If the reponse has a failure status code, returns it as an error.
func (conn *Conn) RoundTripSingleResponse(req []byte) (resp []byte, err error) {
	if err = conn.SendMessage(req); err != nil {
		return nil, err
	}

	if _, err = conn.ReadStatus(string(req)); err != nil {
		return nil, err
	}

	return conn.ReadMessage()
}

func (s *Conn) ReadStatus(req string) (string, error) {
	return readStatusFailureAsError(s, s.rbuf, req)
}

func (s *Conn) ReadMessage() ([]byte, error) {
	return readMessage(s, s.rbuf)
}

func (s *Conn) ReadUntilEof() ([]byte, error) {
	data, err := io.ReadAll(s)
	if err != nil {
		return nil, fmt.Errorf("error reading until EOF: %w", err)
	}
	return data, nil
}

func (conn *Conn) Close() error {
	if err := conn.Conn.Close(); err != nil {
		return fmt.Errorf("error closing connection: %w", err)
	}
	return nil
}

// TODO(zach): All EOF errors returned from networoking calls should use ConnectionResetError.
func isFailureStatus(status string) bool {
	return status == StatusFailure
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
