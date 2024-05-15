package wire

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"time"
)

type SyncScanner interface {
	io.Closer
	StatusReader
	ReadInt32() (int32, error)
	ReadFileMode() (os.FileMode, error)
	ReadTime() (time.Time, error)

	// Reads an octet length, followed by length bytes.
	ReadString() (string, error)

	// Reads an octet length, and returns a reader that will read length
	// bytes (see io.LimitReader). The returned reader should be fully
	// read before reading anything off the Scanner again.
	ReadBytes() (io.Reader, error)
}

type realSyncScanner struct {
	io.Reader
}

func NewSyncScanner(r io.Reader) SyncScanner {
	return &realSyncScanner{r}
}

func (s *realSyncScanner) ReadStatus(req string) (string, error) {
	return readStatusFailureAsError(s.Reader, req, readInt32)
}

func (s *realSyncScanner) ReadInt32() (int32, error) {
	if value, err := readInt32(s.Reader); err != nil {
		return 0, fmt.Errorf("error reading int from sync scanner: %w", err)
	} else {
		return int32(value), nil
	}
}
func (s *realSyncScanner) ReadFileMode() (os.FileMode, error) {
	var value uint32
	err := binary.Read(s.Reader, binary.LittleEndian, &value)
	if err != nil {
		return 0, fmt.Errorf("error reading filemode from sync scanner: %w", err)
	}
	return ParseFileModeFromAdb(value), nil

}
func (s *realSyncScanner) ReadTime() (time.Time, error) {
	seconds, err := s.ReadInt32()
	if err != nil {
		return time.Time{}, fmt.Errorf("error reading time from sync scanner: %w", err)
	}

	return time.Unix(int64(seconds), 0).UTC(), nil
}

func (s *realSyncScanner) ReadString() (string, error) {
	length, err := s.ReadInt32()
	if err != nil {
		return "", fmt.Errorf("error reading length from sync scanner: %w", err)
	}

	bytes := make([]byte, length)
	n, rawErr := io.ReadFull(s.Reader, bytes)
	if rawErr != nil && rawErr != io.ErrUnexpectedEOF {
		return "", fmt.Errorf("error reading string from sync scanner: %w", err)
	} else if rawErr == io.ErrUnexpectedEOF {
		return "", errIncompleteMessage("bytes", n, int(length))
	}

	return string(bytes), nil
}
func (s *realSyncScanner) ReadBytes() (io.Reader, error) {
	length, err := s.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("error reading bytes from sync scanner: %w", err)
	}

	return io.LimitReader(s.Reader, int64(length)), nil
}

func (s *realSyncScanner) Close() error {
	if closer, ok := s.Reader.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
