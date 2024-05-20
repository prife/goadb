package wire

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"
)

// SyncConn is a connection to the adb server in sync mode.
// Assumes the connection has been put into sync mode (by sending "sync" in transport mode).
// The adb sync protocol is defined at
// https://android.googlesource.com/platform/system/core/+/master/adb/SYNC.TXT.
// Unlike the normal adb protocol (implemented in Conn), the sync protocol is binary.
// Lengths are binary-encoded (little-endian) instead of hex.
// Length headers and other integers are encoded in little-endian, with 32 bits.
// File mode seems to be encoded as POSIX file mode.
// Modification time seems to be the Unix timestamp format, i.e. seconds since Epoch UTC.
type ISyncConn interface {
	io.Closer
	StatusReader
	ReadInt32() (int32, error)
	ReadFileMode() (os.FileMode, error)
	ReadTime() (time.Time, error)

	// Reads an octet length, followed by length bytes.
	ReadString() (string, error)
	// Reads an octet length, and then the length of bytes.
	ReadBytes([]byte) ([]byte, error)

	// SendOctetString sends a 4-byte string.
	SendOctetString(string) error
	SendInt32(int32) error
	SendFileMode(os.FileMode) error
	SendTime(time.Time) error
	// Sends len(data) as an octet, followed by the bytes.
	// If data is bigger than SyncMaxChunkSize, it returns an assertion error.
	SendBytes(data []byte) error
}

type SyncConn struct {
	net.Conn
	rbuf []byte
	wbuf []byte
}

func NewSyncConn(r net.Conn) *SyncConn {
	return &SyncConn{r, make([]byte, 4), make([]byte, 4)}
}

// ReadStatus reads a 4-byte status string and returns it.
func (s *SyncConn) ReadStatus(req string) (string, error) {
	return readSyncStatusFailureAsError(s, s.rbuf, req)
}

func (s *SyncConn) ReadInt32() (int32, error) {
	if _, err := io.ReadFull(s, s.rbuf[:4]); err != nil {
		return 0, err
	}
	return int32(binary.LittleEndian.Uint32(s.rbuf)), nil
}

func (s *SyncConn) ReadFileMode() (os.FileMode, error) {
	var value uint32
	err := binary.Read(s, binary.LittleEndian, &value)
	if err != nil {
		return 0, fmt.Errorf("error reading filemode from sync scanner: %w", err)
	}
	return ParseFileModeFromAdb(value), nil

}
func (s *SyncConn) ReadTime() (time.Time, error) {
	seconds, err := s.ReadInt32()
	if err != nil {
		return time.Time{}, fmt.Errorf("error reading time from sync scanner: %w", err)
	}

	return time.Unix(int64(seconds), 0).UTC(), nil
}

func (s *SyncConn) ReadString() (string, error) {
	length, err := s.ReadInt32()
	if err != nil {
		return "", fmt.Errorf("error reading length from sync scanner: %w", err)
	}

	bytes := make([]byte, length)
	n, err := io.ReadFull(s, bytes)
	if err == io.ErrUnexpectedEOF {
		return "", errIncompleteMessage("bytes", n, int(length))
	} else if err != nil {
		return "", fmt.Errorf("error reading string from sync scanner: %w", err)
	}

	return string(bytes), nil
}

func (s *SyncConn) ReadBytes(buf []byte) (out []byte, err error) {
	length, err := s.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("error reading bytes from sync scanner: %w", err)
	}
	if len(buf) < int(length) {
		buf = make([]byte, length)
	}
	n, err := io.ReadFull(s, buf[:length])
	return buf[:n], err
}

// ReadNextChunkSize read the 4-bytes length of next chunk of data,
// returns io.EOF if the last chunk has been read.
func (s *SyncConn) ReadNextChunkSize() (int32, error) {
	status, err := s.ReadStatus("read-chunk")
	if err != nil {
		if strings.Contains(err.Error(), "No such file or directory") {
			err = fmt.Errorf("%w: no such file or directory", ErrFileNoExist)
		}
		return 0, err
	}

	switch status {
	case StatusSyncData:
		return s.ReadInt32()
	case StatusSyncDone:
		return 0, io.EOF
	default:
		return 0, fmt.Errorf("%w: expected chunk id '%s' or '%s', but got '%s'",
			ErrAssertion, StatusSyncData, StatusSyncDone, []byte(status))
	}
}

// Reads the status, and if failure, reads the message and returns it as an error.
// If the status is success, doesn't read the message.
// req is just used to populate the AdbError, and can be nil.
func readSyncStatusFailureAsError(r io.Reader, buf []byte, req string) (string, error) {
	// read 4 bytes
	if len(buf) < 4 {
		buf = make([]byte, 4)
	}
	n, err := io.ReadFull(r, buf[0:4])
	if err == io.ErrUnexpectedEOF {
		return "", fmt.Errorf("error reading status for %s: %w", req, errIncompleteMessage(req, n, 4))
	} else if err != nil {
		return "", fmt.Errorf("error reading status for %s: %w", req, err)
	}

	status := string(buf[:n])
	if isFailureStatus(status) {
		msg, err := readSyncMessage(r, buf)
		if err != nil {
			return "", fmt.Errorf("server returned error for %s, but couldn't read the error message, %w", req, err)
		}

		return "", adbServerError(req, string(msg))
	}

	return status, nil
}

// readSyncMessage reads a 4-byte length from r, then reads length bytes and returns them.
func readSyncMessage(r io.Reader, buf []byte) ([]byte, error) {
	// read 4 byte as FFFF string, means a 16bit number
	if len(buf) < 4 {
		buf = make([]byte, 4)
	}
	n, err := io.ReadFull(r, buf[:4])
	if err != nil {
		return nil, errIncompleteMessage("length", n, 4)
	}

	// parse length
	length := binary.LittleEndian.Uint32(buf[:4])
	// read length buf
	if length > uint32(len(buf)) {
		buf = make([]byte, length)
	}
	n, err = io.ReadFull(r, buf[:length])
	if err == io.ErrUnexpectedEOF {
		return buf[:n], errIncompleteMessage("message data", n, int(length))
	} else if err != nil {
		return buf[:n], fmt.Errorf("error reading message data: %w", err)
	}
	return buf[:n], nil
}

func (s *SyncConn) SendOctetString(str string) error {
	if len(str) != 4 {
		return fmt.Errorf("%w: octet string must be exactly 4 bytes: '%s'", ErrAssertion, str)
	}

	if n, err := s.Write([]byte(str)); err != nil {
		return fmt.Errorf("error send string: %w, sent %d", err, n)
	}
	return nil
}

func (s *SyncConn) SendInt32(val int32) error {
	if err := binary.Write(s, binary.LittleEndian, val); err != nil {
		return fmt.Errorf("error sending int on sync sender: %w", err)
	}
	return nil
}

func (s *SyncConn) SendFileMode(mode os.FileMode) error {
	if err := binary.Write(s, binary.LittleEndian, mode); err != nil {
		return fmt.Errorf("error sending filemode on sync sender: %w", err)
	}
	return nil
}

func (s *SyncConn) SendTime(t time.Time) error {
	if err := s.SendInt32(int32(t.Unix())); err != nil {
		return fmt.Errorf("error sending time on sync sender: %w", err)
	}
	return nil
}

func (s *SyncConn) SendBytes(data []byte) error {
	length := len(data)
	if length > SyncMaxChunkSize {
		// This limit might not apply to filenames, but it's big enough
		// that I don't think it will be a problem.
		return fmt.Errorf("%w: data must be <= %d in length", ErrAssertion, SyncMaxChunkSize)
	}

	if err := s.SendInt32(int32(length)); err != nil {
		return fmt.Errorf("error sending data length on sync sender: %w", err)
	}
	if n, err := s.Write(data); err != nil {
		return fmt.Errorf("error send bytes: %w, sent %d", err, n)
	}
	return nil
}
