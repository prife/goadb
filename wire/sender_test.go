package wire

import (
	"bytes"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWriteMessage(t *testing.T) {
	s, b := NewTestSender()
	err := s.SendMessage([]byte("hello"))
	assert.NoError(t, err)
	assert.Equal(t, "0005hello", b.String())
}

func TestWriteEmptyMessage(t *testing.T) {
	s, b := NewTestSender()
	err := s.SendMessage([]byte(""))
	assert.NoError(t, err)
	assert.Equal(t, "0000", b.String())
}

func NewTestSender() (Sender, *TestWriter) {
	w := new(TestWriter)
	return NewConn(w), w
}

// TestWriter is a wrapper around a bytes.Buffer that implements io.Closer.
type TestWriter struct {
	bytes.Buffer
}

func (b *TestWriter) Close() error {
	// No-op.
	return nil
}

func (b *TestWriter) LocalAddr() net.Addr {
	return nil
}
func (b *TestWriter) RemoteAddr() net.Addr {
	return nil
}

func (b *TestWriter) SetDeadline(t time.Time) error {
	return nil
}

func (b *TestWriter) SetReadDeadline(t time.Time) error {
	return nil
}

func (b *TestWriter) SetWriteDeadline(t time.Time) error {
	return nil
}
