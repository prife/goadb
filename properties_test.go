package adb

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_parseDeviceProperties(t *testing.T) {
	props := []byte(`
[ro.vendor.build.date]: [Thu Apr 18 22:16:50 CST 2024]
[ro.vendor.build.date.utc]: [1713449810]
[ro.vendor.build.fingerprint]: [OnePlus/PJD110/OP5929L1:14/UKQ1.230924.001/U.17a89d1_3ff5_3ff4:user/release-keys]
[ro.build.description]:    [msm8916_64-user 5.1.1 LMY47V eng.root.20161104.171401 dev-keys]
`)

	m := parseDeviceProperties(props, nil)
	assert.Equal(t, len(m), 4)
	for k, v := range m {
		fmt.Printf("[%s]: [%s]\n", k, v)
	}
}

func TestDevice_GetProperites(t *testing.T) {
	assert.NotNil(t, adbclient)
	d := adbclient.Device(AnyDevice())
	m, err := d.GetProperties(func(k, v string) bool {
		return strings.HasPrefix(k, "ro.")
	})
	assert.Nil(t, err)
	// for k, v := range m {
	// 	fmt.Printf("[%s]: [%s]\n", k, v)
	// }

	level, err := m.SdkLevel()
	assert.Nil(t, err)
	fmt.Println("level:", level)

	buildVersion, err := m.BuildVersion()
	assert.Nil(t, err)
	fmt.Println("BuildVersion:", buildVersion)

	manu, err := m.ProductManufacturer()
	assert.Nil(t, err)
	fmt.Println("manu:", manu)

	brand, err := m.ProductBrand()
	assert.Nil(t, err)
	fmt.Println("brand:", brand)

	model, err := m.ProductModel()
	assert.Nil(t, err)
	fmt.Println("model:", model)

	name, err := m.ProductName()
	assert.Nil(t, err)
	fmt.Println("name:", name)

	serial, err := m.Serial()
	assert.Nil(t, err)
	fmt.Println("serial:", serial)

	booted, err := d.BootCompleted()
	assert.Nil(t, err)
	fmt.Println("booted:", booted)
}
