package adb

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prife/goadb/wire"
)

// trackHandshakeTimeout bounds the host:track-devices handshake and the first
// device list. A server that accepts the connection but never answers would
// otherwise block the worker forever and defeat disconnect reporting. It is a
// "something is badly wrong" bound, not a tuning knob; the established stream
// that follows is idle by design and carries no deadline.
const trackHandshakeTimeout = 30 * time.Second

// DeviceWatcher publishes device status change events.
// The legacy constructor restarts the server if it dies. The context
// constructor reconnects but never touches the server lifecycle.
type DeviceWatcher struct {
	*deviceWatcherImpl
}

// DeviceStateChangedEvent represents a device state transition.
// Contains the device’s old and new states, but also provides methods to query the
// type of state transition.
type DeviceStateChangedEvent struct {
	Serial   string
	OldState DeviceState
	NewState DeviceState
}

// CameOnline returns true if this event represents a device coming online.
func (s DeviceStateChangedEvent) CameOnline() bool {
	return s.OldState != StateOnline && s.NewState == StateOnline
}

// WentOffline returns true if this event represents a device going offline.
func (s DeviceStateChangedEvent) WentOffline() bool {
	return s.OldState == StateOnline && s.NewState != StateOnline
}

type deviceWatcherImpl struct {
	server server
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}

	// reconnect selects the externally-managed-server behavior: retry forever
	// and never call server.Start.
	reconnect     bool
	retryDelay    time.Duration
	maxRetryDelay time.Duration
	offlineAfter  time.Duration
	lastLog       time.Time

	// conn is the connection the worker is blocked on. Closing it is the only
	// way to interrupt a blocked read, so Shutdown and the context both do it.
	connMu sync.Mutex
	conn   wire.IConn

	// The terminal error, stored before eventChan is closed.
	err atomic.Value

	eventChan chan DeviceStateChangedEvent
}

func newDeviceWatcher(server server) *DeviceWatcher {
	return startDeviceWatcher(newDeviceWatcherImpl(context.Background(), server, false))
}

func newDeviceWatcherImpl(ctx context.Context, server server, reconnect bool) *deviceWatcherImpl {
	ctx, cancel := context.WithCancel(ctx)
	return &deviceWatcherImpl{
		server: server, ctx: ctx, cancel: cancel,
		done: make(chan struct{}), eventChan: make(chan DeviceStateChangedEvent),
		reconnect: reconnect, retryDelay: time.Second,
		maxRetryDelay: 30 * time.Second, offlineAfter: 30 * time.Second,
	}
}

func startDeviceWatcher(impl *deviceWatcherImpl) *DeviceWatcher {
	go publishDevices(impl)
	return &DeviceWatcher{impl}
}

// C returns a channel than can be received on to get events.
// If an unrecoverable error occurs, or Shutdown is called, the channel will be closed.
func (w *DeviceWatcher) C() <-chan DeviceStateChangedEvent {
	return w.eventChan
}

// Device creates a client for a discovered serial without performing I/O.
// A context watcher hands out AutoStart=false clients, so operations on them
// never start the adb server either.
func (w *DeviceWatcher) Device(serial string) *Device {
	return (&Adb{server: w.server}).Device(DeviceWithSerial(serial))
}

// Err returns the error that closed the channel returned by C. Shutdown and
// parent cancellation are clean exits and leave it nil.
func (w *DeviceWatcher) Err() error {
	if err, ok := w.err.Load().(error); ok {
		return err
	}
	return nil
}

// Shutdown stops the watcher, closes the channel returned from C and waits for
// the worker to exit. It is idempotent and safe to call from any goroutine.
func (w *DeviceWatcher) Shutdown() {
	w.cancel()
	w.closeConn()
	<-w.done
}

func (w *deviceWatcherImpl) closeConn() {
	w.connMu.Lock()
	conn := w.conn
	w.conn = nil
	w.connMu.Unlock()
	if conn != nil {
		_ = conn.Close()
	}
}

func (w *deviceWatcherImpl) attachConn(conn wire.IConn) error {
	w.connMu.Lock()
	defer w.connMu.Unlock()
	if err := w.ctx.Err(); err != nil {
		_ = conn.Close()
		return err
	}
	w.conn = conn
	return nil
}

// send delivers an event, applying backpressure until the consumer reads it.
// It returns false once the watcher is cancelled, so a slow or absent consumer
// can never pin the worker or make it drop events.
func (w *deviceWatcherImpl) send(event DeviceStateChangedEvent) bool {
	select {
	case <-w.ctx.Done():
		return false
	case w.eventChan <- event:
		return true
	}
}

// reportErr records a terminal error. Cancellation is a clean exit, so Err()
// stays nil after Shutdown; a parent deadline is still an error.
func (w *deviceWatcherImpl) reportErr(err error) {
	if err != nil && !errors.Is(err, context.Canceled) {
		w.err.Store(err)
	}
}

// publishDevices reads device lists from the server, calculates diffs, and
// publishes events on eventChan until the watcher is cancelled or hits a
// terminal error.
func publishDevices(watcher *deviceWatcherImpl) {
	defer close(watcher.done)
	defer close(watcher.eventChan)
	defer watcher.cancel()
	defer watcher.closeConn()
	// Closing the connection is what unblocks a read or handshake that is
	// already in flight when the caller cancels.
	defer context.AfterFunc(watcher.ctx, watcher.closeConn)()

	var lastKnownStates map[string]DeviceState
	var outageStart time.Time
	failures := 0

	for {
		if err := watcher.ctx.Err(); err != nil {
			watcher.reportErr(err)
			return
		}

		gotList := false
		conn, err := connectToTrackDevices(watcher)
		if err == nil {
			gotList, err = publishDevicesUntilError(conn, watcher, &lastKnownStates)
		}
		watcher.closeConn()

		if ctxErr := watcher.ctx.Err(); ctxErr != nil {
			watcher.reportErr(ctxErr)
			return
		}
		if gotList {
			outageStart, failures = time.Time{}, 0
		}

		if watcher.reconnect {
			if outageStart.IsZero() {
				outageStart = time.Now()
			}
			// A brief interruption is not a device disconnect: keep the last
			// list so that reconnecting re-diffs against it and produces no
			// events at all. Only a sustained outage evicts, and only once.
			if len(lastKnownStates) > 0 && time.Since(outageStart) >= watcher.offlineAfter {
				for serial, state := range lastKnownStates {
					if !watcher.send(DeviceStateChangedEvent{serial, state, StateDisconnected}) {
						watcher.reportErr(watcher.ctx.Err())
						return
					}
				}
				lastKnownStates = nil
			}
			failures++
			delay := watcher.backoff(failures)
			watcher.logRetry(err, delay)
			if !watcher.wait(delay) {
				watcher.reportErr(watcher.ctx.Err())
				return
			}
			continue
		}

		if !errors.Is(err, wire.ErrConnectionReset) {
			// Unknown error, don't retry.
			watcher.reportErr(err)
			return
		}

		// The server died, restart and reconnect.
		if realServer, ok := watcher.server.(*realServer); ok {
			if !realServer.config.AutoStart {
				watcher.reportErr(fmt.Errorf("server killed"))
				return
			}
		}

		// report all devices removed
		for serial, deviceState := range lastKnownStates {
			if !watcher.send(DeviceStateChangedEvent{serial, deviceState, StateDisconnected}) {
				watcher.reportErr(watcher.ctx.Err())
				return
			}
		}
		lastKnownStates = nil

		// Delay by a random [0ms, 500ms) in case multiple DeviceWatchers are trying to
		// start the same server.
		delay := time.Duration(rand.Intn(500)) * time.Millisecond
		log.Printf("[DeviceWatcher] server died, restarting in %s…", delay)
		if !watcher.wait(delay) {
			watcher.reportErr(watcher.ctx.Err())
			return
		}
		if err := watcher.server.Start(); err != nil {
			log.Println("[DeviceWatcher] error restarting server, giving up")
			watcher.reportErr(err)
			return
		} // Else server should be running, continue listening.
	}
}

// backoff grows retryDelay towards maxRetryDelay and subtracts up to 20% of
// jitter, so agents restarted together do not retry in lockstep.
func (w *deviceWatcherImpl) backoff(failures int) time.Duration {
	delay := w.retryDelay
	for i := 1; i < failures && delay < w.maxRetryDelay; i++ {
		delay *= 2
	}
	if delay > w.maxRetryDelay {
		delay = w.maxRetryDelay
	}
	return delay - time.Duration(rand.Int63n(int64(delay)/5+1))
}

// logRetry reports an ongoing outage at most once a minute. Reconnecting is
// expected, so it must not flood the log of a host whose server is down.
func (w *deviceWatcherImpl) logRetry(err error, delay time.Duration) {
	if !w.lastLog.IsZero() && time.Since(w.lastLog) < time.Minute {
		return
	}
	w.lastLog = time.Now()
	log.Printf("[DeviceWatcher] track-devices interrupted: %v; retrying in %s", err, delay)
}

func (w *deviceWatcherImpl) wait(delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-w.ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func connectToTrackDevices(w *deviceWatcherImpl) (wire.IConn, error) {
	conn, err := w.server.Dial()
	if err != nil {
		return nil, err
	}
	if err := w.attachConn(conn); err != nil {
		return nil, err
	}
	// publishDevicesUntilError clears this deadline once the list arrives.
	if err := conn.SetDeadline(time.Now().Add(trackHandshakeTimeout)); err != nil {
		return nil, err
	}
	if err := conn.SendMessage([]byte("host:track-devices")); err != nil {
		return nil, err
	}
	if _, err := conn.ReadStatus("host:track-devices"); err != nil {
		return nil, err
	}
	return conn, nil
}

// publishDevicesUntilError reports whether at least one device list was read,
// which is what tells the caller the server is healthy again.
func publishDevicesUntilError(conn wire.IConn, w *deviceWatcherImpl, lastKnownStates *map[string]DeviceState) (bool, error) {
	gotList := false
	for {
		msg, err := conn.ReadMessage()
		if err != nil {
			return gotList, err
		}
		if !gotList {
			// An established track stream is idle by design and must not
			// expire. A failure here means the connection is already gone, and
			// the next read reports it; the list just received is still valid,
			// so it must not be discarded.
			_ = conn.SetDeadline(time.Time{})
		}

		deviceStates, err := parseDeviceStates(string(msg))
		if err != nil {
			return gotList, err
		}
		gotList = true

		for _, event := range calculateStateDiffs(*lastKnownStates, deviceStates) {
			if !w.send(event) {
				return gotList, w.ctx.Err()
			}
		}
		*lastKnownStates = deviceStates
	}
}

func parseDeviceStates(msg string) (map[string]DeviceState, error) {
	states := make(map[string]DeviceState)

	for lineNum, line := range strings.Split(msg, "\n") {
		if len(line) == 0 {
			continue
		}

		fields := strings.Split(line, "\t")
		if len(fields) != 2 {
			return nil, fmt.Errorf("%w: invalid device state line %d: %s", wire.ErrParse, lineNum, line)
		}
		states[fields[0]] = parseDeviceState(fields[1])
	}

	return states, nil
}

func calculateStateDiffs(oldStates, newStates map[string]DeviceState) (events []DeviceStateChangedEvent) {
	for serial, newState := range newStates {
		if oldState, ok := oldStates[serial]; ok {
			// Device present in both lists: state changed.
			if oldState != newState {
				events = append(events, DeviceStateChangedEvent{serial, oldState, newState})
			}
		} else {
			// Device only present in new list: device added.
			events = append(events, DeviceStateChangedEvent{serial, StateDisconnected, newState})
		}
	}

	for serial, oldState := range oldStates {
		if _, ok := newStates[serial]; !ok {
			// Device only present in old list: device removed.
			events = append(events, DeviceStateChangedEvent{serial, oldState, StateDisconnected})
		}
	}

	return events
}
