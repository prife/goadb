package adb

import (
	"fmt"
	"testing"

	"github.com/prife/goadb/wire"
	"github.com/stretchr/testify/assert"
)

func TestParseDeviceLongAdbdPort(t *testing.T) {
	for _, parse := range []struct {
		name string
		fn   func(string) (*DeviceInfo, error)
	}{{"long", parseDeviceLong}, {"longE", parseDeviceLongE}} {
		for _, tc := range []struct {
			value string
			want  int
		}{{"5555", 5555}, {"5556", 5556}, {"65535", 65535},
			{"", 0}, {"0", 0}, {"-1", 0}, {"65536", 0}, {"invalid", 0}} {
			t.Run(parse.name+"/"+tc.value, func(t *testing.T) {
				line := fmt.Sprintf("SERIAL device usb:1-1 transport_id:9 adbd_port:%s", tc.value)
				info, err := parse.fn(line)
				assert.NoError(t, err)
				assert.Equal(t, tc.want, info.AdbdPort)
				assert.Equal(t, "1-1", info.Usb)
				assert.Equal(t, 9, info.TransportID)
			})
		}
	}
}

func TestListDevicesAdbdPort(t *testing.T) {
	s := &MockServer{Status: wire.StatusSuccess, Messages: []string{
		"USB_A device usb:1-1 transport_id:1 adbd_port:5555\n" +
			"USB_B device usb:1-2 transport_id:2 adbd_port:5556\n" +
			"STANDARD device usb:1-3 transport_id:3\n",
	}}
	devices, err := (&Adb{s}).ListDevices()
	assert.NoError(t, err)
	if !assert.Len(t, devices, 3) {
		return
	}
	assert.Equal(t, 5555, devices[0].AdbdPort)
	assert.Equal(t, 5556, devices[1].AdbdPort)
	assert.Zero(t, devices[2].AdbdPort)
	assert.Equal(t, []string{"host:devices-l"}, s.Requests)
}

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
