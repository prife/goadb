package wire

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"time"
)

type SyncSender interface {
	io.Closer

	// SendOctetString sends a 4-byte string.
	SendOctetString(string) error
	SendInt32(int32) error
	SendFileMode(os.FileMode) error
	SendTime(time.Time) error

	// Sends len(data) as an octet, followed by the bytes.
	// If data is bigger than SyncMaxChunkSize, it returns an assertion error.
	SendBytes(data []byte) error
}

type realSyncSender struct {
	io.Writer
}

func NewSyncSender(w io.Writer) SyncSender {
	return &realSyncSender{w}
}

func (s *realSyncSender) SendOctetString(str string) error {
	if len(str) != 4 {
		return fmt.Errorf("%w: octet string must be exactly 4 bytes: '%s'", ErrAssertion, str)
	}

	if n, err := s.Writer.Write([]byte(str)); err != nil {
		return fmt.Errorf("error send string: %w, sent %d", err, n)
	}
	return nil
}

func (s *realSyncSender) SendInt32(val int32) error {
	if err := binary.Write(s.Writer, binary.LittleEndian, val); err != nil {
		return fmt.Errorf("error sending int on sync sender: %w", err)
	}
	return nil
}

func (s *realSyncSender) SendFileMode(mode os.FileMode) error {
	if err := binary.Write(s.Writer, binary.LittleEndian, mode); err != nil {
		return fmt.Errorf("error sending filemode on sync sender: %w", err)
	}
	return nil
}

func (s *realSyncSender) SendTime(t time.Time) error {
	if err := s.SendInt32(int32(t.Unix())); err != nil {
		return fmt.Errorf("error sending time on sync sender: %w", err)
	}
	return nil
}

func (s *realSyncSender) SendBytes(data []byte) error {
	length := len(data)
	if length > SyncMaxChunkSize {
		// This limit might not apply to filenames, but it's big enough
		// that I don't think it will be a problem.
		return fmt.Errorf("%w: data must be <= %d in length", ErrAssertion, SyncMaxChunkSize)
	}

	if err := s.SendInt32(int32(length)); err != nil {
		return fmt.Errorf("error sending data length on sync sender: %w", err)
	}
	if n, err := s.Writer.Write(data); err != nil {
		return fmt.Errorf("error send bytes: %w, sent %d", err, n)
	}
	return nil
}

func (s *realSyncSender) Close() error {
	if closer, ok := s.Writer.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
