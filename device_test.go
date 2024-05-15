package adb

import (
	"errors"
	"testing"

	"github.com/prife/goadb/wire"
	"github.com/stretchr/testify/assert"
)

func TestGetAttribute(t *testing.T) {
	s := &MockServer{
		Status:   wire.StatusSuccess,
		Messages: []string{"value"},
	}
	client := (&Adb{s}).Device(DeviceWithSerial("serial"))

	v, err := client.getAttribute("attr")
	assert.Equal(t, "host-serial:serial:attr", s.Requests[0])
	assert.NoError(t, err)
	assert.Equal(t, "value", v)
}

func TestGetDeviceInfo(t *testing.T) {
	deviceLister := func() ([]*DeviceInfo, error) {
		return []*DeviceInfo{
			&DeviceInfo{
				Serial:  "abc",
				Product: "Foo",
			},
			&DeviceInfo{
				Serial:  "def",
				Product: "Bar",
			},
		}, nil
	}

	client := newDeviceClientWithDeviceLister("abc", deviceLister)
	device, err := client.DeviceInfo()
	assert.NoError(t, err)
	assert.Equal(t, "Foo", device.Product)

	client = newDeviceClientWithDeviceLister("def", deviceLister)
	device, err = client.DeviceInfo()
	assert.NoError(t, err)
	assert.Equal(t, "Bar", device.Product)

	client = newDeviceClientWithDeviceLister("serial", deviceLister)
	device, err = client.DeviceInfo()
	assert.True(t, errors.Is(err, wire.ErrDeviceNotFound))
	assert.EqualError(t, errors.Unwrap(err),
		"DeviceNotFound: device list doesn't contain serial serial")
	assert.Nil(t, device)
}

func newDeviceClientWithDeviceLister(serial string, deviceLister func() ([]*DeviceInfo, error)) *Device {
	client := (&Adb{&MockServer{
		Status:   wire.StatusSuccess,
		Messages: []string{serial},
	}}).Device(DeviceWithSerial(serial))
	client.deviceListFunc = deviceLister
	return client
}

func TestRunCommandNoArgs(t *testing.T) {
	s := &MockServer{
		Status:   wire.StatusSuccess,
		Messages: []string{"output"},
	}
	client := (&Adb{s}).Device(AnyDevice())

	v, err := client.RunCommand("cmd")
	assert.Equal(t, "host:transport-any", s.Requests[0])
	assert.Equal(t, "shell:cmd", s.Requests[1])
	assert.NoError(t, err)
	assert.Equal(t, "output", v)
}

func TestPrepareCommandLineNoArgs(t *testing.T) {
	result, err := prepareCommandLine("cmd")
	assert.NoError(t, err)
	assert.Equal(t, "cmd", result)
}

func TestPrepareCommandLineEmptyCommand(t *testing.T) {
	_, err := prepareCommandLine("")
	assert.True(t, errors.Is(err, wire.ErrAssertion))
	assert.Contains(t, err.Error(), "command cannot be empty")
}

func TestPrepareCommandLineBlankCommand(t *testing.T) {
	_, err := prepareCommandLine("  ")
	assert.True(t, errors.Is(err, wire.ErrAssertion))
	assert.Contains(t, err.Error(), "command cannot be empty")
}

func TestPrepareCommandLineCleanArgs(t *testing.T) {
	result, err := prepareCommandLine("cmd", "arg1", "arg2")
	assert.NoError(t, err)
	assert.Equal(t, "cmd arg1 arg2", result)
}

func TestPrepareCommandLineArgWithWhitespaceQuotes(t *testing.T) {
	result, err := prepareCommandLine("cmd", "arg with spaces")
	assert.NoError(t, err)
	assert.Equal(t, "cmd \"arg with spaces\"", result)
}

func TestPrepareCommandLineArgWithDoubleQuoteFails(t *testing.T) {
	_, err := prepareCommandLine("cmd", "quoted\"arg")
	assert.True(t, errors.Is(err, wire.ErrParse))
	assert.Contains(t, err.Error(), "arg at index 0 contains an invalid double quote: quoted\"arg")
}
