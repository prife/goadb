package adb

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_parseUptime(t *testing.T) {
	upttimestr := "73681.99 69586.45"
	uptime, err := parseUptime([]byte(upttimestr))
	assert.Nil(t, err)
	assert.Equal(t, uptime, float64(73681.99))

	upttimestr = "\x0D\x0A73681.99 69586.45\x0D\x0A"
	uptime, err = parseUptime([]byte(upttimestr))
	assert.Nil(t, err)
	assert.Equal(t, uptime, float64(73681.99))

	upttimestr = "\x0D\x0A73681.99    69586.45\x0D\x0A"
	uptime, err = parseUptime([]byte(upttimestr))
	assert.Nil(t, err)
	assert.Equal(t, uptime, float64(73681.99))
}

func TestDevice_Uptime(t *testing.T) {
	assert.NotNil(t, adbclient)
	d := adbclient.Device(AnyDevice())
	uptime, err := d.Uptime()
	assert.Nil(t, err)
	fmt.Println(uptime / 3600)
}

func Test_parseUname(t *testing.T) {
	// xiaomi 6, Android 9
	versionStr := "Linux version 4.4.153-perf+ (builder@c3-miui-ota-bd114.bj) (gcc version 4.9.x 20150123 (prerelease) (GCC) ) #1 SMP PREEMPT Thu Mar 5 11:28:37 CST 2020"
	info, err := parseUname([]byte(versionStr))
	assert.Nil(t, err)
	assert.Equal(t, info.Version, "4.4.153")
	built, _ := time.Parse("Mon Jan 2 15:04:05 MST 2006", "Thu Mar 5 11:28:37 CST 2020")
	assert.Equal(t, info.Built, built)

	// huawei hormonyOS 2.0
	versionStr = "Linux version 4.14.116 (HarmonyOS@localhost) (Android (5484270 based on r353983c) clang version 9.0.3 (https://android.googlesource.com/toolchain/clang 745b335211bb9eadfa6aa6301f84715cee4b37c5) (https://android.googlesource.com/toolchain/llvm 60cf23e54e46c807513f7a36d0a7b777920b5881) (based on LLVM 9.0.3svn)) #1 SMP PREEMPT Tue Mar 22 17:09:22 CST 2022"
	info, err = parseUname([]byte(versionStr))
	assert.Nil(t, err)
	assert.Equal(t, info.Version, "4.14.116")
	built, _ = time.Parse("Mon Jan 2 15:04:05 MST 2006", "Tue Mar 22 17:09:22 CST 2022")
	assert.Equal(t, info.Built, built)

	// a59, android5.1
	versionStr = "\x0d\x0aLinux version 3.10.72+ (root@ubuntu-121-147) (gcc version 4.9 20140514 (mtk-20150408) (GCC) ) #1 SMP PREEMPT Wed Dec 18 20:06:03 CST 2019\x0d\x0a"
	info, err = parseUname([]byte(versionStr))
	assert.Nil(t, err)
	assert.Equal(t, info.Version, "3.10.72")
	built, _ = time.Parse("Mon Jan 2 15:04:05 MST 2006", "Wed Dec 18 20:06:03 CST 2019")
	assert.Equal(t, info.Built, built)
}

func TestDevice_Uname(t *testing.T) {
	assert.NotNil(t, adbclient)
	d := adbclient.Device(AnyDevice())
	info, err := d.Uname()
	assert.Nil(t, err)
	fmt.Println(info)
}
