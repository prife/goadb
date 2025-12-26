package adb

import (
	"strings"
)

func checkNameValid(name string) bool {
	return !(name == "" || name == "null" || strings.Contains(strings.ToLower(name), "error"))
}

// see: https://stackoverflow.com/questions/16704597/how-do-you-get-the-user-defined-device-name-in-android
func (d *Device) GetDeviceName() (name string, err error) {
	// fist try
	resp, err := d.RunCommand("settings get global device_name")
	if err != nil {
		return
	}
	name = string(resp)
	if checkNameValid(name) {
		return
	}

	// try again
	resp, err = d.RunCommand("settings get secure bluetooth_name")
	if err != nil {
		return
	}
	name = string(resp)
	if checkNameValid(name) {
		return
	}

	// final try
	name, err = d.GetProperty(PropProductName)
	return
}

// GetMarketName returns the market name of the device.
func (d *Device) GetMarketName() (name string, err error) {
	candidates := []string{
		// 高优先级
		"ro.product.marketname",
		"ro.config.marketing_name",
		// 品牌特有
		"ro.vendor.oplus.market.name",
		"ro.vivo.market.name",
		// 保底-低优先级
		"ro.product.name",
		"ro.product.model",
	}

	for _, prop := range candidates {
		name, _ = d.GetProperty(prop)
		if checkNameValid(name) {
			return
		}
	}
	return
}

func (d *Device) SetAccelerometerRotation(enable bool) error {
	var value string
	if enable {
		value = "1"
	} else {
		value = "0"
	}
	_, err := d.RunCommand("settings put system accelerometer_rotation " + value)
	return err
}
