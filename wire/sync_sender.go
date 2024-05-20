package wire

import (
	"encoding/binary"
	"fmt"
	"os"
	"time"
)

func (s *realSyncScanner) SendOctetString(str string) error {
	if len(str) != 4 {
		return fmt.Errorf("%w: octet string must be exactly 4 bytes: '%s'", ErrAssertion, str)
	}

	if n, err := s.Write([]byte(str)); err != nil {
		return fmt.Errorf("error send string: %w, sent %d", err, n)
	}
	return nil
}

func (s *realSyncScanner) SendInt32(val int32) error {
	if err := binary.Write(s, binary.LittleEndian, val); err != nil {
		return fmt.Errorf("error sending int on sync sender: %w", err)
	}
	return nil
}

func (s *realSyncScanner) SendFileMode(mode os.FileMode) error {
	if err := binary.Write(s, binary.LittleEndian, mode); err != nil {
		return fmt.Errorf("error sending filemode on sync sender: %w", err)
	}
	return nil
}

func (s *realSyncScanner) SendTime(t time.Time) error {
	if err := s.SendInt32(int32(t.Unix())); err != nil {
		return fmt.Errorf("error sending time on sync sender: %w", err)
	}
	return nil
}

func (s *realSyncScanner) SendBytes(data []byte) error {
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
