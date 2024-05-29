package adb

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"

	"github.com/Masterminds/semver"
)

const (
	PropSysBootCompleted       = "sys.boot_completed"
	PropSerial                 = "ro.serialno"
	PropProductName            = "ro.product.name"
	PropProductBrand           = "ro.product.brand"
	PropProductModel           = "ro.product.model"
	PropProductManu            = "ro.product.manufacturer"
	PropProductCpuAbi          = "ro.product.cpu.abi"
	PropRroductBuildVersionSdk = "ro.product.build.version.sdk" // api level
	PropBuildVersionRelease    = "ro.build.version.release"     // android os version
	// for vendor
	PropHwPlatformVersion = "hw_sc.build.platform.version"
)

var (
	devicePropertyRegex = regexp.MustCompile(`(?m)\[(\S+)\]:\s*\[(.*)\]\s*$`)
	// devicePropertyRegex = regexp.MustCompile(`\[([\s\S]*?)\]: \[([\s\S]*?)\]\r?`)
)

var (
	ErrNotFound = errors.New("NotFound")
)

type PropertiesFilter func(k, v string) bool
type AndroidProperties map[string]string

func parseDeviceProperties(resp []byte, filter PropertiesFilter) map[string]string {
	matches := devicePropertyRegex.FindAllSubmatch(resp, -1)
	properties := make(map[string]string)
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		key := string(match[1])
		value := string(match[2])

		if filter == nil || filter(key, value) {
			properties[key] = value
		}
	}
	return properties
}

// GetProperites adb shell getprop
func (d *Device) GetProperites(filter PropertiesFilter) (properties AndroidProperties, err error) {
	resp, err := d.RunCommand("getprop")
	if err != nil {
		return
	}

	properties = parseDeviceProperties(resp, filter)
	if len(properties) == 0 {
		err = fmt.Errorf("not found any properties")
	}
	return
}

// GetProperites adb shell getprop
func (d *Device) SetProperty(key, value string) (err error) {
	resp, err := d.RunCommand("setprop", key, value)
	_ = resp
	return
}

func (a AndroidProperties) GetMapValue(key string) (string, error) {
	if v, ok := a[key]; ok {
		return v, nil
	} else {
		return "", fmt.Errorf("getprop %s: %w", PropRroductBuildVersionSdk, ErrNotFound)
	}
}

func (a AndroidProperties) Serial() (string, error) {
	return a.GetMapValue(PropSerial)
}

func (a AndroidProperties) ProductName() (string, error) {
	return a.GetMapValue(PropProductName)
}

func (a AndroidProperties) ProductBrand() (string, error) {
	return a.GetMapValue(PropProductBrand)
}

func (a AndroidProperties) ProductManufacturer() (string, error) {
	return a.GetMapValue(PropProductManu)
}

func (a AndroidProperties) ProductModel() (string, error) {
	return a.GetMapValue(PropProductModel)
}

func (a AndroidProperties) SdkLevel() (int, error) {
	sdkstr, err := a.GetMapValue(PropRroductBuildVersionSdk)
	if err != nil {
		return 0, err
	}
	v, err := strconv.Atoi(sdkstr)
	if err != nil {
		return 0, fmt.Errorf("parse 'getprop %s': %w", PropRroductBuildVersionSdk, err)
	}
	return v, nil
}

func (a AndroidProperties) BuildVersion() (version *semver.Version, err error) {
	versionStr, err := a.GetMapValue(PropBuildVersionRelease)
	if err != nil {
		return
	}
	return semver.NewVersion(versionStr)
}