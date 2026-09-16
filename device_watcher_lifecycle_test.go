package adb

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prife/goadb/wire"
)

// watcherPipeServer hands out pre-built connections, then reports the server as
// unavailable. It records Start calls so tests can assert the watcher never
// manages the server lifecycle.
type watcherPipeServer struct {
	mu          sync.Mutex
	connections []wire.IConn
	starts      int
	dialed      chan struct{}
}

func (s *watcherPipeServer) Dial() (wire.IConn, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dialed != nil {
		select {
		case s.dialed <- struct{}{}:
		default:
		}
	}
	if len(s.connections) == 0 {
		return nil, wire.ErrServerNotAvailable
	}
	conn := s.connections[0]
	s.connections = s.connections[1:]
	return conn, nil
}

func (s *watcherPipeServer) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.starts++
	return nil
}

// watcherPipe speaks the real framed track protocol over net.Pipe. The returned
// channel yields the handshake outcome so a fixture that was never dialed fails
// loudly instead of silently timing out the test that waits on it.
func watcherPipe(t *testing.T) (*wire.Conn, *wire.Conn, <-chan error) {
	t.Helper()
	client, remote := net.Pipe()
	c, r := wire.NewConn(client), wire.NewConn(remote)
	done := make(chan error, 1)
	go func() {
		request, err := r.ReadMessage()
		if err != nil {
			done <- fmt.Errorf("handshake read: %w", err)
			return
		}
		if string(request) != "host:track-devices" {
			done <- fmt.Errorf("handshake request=%q", request)
			return
		}
		if _, err := io.WriteString(remote, "OKAY"); err != nil {
			done <- fmt.Errorf("handshake reply: %w", err)
			return
		}
		done <- nil
	}()
	t.Cleanup(func() {
		_ = c.Close()
		_ = r.Close()
	})
	return c, r, done
}

// awaitHandshake blocks until the watcher has subscribed on this pipe.
func awaitHandshake(t *testing.T, ready <-chan error) {
	t.Helper()
	select {
	case err := <-ready:
		if err != nil {
			t.Fatalf("watcher did not subscribe: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("watcher never subscribed")
	}
}

func newTestWatcher(t *testing.T, server server) *DeviceWatcher {
	t.Helper()
	impl := newDeviceWatcherImpl(context.Background(), server, true)
	impl.retryDelay, impl.maxRetryDelay = time.Millisecond, 2*time.Millisecond
	impl.offlineAfter = 200 * time.Millisecond
	w := startDeviceWatcher(impl)
	t.Cleanup(w.Shutdown)
	return w
}

func nextWatcherEvent(t *testing.T, w *DeviceWatcher) DeviceStateChangedEvent {
	t.Helper()
	select {
	case event, ok := <-w.C():
		if !ok {
			t.Fatalf("watcher closed: %v", w.Err())
		}
		return event
	case <-time.After(2 * time.Second):
		t.Fatal("watcher event timed out")
	}
	return DeviceStateChangedEvent{}
}

func assertNoEvent(t *testing.T, w *DeviceWatcher, within time.Duration) {
	t.Helper()
	select {
	case e := <-w.C():
		t.Fatalf("unexpected event: %+v", e)
	case <-time.After(within):
	}
}

// stopTestWatcher asserts the whole shutdown contract: it returns promptly,
// closes the event stream, reports no error and can be called twice.
func stopTestWatcher(t *testing.T, w *DeviceWatcher) {
	t.Helper()
	done := make(chan struct{})
	go func() { w.Shutdown(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Shutdown blocked")
	}
	if _, ok := <-w.C(); ok {
		t.Fatal("event stream remains open after Shutdown")
	}
	if w.Err() != nil {
		t.Fatalf("clean shutdown reported err=%v", w.Err())
	}
	w.Shutdown() // idempotent
}

func TestDeviceWatcherOrdersEventsUnderBackpressure(t *testing.T) {
	c, remote, ready := watcherPipe(t)
	w := newTestWatcher(t, &watcherPipeServer{connections: []wire.IConn{c}})
	awaitHandshake(t, ready)

	const count = 80
	var list strings.Builder
	for i := 0; i < count; i++ {
		fmt.Fprintf(&list, "SERIAL-%d\tdevice\n", i)
	}
	sent := make(chan error, 1)
	go func() {
		if err := remote.SendMessage([]byte(list.String())); err != nil {
			sent <- err
			return
		}
		// An identical list must be deduplicated into nothing.
		if err := remote.SendMessage([]byte(list.String())); err != nil {
			sent <- err
			return
		}
		sent <- remote.SendMessage(nil)
	}()

	seen := make(map[string]bool)
	for i := 0; i < count; i++ {
		e := nextWatcherEvent(t, w)
		if !e.CameOnline() || seen[e.Serial] {
			t.Fatalf("duplicate/invalid connect: %+v", e)
		}
		seen[e.Serial] = true
	}
	for i := 0; i < count; i++ {
		e := nextWatcherEvent(t, w)
		if !e.WentOffline() || !seen[e.Serial] {
			t.Fatalf("out-of-order/invalid disconnect: %+v", e)
		}
		delete(seen, e.Serial)
	}
	if err := <-sent; err != nil {
		t.Fatal(err)
	}
	stopTestWatcher(t, w) // worker is now blocked reading an idle stream
}

func TestDeviceWatcherUnusableDeviceDoesNotDisconnectHealthyOnes(t *testing.T) {
	c, remote, ready := watcherPipe(t)
	w := newTestWatcher(t, &watcherPipeServer{connections: []wire.IConn{c}})
	awaitHandshake(t, ready)
	go func() { _ = remote.SendMessage([]byte("A\tdevice\nB\trecovery\n")) }()

	states := map[string]DeviceState{}
	for i := 0; i < 2; i++ {
		e := nextWatcherEvent(t, w)
		states[e.Serial] = e.NewState
	}
	if states["A"] != StateOnline || states["B"] != StateOffline {
		t.Fatalf("states=%v", states)
	}
	// The unusable device must not restart the stream behind our back.
	assertNoEvent(t, w, 300*time.Millisecond)
	stopTestWatcher(t, w)
}

func TestDeviceWatcherQuickReconnectPreservesSnapshot(t *testing.T) {
	c1, remote1, ready1 := watcherPipe(t)
	c2, remote2, ready2 := watcherPipe(t)
	server := &watcherPipeServer{connections: []wire.IConn{c1, c2}}
	w := newTestWatcher(t, server)

	awaitHandshake(t, ready1)
	go func() { _ = remote1.SendMessage([]byte("SERIAL-1\tdevice\n")); _ = remote1.Close() }()
	if e := nextWatcherEvent(t, w); !e.CameOnline() {
		t.Fatalf("initial=%+v", e)
	}
	awaitHandshake(t, ready2)
	go func() {
		_ = remote2.SendMessage([]byte("SERIAL-1\tdevice\n"))
		_ = remote2.SendMessage([]byte("SERIAL-1\tdevice\nMARKER\tdevice\n"))
	}()
	// SERIAL-1 never left, so the only event may be the new device.
	if e := nextWatcherEvent(t, w); e.Serial != "MARKER" || !e.CameOnline() {
		t.Fatalf("quick reconnect caused churn: %+v", e)
	}
	stopTestWatcher(t, w)

	server.mu.Lock()
	defer server.mu.Unlock()
	if server.starts != 0 {
		t.Fatalf("watcher started the server %d times", server.starts)
	}
}

func TestDeviceWatcherSustainedOutageEvictsOnceThenRediscovers(t *testing.T) {
	c1, remote1, ready1 := watcherPipe(t)
	c2, remote2, ready2 := watcherPipe(t)
	server := &watcherPipeServer{connections: []wire.IConn{c1}}
	w := newTestWatcher(t, server)

	awaitHandshake(t, ready1)
	go func() { _ = remote1.SendMessage([]byte("SERIAL-1\tdevice\n")); _ = remote1.Close() }()
	if e := nextWatcherEvent(t, w); !e.CameOnline() {
		t.Fatalf("initial=%+v", e)
	}
	// No connection is available, so the outage runs past offlineAfter.
	start := time.Now()
	e := nextWatcherEvent(t, w)
	if !e.WentOffline() || e.Serial != "SERIAL-1" {
		t.Fatalf("eviction=%+v", e)
	}
	if elapsed := time.Since(start); elapsed < 150*time.Millisecond {
		t.Fatalf("evicted after %s, before offlineAfter elapsed", elapsed)
	}
	// Eviction happens once, not on every retry.
	assertNoEvent(t, w, 100*time.Millisecond)

	server.mu.Lock()
	server.connections = append(server.connections, c2)
	server.mu.Unlock()
	awaitHandshake(t, ready2)
	go func() { _ = remote2.SendMessage([]byte("SERIAL-1\tdevice\n")) }()
	if e := nextWatcherEvent(t, w); !e.CameOnline() {
		t.Fatalf("rediscovery=%+v", e)
	}
	stopTestWatcher(t, w)

	server.mu.Lock()
	defer server.mu.Unlock()
	if server.starts != 0 {
		t.Fatalf("watcher started the server %d times", server.starts)
	}
}

func TestDeviceWatcherShutdownCancelsBlockedDelivery(t *testing.T) {
	c, remote, ready := watcherPipe(t)
	w := newTestWatcher(t, &watcherPipeServer{connections: []wire.IConn{c}})
	awaitHandshake(t, ready)
	written := make(chan error, 1)
	go func() { written <- remote.SendMessage([]byte("SERIAL-1\tdevice\n")) }()
	if err := <-written; err != nil {
		t.Fatal(err)
	}
	// Never consume the connect event: the worker must exit anyway.
	stopTestWatcher(t, w)
}

func TestDeviceWatcherShutdownCancelsBlockedHandshake(t *testing.T) {
	client, remote := net.Pipe()
	t.Cleanup(func() { _ = client.Close(); _ = remote.Close() })
	w := newTestWatcher(t, &watcherPipeServer{connections: []wire.IConn{wire.NewConn(client)}})
	request, err := wire.NewConn(remote).ReadMessage()
	if err != nil || string(request) != "host:track-devices" {
		t.Fatalf("handshake=%q err=%v", request, err)
	}
	// Deliberately never reply OKAY: Shutdown must close the blocked read.
	stopTestWatcher(t, w)
}

func TestDeviceWatcherShutdownDuringReconnectWait(t *testing.T) {
	server := &watcherPipeServer{dialed: make(chan struct{}, 1)}
	w := newTestWatcher(t, server)
	<-server.dialed
	stopTestWatcher(t, w)

	server.mu.Lock()
	defer server.mu.Unlock()
	if server.starts != 0 {
		t.Fatal("context watcher attempted to start a server")
	}
}

func TestDeviceWatcherCancelledBeforeDial(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	server := &watcherPipeServer{dialed: make(chan struct{}, 1)}
	w := (&Adb{server: server}).NewDeviceWatcherWithContext(ctx)
	stopTestWatcher(t, w)
	select {
	case <-server.dialed:
		t.Fatal("cancelled watcher attempted a dial")
	default:
	}
}

func TestDeviceWatcherRepeatedCancellation(t *testing.T) {
	for i := 0; i < 100; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		w := (&Adb{server: &watcherPipeServer{}}).NewDeviceWatcherWithContext(ctx)
		cancel()
		stopTestWatcher(t, w)
	}
}

// deadDeadlineConn fails only when the deadline is cleared, like a connection
// the peer closed between the list arriving and the watcher clearing the
// handshake deadline.
type deadDeadlineConn struct {
	wire.IConn
}

func (c deadDeadlineConn) SetDeadline(t time.Time) error {
	if t.IsZero() {
		return io.ErrClosedPipe
	}
	return c.IConn.SetDeadline(t)
}

func TestDeviceWatcherKeepsListWhenDeadlineCleanupFails(t *testing.T) {
	c, remote, ready := watcherPipe(t)
	w := newTestWatcher(t, &watcherPipeServer{connections: []wire.IConn{deadDeadlineConn{c}}})
	awaitHandshake(t, ready)
	go func() { _ = remote.SendMessage([]byte("SERIAL-1\tdevice\n")) }()
	if e := nextWatcherEvent(t, w); !e.CameOnline() || e.Serial != "SERIAL-1" {
		t.Fatalf("parsed list was discarded: %+v", e)
	}
	stopTestWatcher(t, w)
}

type failedWatcherDialer struct{}

func (failedWatcherDialer) Dial(string, time.Duration) (*wire.Conn, error) { return nil, net.ErrClosed }

func TestDeviceWatcherNeverStartsTheServer(t *testing.T) {
	var starts int
	server := &realServer{address: "unused", config: ServerConfig{
		AutoStart: true, Dialer: failedWatcherDialer{},
		fs: &filesystem{CmdCombinedOutput: func(string, ...string) ([]byte, error) { starts++; return nil, nil }},
	}}
	w := (&Adb{server: server}).NewDeviceWatcherWithContext(context.Background())
	if w.server.(*realServer).config.AutoStart {
		t.Error("context watcher retained AutoStart")
	}
	if w.Device("SERIAL-1").server.(*realServer).config.AutoStart {
		t.Error("watcher handed out an AutoStart device")
	}
	if !server.config.AutoStart {
		t.Error("the original client was modified")
	}
	stopTestWatcher(t, w)
	if starts != 0 {
		t.Fatal("watcher managed the server lifecycle")
	}
}
