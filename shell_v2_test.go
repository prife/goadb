package adb

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/prife/goadb/wire"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeviceShellStreamsBinaryOutputAndReturnsRemoteExitCode(t *testing.T) {
	stdoutPayload := []byte{0x00, 0xff, 'm', 't', '2'}
	stderrPayload := []byte("warning\n")
	server := newShellV2MockServer(
		shellFrame(shellStdout, stdoutPayload),
		shellFrame(shellStderr, stderrPayload),
		shellFrame(shellExit, []byte{7}),
	)
	device := (&Adb{server}).Device(AnyDevice())

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode, err := device.Shell(
		context.Background(), &stdout, &stderr, "run-as", "com.example.game", "cat", "files/a b.mt2")

	require.NoError(t, err)
	assert.Equal(t, 7, exitCode)
	assert.Equal(t, stdoutPayload, stdout.Bytes())
	assert.Equal(t, stderrPayload, stderr.Bytes())
	assert.Equal(t, []string{
		"host:features",
		"host:transport-any",
		"shell,v2,raw:'run-as' 'com.example.game' 'cat' 'files/a b.mt2'",
	}, server.Requests)
	assert.Equal(t, shellFrame(shellCloseStdin, nil), server.Buffer.Bytes())
}

func TestDeviceShellRejectsUnsupportedDeviceWithoutFallingBack(t *testing.T) {
	server := &MockServer{
		Status:   wire.StatusSuccess,
		Messages: []string{"cmd,stat_v2"},
	}
	device := (&Adb{server}).Device(AnyDevice())

	exitCode, err := device.Shell(context.Background(), io.Discard, io.Discard, "true")

	assert.Equal(t, ShellExitCodeUnavailable, exitCode)
	assert.ErrorIs(t, err, ErrShellV2Unsupported)
	assert.Equal(t, []string{"host:features"}, server.Requests)
}

func TestDeviceShellDoesNotDialWhenContextAlreadyCanceled(t *testing.T) {
	server := &MockServer{Status: wire.StatusSuccess}
	device := (&Adb{server}).Device(AnyDevice())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	exitCode, err := device.Shell(ctx, io.Discard, io.Discard, "true")

	assert.Equal(t, ShellExitCodeUnavailable, exitCode)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Empty(t, server.Requests)
}

func TestDeviceShellCancellationClosesConnectionAndJoinsReader(t *testing.T) {
	featureConn := &MockServer{
		Status:   wire.StatusSuccess,
		Messages: []string{FeatureShell2},
	}
	shellConn := newBlockingShellConn()
	server := &sequenceServer{connections: []wire.IConn{featureConn, shellConn}}
	device := (&Adb{server}).Device(AnyDevice())
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		exitCode, err := device.Shell(ctx, io.Discard, io.Discard, "sleep", "60")
		if exitCode != ShellExitCodeUnavailable {
			done <- errors.New("canceled shell returned an exit code")
			return
		}
		done <- err
	}()

	<-shellConn.readStarted
	cancel()
	err := <-done
	assert.ErrorIs(t, err, context.Canceled)
	select {
	case <-shellConn.readReturned:
		// The reader has been joined before Shell returned.
	default:
		t.Fatal("Shell returned before its reader goroutine stopped")
	}
}

func TestDeviceShellV1RawStreamsBinaryWithExplicitLegacySemantics(t *testing.T) {
	payload := []byte{0x00, 0xff, 'm', 't', '2', '\n'}
	server := &MockServer{
		Status: wire.StatusSuccess,
		mockConn: mockConn{
			Buffer: bytes.NewBuffer(nil),
			rbuf:   bytes.NewBuffer(payload),
		},
	}
	device := (&Adb{server}).Device(AnyDevice())
	var output bytes.Buffer

	err := device.ShellV1Raw(
		context.Background(), &output, "run-as", "com.example.game", "cat", "files/a b.mt2")

	require.NoError(t, err)
	assert.Equal(t, payload, output.Bytes())
	assert.Equal(t, []string{
		"host:transport-any",
		"shell:'run-as' 'com.example.game' 'cat' 'files/a b.mt2'",
	}, server.Requests)
}

func TestDeviceShellV1RawCancellationClosesConnectionAndJoinsReader(t *testing.T) {
	shellConn := newBlockingShellConn()
	server := &sequenceServer{connections: []wire.IConn{shellConn}}
	device := (&Adb{server}).Device(AnyDevice())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() {
		done <- device.ShellV1Raw(ctx, io.Discard, "sleep", "60")
	}()
	<-shellConn.readStarted
	cancel()

	err := <-done
	assert.ErrorIs(t, err, context.Canceled)
	select {
	case <-shellConn.readReturned:
	default:
		t.Fatal("ShellV1Raw returned before its reader goroutine stopped")
	}
}

func TestDeviceShellV1RawWriterFailure(t *testing.T) {
	wantErr := errors.New("destination failed")
	server := &MockServer{
		Status: wire.StatusSuccess,
		mockConn: mockConn{
			Buffer: bytes.NewBuffer(nil),
			rbuf:   bytes.NewBufferString("payload"),
		},
	}
	device := (&Adb{server}).Device(AnyDevice())

	err := device.ShellV1Raw(
		context.Background(), errorWriter{err: wantErr}, "command")

	assert.ErrorIs(t, err, wantErr)
}

func TestDeviceShellWriterFailure(t *testing.T) {
	wantErr := errors.New("destination failed")
	tests := []struct {
		name   string
		writer io.Writer
		want   error
	}{
		{name: "writer error", writer: errorWriter{err: wantErr}, want: wantErr},
		{name: "short write", writer: shortWriter{}, want: io.ErrShortWrite},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newShellV2MockServer(
				shellFrame(shellStdout, []byte("payload")),
				shellFrame(shellExit, []byte{0}),
			)
			device := (&Adb{server}).Device(AnyDevice())

			exitCode, err := device.Shell(
				context.Background(), tt.writer, io.Discard, "command")

			assert.Equal(t, ShellExitCodeUnavailable, exitCode)
			assert.ErrorIs(t, err, tt.want)
		})
	}
}

func TestReadShellV2RejectsMalformedProtocol(t *testing.T) {
	tests := []struct {
		name      string
		response  []byte
		wantError string
	}{
		{
			name:      "missing exit",
			response:  shellFrame(shellStdout, []byte("done")),
			wantError: "command exited without exit status",
		},
		{
			name:      "empty exit payload",
			response:  shellFrame(shellExit, nil),
			wantError: "invalid shell exit payload length 0",
		},
		{
			name:      "oversized exit payload",
			response:  shellFrame(shellExit, []byte{0, 1}),
			wantError: "invalid shell exit payload length 2",
		},
		{
			name:      "unexpected message",
			response:  shellFrame(shellStdin, []byte("input")),
			wantError: "unexpected shell message type 0",
		},
		{
			name: "oversized frame",
			response: func() []byte {
				var response bytes.Buffer
				response.WriteByte(byte(shellStdout))
				require.NoError(t, binary.Write(
					&response, binary.LittleEndian, uint32(wire.MaxPayload+1)))
				return response.Bytes()
			}(),
			wantError: "exceeds maximum",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := &mockConn{Buffer: bytes.NewBuffer(tt.response)}
			exitCode, err := readShellV2(newShellTransport(conn, 0), io.Discard, io.Discard)
			assert.Equal(t, ShellExitCodeUnavailable, exitCode)
			assert.ErrorContains(t, err, tt.wantError)
		})
	}
}

func TestPrepareShellCommandLineQuotesEveryArg(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
		want    string
		wantErr error
	}{
		{name: "simple", command: "echo", args: []string{"hello"}, want: "'echo' 'hello'"},
		{name: "empty arg", command: "echo", args: []string{""}, want: "'echo' ''"},
		{
			name: "metacharacters stay literal", command: "printf",
			args: []string{"$(touch /tmp/bad); * $HOME"},
			want: "'printf' '$(touch /tmp/bad); * $HOME'",
		},
		{
			name: "single quote", command: "printf", args: []string{"a'b"},
			want: "'printf' 'a'\"'\"'b'",
		},
		{name: "blank command", command: "  ", wantErr: wire.ErrAssertion},
		{name: "NUL command", command: "bad\x00cmd", wantErr: wire.ErrParse},
		{name: "NUL arg", command: "echo", args: []string{"bad\x00arg"}, wantErr: wire.ErrParse},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := prepareShellCommandLine(tt.command, tt.args...)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func newShellV2MockServer(frames ...[]byte) *MockServer {
	return &MockServer{
		Status:   wire.StatusSuccess,
		Messages: []string{FeatureShell2 + ",cmd"},
		mockConn: mockConn{
			Buffer: bytes.NewBuffer(nil),
			rbuf:   bytes.NewBuffer(bytes.Join(frames, nil)),
		},
	}
}

func shellFrame(messageType shellMessageType, payload []byte) []byte {
	var frame bytes.Buffer
	frame.WriteByte(byte(messageType))
	_ = binary.Write(&frame, binary.LittleEndian, uint32(len(payload)))
	frame.Write(payload)
	return frame.Bytes()
}

type errorWriter struct{ err error }

func (w errorWriter) Write([]byte) (int, error) { return 0, w.err }

type shortWriter struct{}

func (shortWriter) Write(payload []byte) (int, error) { return len(payload) - 1, nil }

type sequenceServer struct {
	mu          sync.Mutex
	connections []wire.IConn
}

func (*sequenceServer) Start() error { return nil }

func (s *sequenceServer) Dial() (wire.IConn, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.connections) == 0 {
		return nil, errors.New("unexpected dial")
	}
	connection := s.connections[0]
	s.connections = s.connections[1:]
	return connection, nil
}

type blockingShellConn struct {
	closed       chan struct{}
	readStarted  chan struct{}
	readReturned chan struct{}
	closeOnce    sync.Once
	readOnce     sync.Once
}

func newBlockingShellConn() *blockingShellConn {
	return &blockingShellConn{
		closed:       make(chan struct{}),
		readStarted:  make(chan struct{}),
		readReturned: make(chan struct{}),
	}
}

func (c *blockingShellConn) Read([]byte) (int, error) {
	c.readOnce.Do(func() { close(c.readStarted) })
	<-c.closed
	close(c.readReturned)
	return 0, net.ErrClosed
}

func (*blockingShellConn) Write(payload []byte) (int, error) { return len(payload), nil }

func (c *blockingShellConn) Close() error {
	c.closeOnce.Do(func() { close(c.closed) })
	return nil
}

func (*blockingShellConn) LocalAddr() net.Addr               { return nil }
func (*blockingShellConn) RemoteAddr() net.Addr              { return nil }
func (*blockingShellConn) SetDeadline(time.Time) error       { return nil }
func (*blockingShellConn) SetReadDeadline(time.Time) error   { return nil }
func (*blockingShellConn) SetWriteDeadline(time.Time) error  { return nil }
func (*blockingShellConn) ReadStatus(string) (string, error) { return wire.StatusSuccess, nil }
func (*blockingShellConn) SendMessage([]byte) error          { return nil }
func (*blockingShellConn) ReadMessage() ([]byte, error)      { return nil, io.EOF }
func (*blockingShellConn) ReadUntilEof() ([]byte, error)     { return nil, io.EOF }
func (*blockingShellConn) RoundTripSingleResponse([]byte) ([]byte, error) {
	return nil, errors.New("unexpected round trip")
}
