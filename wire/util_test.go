package wire

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAdbServerError_NoRequest(t *testing.T) {
	err := adbServerError("", "fail")
	assert.True(t, errors.Is(err, ErrAdb))
	assert.EqualError(t, err, "AdbError: request , server error: fail")
}

func TestAdbServerError_WithRequest(t *testing.T) {
	err := adbServerError("polite", "fail")
	assert.True(t, errors.Is(err, ErrAdb))
	assert.EqualError(t, err, "AdbError: request polite, server error: fail")
}

func TestAdbServerError_DeviceNotFound(t *testing.T) {
	err := adbServerError("", "device not found")
	assert.True(t, errors.Is(err, ErrDeviceNotFound))
	assert.EqualError(t, err, "DeviceNotFound: request , server error: device not found")
}

func TestAdbServerError_DeviceSerialNotFound(t *testing.T) {
	err := adbServerError("", "device 'LGV4801c74eccd' not found")
	assert.True(t, errors.Is(err, ErrDeviceNotFound))
	assert.EqualError(t, err, "DeviceNotFound: request , server error: device 'LGV4801c74eccd' not found")
}
