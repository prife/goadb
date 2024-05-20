package wire

import (
	"fmt"
	"io"
	"net"
)

const (
	// The official implementation of adb imposes an undocumented 255-byte limit
	// on messages.
	MaxMessageLength = 255
	// Chunks cannot be longer than 64k.
	SyncMaxChunkSize = 1024 * 1024
)

type IConn interface {
	io.Closer
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
	return &SyncConn{
		// SyncScanner: c.Scanner.NewSyncScanner(),
		// SyncSender:  c.Sender.NewSyncSender(),
	}
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

// SyncConn is a connection to the adb server in sync mode.
// Assumes the connection has been put into sync mode (by sending "sync" in transport mode).
// The adb sync protocol is defined at
// https://android.googlesource.com/platform/system/core/+/master/adb/SYNC.TXT.
// Unlike the normal adb protocol (implemented in Conn), the sync protocol is binary.
// Lengths are binary-encoded (little-endian) instead of hex.
// Length headers and other integers are encoded in little-endian, with 32 bits.
// File mode seems to be encoded as POSIX file mode.
// Modification time seems to be the Unix timestamp format, i.e. seconds since Epoch UTC.
type SyncConn struct {
	SyncScanner
	SyncSender
}

// Close closes both the sender and the scanner, and returns any errors.
func (c SyncConn) Close() error {
	senderErr := c.SyncSender.Close()
	scannerErr := c.SyncScanner.Close()
	if senderErr != nil || scannerErr != nil {
		return fmt.Errorf("error closing SyncConn: %w, %w", senderErr, scannerErr)
	}
	return nil
}
