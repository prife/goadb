package adb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func ParseDeviceList(t *testing.T) {
	devs, err := parseDeviceList(`192.168.56.101:5555	device
05856558`, parseDeviceShort)

	assert.NoError(t, err)
	assert.Len(t, devs, 2)
	assert.Equal(t, "192.168.56.101:5555", devs[0].Serial)
	assert.Equal(t, "05856558", devs[1].Serial)
}

func TestParseDeviceShort(t *testing.T) {
	dev, err := parseDeviceShort("192.168.56.101:5555	device\n")
	assert.NoError(t, err)
	assert.Equal(t, &DeviceInfo{
		Serial: "192.168.56.101:5555",
		State:  "device"}, dev)
}

func TestParseDeviceLong(t *testing.T) {
	dev, err := parseDeviceLong("SERIAL    device product:PRODUCT model:MODEL device:DEVICE\n")
	assert.NoError(t, err)
	assert.Equal(t, &DeviceInfo{
		Serial:     "SERIAL",
		State:      "device",
		Product:    "PRODUCT",
		Model:      "MODEL",
		DeviceInfo: "DEVICE"}, dev)
}

func TestParseDeviceLongUnauthorized(t *testing.T) {
	dev, err := parseDeviceLong("SERIAL    unauthorized usb:1234 transport_id:8")
	assert.NoError(t, err)
	assert.Equal(t, &DeviceInfo{
		Serial:      "SERIAL",
		State:       "unauthorized",
		Usb:         "1234",
		TransportID: 8}, dev)
}

func TestParseDeviceLongUsb(t *testing.T) {
	dev, err := parseDeviceLong("SERIAL    device usb:1234 product:PRODUCT model:MODEL device:DEVICE \n")
	assert.NoError(t, err)
	assert.Equal(t, &DeviceInfo{
		Serial:     "SERIAL",
		State:      "device",
		Product:    "PRODUCT",
		Model:      "MODEL",
		DeviceInfo: "DEVICE",
		Usb:        "1234"}, dev)
}

func Test_parseDeviceLongE(t *testing.T) {
	tests := []struct {
		line string
		want *DeviceInfo
	}{
		{
			"SERIAL device product:PRODUCT   model:MODEL   device:DEVICE", &DeviceInfo{
				Serial:     "SERIAL",
				State:      "device",
				Product:    "PRODUCT",
				Model:      "MODEL",
				DeviceInfo: "DEVICE",
			},
		},
		{
			"UYT5T18414003349       unauthorized usb:1114112X transport_id:23", &DeviceInfo{
				Serial:      "UYT5T18414003349",
				State:       "unauthorized",
				Usb:         "1114112X",
				TransportID: 23,
			},
		},
		{
			"UYT5T18414003349       device usb:1114112X product:ALP_AL00 model:ALP_AL00 device:HWALP transport_id:23", &DeviceInfo{
				Serial:      "UYT5T18414003349",
				State:       "device",
				Usb:         "1114112X",
				Product:     "ALP_AL00",
				Model:       "ALP_AL00",
				DeviceInfo:  "HWALP",
				TransportID: 23,
			},
		},
		{
			"UYT5T18414003349       device usb:1114112X product:ALP AL00 model:ALP AL00 device:HWALP transport_id:23", &DeviceInfo{
				Serial:      "UYT5T18414003349",
				State:       "device",
				Usb:         "1114112X",
				Product:     "ALP AL00",
				Model:       "ALP AL00",
				DeviceInfo:  "HWALP",
				TransportID: 23,
			},
		},
		{
			"119.29.201.189:41012   offline product:PRODUCT model:MODEL device:DEVICE transport_id:24", &DeviceInfo{
				Serial:      "119.29.201.189:41012",
				State:       "offline",
				Product:     "PRODUCT",
				Model:       "MODEL",
				DeviceInfo:  "DEVICE",
				TransportID: 24,
			},
		},
	}

	for _, tt := range tests {
		dev, err := parseDeviceLongE(tt.line)
		assert.NoError(t, err)
		assert.Equal(t, tt.want, dev)
	}
}
