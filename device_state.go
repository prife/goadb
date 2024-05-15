package adb

import (
	"fmt"

	"github.com/prife/goadb/wire"
)

// DeviceState represents one of the 3 possible states adb will report devices.
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
}

func parseDeviceState(str string) (DeviceState, error) {
	state, ok := deviceStateStrings[str]
	if !ok {
		return StateInvalid, fmt.Errorf("%w: invalid device state: %s", wire.ErrParse, str)
	}
	return state, nil
}
