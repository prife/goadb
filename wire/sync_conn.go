package wire

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
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
type SyncConn struct {
	net.Conn
	rbuf []byte
	wbuf []byte
}

func NewSyncConn(r net.Conn) *SyncConn {
	return &SyncConn{r, make([]byte, 8), make([]byte, 8)}
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

func (s *SyncConn) ReadTime() (time.Time, error) {
	seconds, err := s.ReadInt32()
	if err != nil {
		return time.Time{}, fmt.Errorf("error reading time from sync scanner: %w", err)
	}

	return time.Unix(int64(seconds), 0).UTC(), nil
}

// Reads an octet length, followed by length bytes.
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

// Reads an octet length, and then the length of bytes.
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
//
//	struct __attribute__((packed)) {
//		uint32_t id;
//		uint32_t size;
//	} data; // followed by `size` bytes of data.
//
//  struct __attribute__((packed)) {
// 	    uint32_t id;
// 	    uint32_t msglen;
//  } status; // followed by `msglen` bytes of error message, if id == ID_FAIL.

func (s *SyncConn) ReadNextChunkSize() (int32, error) {
	_, err := io.ReadFull(s, s.rbuf[:8])
	if err != nil {
		return 0, fmt.Errorf("sync read: %w", err)
	}

	id := string(s.rbuf[:4])
	size := int32(binary.LittleEndian.Uint32(s.rbuf[4:8]))

	switch id {
	case StatusSyncData:
		return size, nil
	case StatusSyncDone:
		return 0, io.EOF
	case StatusFailure:
		buf := make([]byte, size)
		_, err := io.ReadFull(s, buf[:size])
		if err != nil {
			return 0, fmt.Errorf("sync read: %w", err)
		}
		if bytes.Contains(buf[:size], []byte("No such file or directory")) {
			err = fmt.Errorf("%w: no such file or directory", ErrFileNoExist)
		} else {
			err = adbServerError("read-chunk", string(buf[:size]))
		}
		return 0, err
	default:
		return 0, fmt.Errorf("%w: expected chunk id '%s' or '%s', but got '%s'",
			ErrAssertion, StatusSyncData, StatusSyncDone, []byte(id))
	}
}

// Reads the status, and if failure, reads the message and returns it as an error.
// If the status is success, doesn't read the message.
// req is just used to populate the AdbError, and can be nil.
func readSyncStatusFailureAsError(r io.Reader, buf []byte, req string) (string, error) {
	// read 8 bytes
	if len(buf) < 8 {
		buf = make([]byte, 8)
	}

	n, err := io.ReadFull(r, buf[0:8])
	if err == io.ErrUnexpectedEOF {
		return "", fmt.Errorf("error reading status for %s: %w", req, errIncompleteMessage(req, n, 4))
	} else if err != nil {
		return "", fmt.Errorf("error reading status for %s: %w", req, err)
	}

	status := string(buf[:4])
	fmt.Println("<---status: ", status)
	if status == StatusSuccess {
		return status, nil
	}

	// reads a 4-byte length from r, then reads length bytes
	length := binary.LittleEndian.Uint32(buf[4:8])
	if length > 0 {
		if length > uint32(len(buf)) {
			buf = make([]byte, length)
		}
		_, err = io.ReadFull(r, buf[:length])
		if err != nil {
			return status, fmt.Errorf("read status body error: %w", err)
		}
	}

	if status == StatusFailure {
		return status, adbServerError(req, string(buf[:length]))
	}

	return status, fmt.Errorf("unknown reason %s", status)
}

// SendOctetString sends a 4-byte string.
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

// SendBytes send len(data) as an octet, followed by the bytes.
// if data is bigger than SyncMaxChunkSize, it returns an assertion error.
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
