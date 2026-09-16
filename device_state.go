package adb

import "strings"

// DeviceState represents a device's availability for ordinary adb operations.
// A device can be communicated with when it's in StateOnline.
// A USB device will make the following state transitions:
//	Plugged in: StateDisconnected->StateOffline->StateOnline
//	Unplugged:  StateOnline->StateDisconnected

//go:generate stringer -type=DeviceState
type DeviceState int8

const (
	StateInvalid DeviceState = iota
	StateUnauthorized
	StateAuthorizing
	StateDisconnected
	StateOffline
	StateOnline
	StateHost
)

var deviceStateStrings = map[string]DeviceState{
	"":             StateDisconnected,
	"offline":      StateOffline,
	"device":       StateOnline,
	"unauthorized": StateUnauthorized,
	"authorizing":  StateAuthorizing,
	"host":         StateHost,
	"bootloader":   StateOffline,
	"recovery":     StateOffline,
	"sideload":     StateOffline,
	"connecting":   StateOffline,
}

// parseDeviceState maps a state reported by the server. Unrecognized states are
// StateOffline: adb keeps adding them ("unknown", the multi-word no-permissions
// text, ...) and none of them are usable, so a single odd device must not fail
// the whole device list.
func parseDeviceState(str string) DeviceState {
	if state, ok := deviceStateStrings[strings.TrimSpace(str)]; ok {
		return state
	}
	return StateOffline
}
