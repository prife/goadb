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

func Test_parseGpu(t *testing.T) {
	gpuStr := "GLES: Qualcomm, Adreno (TM) 618, OpenGL ES 3.2 V@415.0 (GIT@663be55, I724753c5e3, 1573037262) (Date:11/06/19)"
	info, err := parseGpu([]byte(gpuStr))
	assert.Nil(t, err)
	assert.Equal(t, info, GpuInfo{
		Vendor:        "Qualcomm",
		Model:         "Adreno (TM) 618",
		OpenGLVersion: "OpenGL ES 3.2",
	})

	gpuStr = "GLES: ARM, Mali-G78, OpenGL ES 3.2 v1.r34p0-01eac0.a1b116bd871d46ef040e8feef9ed691e"
	info, err = parseGpu([]byte(gpuStr))
	assert.Nil(t, err)
	assert.Equal(t, info, GpuInfo{
		Vendor:        "ARM",
		Model:         "Mali-G78",
		OpenGLVersion: "OpenGL ES 3.2",
	})

	gpuStr = `------------RE GLES------------
GLES: Qualcomm, Adreno (TM) 750, OpenGL ES 3.2 V@0762.10 (GIT@1394a2c7a8, Id12349e41b, 1708672982) (Date:02/23/24)
`
	info, err = parseGpu([]byte(gpuStr))
	assert.Nil(t, err)
	assert.Equal(t, info, GpuInfo{
		Vendor:        "Qualcomm",
		Model:         "Adreno (TM) 750",
		OpenGLVersion: "OpenGL ES 3.2",
	})
}

func Test_parseIpAddressWlan0(t *testing.T) {
	// shell@A33:/ $
	output := []byte(`
29: wlan0: <NO-CARRIER,BROADCAST,MULTICAST,UP> mtu 1500 qdisc mq state DOWN qlen 1000
link/ether 2c:5b:b8:e5:d6:12 brd ff:ff:ff:ff:ff:ff
`)
	m, _ := parseIpAddressWlan0(output)
	assert.Equal(t, m.LinkAddr, "2c:5b:b8:e5:d6:12")
	assert.True(t, m.Ipv4 == nil)
	assert.True(t, m.Ipv6 == nil)

	// Android 14
	output = []byte(`
	24: wlan0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc htb state UP group default qlen 3000
	link/ether 62:7b:0f:61:b2:d6 brd ff:ff:ff:ff:ff:ff
	inet 192.168.31.222/24 brd 192.168.31.255 scope global wlan0
		valid_lft forever preferred_lft forever
	inet6 fe80::607b:fff:fe61:b2d6/64 scope link
		valid_lft forever preferred_lft forever`)
	m, _ = parseIpAddressWlan0(output)
	assert.Equal(t, m.LinkAddr, "62:7b:0f:61:b2:d6")
	assert.Equal(t, m.Ipv4, []byte("192.168.31.222/24"))
	assert.Equal(t, m.Ipv6, []byte("fe80::607b:fff:fe61:b2d6/64"))
}

func TestDevice_GetWlanInfo(t *testing.T) {
	assert.NotNil(t, adbclient)
	d := adbclient.Device(AnyDevice())
	info, err := d.GetWlanInfo()
	assert.Nil(t, err)
	fmt.Println(info)
}

func Test_parseMemoryInfo(t *testing.T) {
	output := []byte(`
MemTotal:       23794344 kB
MemFree:          430780 kB
MemAvailable:   17117864 kB
Buffers:            8628 kB
Cached:         12901756 kB
SwapCached:        27376 kB
SecPageTables:         0 kB
NFS_Unstable:          0 kB
Bounce:                0 kB
WritebackTmp:          0 kB
`)

	info, err := parseMemoryInfo(output)
	assert.Nil(t, err)
	assert.Equal(t, len(info), 10)
	for k, v := range info {
		fmt.Printf("%-16s %9d kB\n", k, v)
	}

	assert.Equal(t, info["MemTotal"], uint64(23794344))
	assert.Equal(t, info["MemFree"], uint64(430780))
	assert.Equal(t, info["MemAvailable"], uint64(17117864))
}
