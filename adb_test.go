package adb

import (
	"bytes"
	"net"
	"testing"
	"time"

	"github.com/prife/goadb/wire"
	"github.com/stretchr/testify/assert"
)

func TestGetServerVersion(t *testing.T) {
	s := &MockServer{
		Status:   wire.StatusSuccess,
		Messages: []string{"000a"},
	}
	client := &Adb{s}

	v, err := client.ServerVersion()
	assert.Equal(t, "host:version", s.Requests[0])
	assert.NoError(t, err)
	assert.Equal(t, 10, v)
}

// mockConn is a wrapper around a bytes.Buffer that implements net.Conn
type mockConn struct {
	// io.ReadWriter
	*bytes.Buffer // write buffer
	rbuf          *bytes.Buffer
}

func makeMockConnStr(str string) net.Conn {
	w := &mockConn{
		Buffer: bytes.NewBufferString(str),
	}
	return w
}

func makeMockConnBuf(buf *bytes.Buffer) net.Conn {
	w := &mockConn{
		Buffer: buf,
	}
	return w
}

func makeMockConnBytes(b []byte) net.Conn {
	w := &mockConn{
		Buffer: bytes.NewBuffer(b),
	}
	return w
}

func makeMockConn2(str string, buf *bytes.Buffer) net.Conn {
	w := &mockConn{
		rbuf:   bytes.NewBufferString(str),
		Buffer: buf,
	}
	return w
}

func (b *mockConn) Read(p []byte) (n int, err error) {
	if b.rbuf != nil {
		return b.rbuf.Read(p)
	}

	return b.Buffer.Read(p)
}

func (b *mockConn) Write(p []byte) (n int, err error) {
	return b.Buffer.Write(p)
}

func (b *mockConn) Close() error {
	// No-op.
	return nil
}

func (b *mockConn) LocalAddr() net.Addr {
	return nil
}
func (b *mockConn) RemoteAddr() net.Addr {
	return nil
}

func (b *mockConn) SetDeadline(t time.Time) error {
	return nil
}

func (b *mockConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (b *mockConn) SetWriteDeadline(t time.Time) error {
	return nil
}
