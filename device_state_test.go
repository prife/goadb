package adb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseDeviceState(t *testing.T) {
	for _, test := range []struct {
		String    string
		WantState DeviceState
		WantName  string
	}{
		{"", StateDisconnected, "StateDisconnected"},
		{"offline", StateOffline, "StateOffline"},
		{"device", StateOnline, "StateOnline"},
		{"unauthorized", StateUnauthorized, "StateUnauthorized"},
		{"authorizing", StateAuthorizing, "StateAuthorizing"},
		{"host", StateHost, "StateHost"},
		{" device\r", StateOnline, "StateOnline"},
		// Modes adb reports that cannot serve ordinary commands.
		{"recovery", StateOffline, "StateOffline"},
		{"bootloader", StateOffline, "StateOffline"},
		{"sideload", StateOffline, "StateOffline"},
		{"connecting", StateOffline, "StateOffline"},
		{"no permissions (user in plugdev group; are your udev rules wrong?)", StateOffline, "StateOffline"},
		{"some-future-state", StateOffline, "StateOffline"},
	} {
		state := parseDeviceState(test.String)
		assert.Equal(t, test.WantState, state, test.String)
		assert.Equal(t, test.WantName, state.String(), test.String)
	}
}
