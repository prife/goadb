package wire

import (
	"fmt"
	"io"
)

// Sender sends messages to the server.
type Sender interface {
	SendMessage(msg []byte) error

	NewSyncSender() SyncSender

	Close() error
}

type realSender struct {
	writer io.WriteCloser
}

func NewSender(w io.WriteCloser) Sender {
	return &realSender{w}
}

func (s *realSender) SendMessage(msg []byte) error {
	if len(msg) > MaxMessageLength {
		return fmt.Errorf("message length exceeds maximum:%d", len(msg))
	}

	// FIXME: when message is very large, if cost heavy
	lengthAndMsg := fmt.Sprintf("%04x%s", len(msg), msg)
	_, err := s.writer.Write([]byte(lengthAndMsg))
	return err
}

func (s *realSender) NewSyncSender() SyncSender {
	return NewSyncSender(s.writer)
}

func (s *realSender) Close() error {
	if err := s.writer.Close(); err != nil {
		return fmt.Errorf("error closing sender: %w", err)
	}
	return nil
}

var _ Sender = &realSender{}
