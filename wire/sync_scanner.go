package wire

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"time"
)

// ReadStatus reads a little-endian length from r, then reads length bytes and returns them
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
func (s *SyncConn) ReadBytes() (io.Reader, error) {
	length, err := s.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("error reading bytes from sync scanner: %w", err)
	}

	return io.LimitReader(s, int64(length)), nil
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
