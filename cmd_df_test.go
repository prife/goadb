package adb

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_parseDfNumber(t *testing.T) {
	v, err := parseDfNumber("956.5M")
	assert.Nil(t, err)
	assert.Equal(t, v, 956.5*1024*1024)

	v, err = parseDfNumber("148.0K")
	assert.Nil(t, err)
	assert.Equal(t, v, 148.0*1024)

	v, err = parseDfNumber("4096")
	assert.Nil(t, err)
	assert.Equal(t, v, float64(4096))

	v, err = parseDfNumber("2.9G")
	assert.Nil(t, err)
	assert.Equal(t, v, 2.9*1024*1024*1024)

	v, err = parseDfNumber("0.0K")
	assert.Nil(t, err)
	assert.Equal(t, v, float64(0))

	v, err = parseDfNumber("0")
	assert.Nil(t, err)
	assert.Equal(t, v, float64(0))
}

func dfNumberToString(number float64) string {
	if number >= 1024*1024*1024 {
		return fmt.Sprintf("%.1fG", number/1024/1024/1024)
	} else if number >= 1024*1024 {
		return fmt.Sprintf("%.1fM", number/1024/1024)
	} else {
		return fmt.Sprintf("%.1fK", number/1024)
	}
}

func Test_unpackDfV1_A59_Android51(t *testing.T) {
	var dfOutputV1_Android51 = `shell@A59:/ $ df
Filesystem               Size     Used     Free   Blksize
/dev                     1.8G   104.0K     1.8G   4096
/sys/fs/cgroup           1.8G    12.0K     1.8G   4096
/mnt/asec                1.8G     0.0K     1.8G   4096
/mnt/obb                 1.8G     0.0K     1.8G   4096
/storage/emulated        1.8G     0.0K     1.8G   4096
/system                  1.9G     1.9G    53.7M   4096
/data                   26.1G     8.1G    18.0G   4096
/cache                 248.0M     2.4M   245.6M   4096
/protect_f               3.9M    68.0K     3.8M   4096
/protect_s              11.2M    60.0K    11.2M   4096
/nvdata                 27.5M     6.8M    20.7M   4096
/mnt/cd-rom             20.5M    20.5M     0.0K   2048
/mnt/shell/emulated     26.1G     8.1G    17.9G   4096
`

	entries := unpackDfV1([]byte(dfOutputV1_Android51))
	assert.Equal(t, len(entries), 13)
	for _, entry := range entries {
		s := fmt.Sprintf("%-20s %8s %8s %8s", entry.FileSystem,
			dfNumberToString(entry.Size), dfNumberToString(entry.Used), dfNumberToString(entry.Avail))
		fmt.Println(s)
		assert.Contains(t, dfOutputV1_Android51, s)
	}
}

func Test_unpackDfV1_A33_Android51(t *testing.T) {
	var dfOutputV1_Android51 = "/dev                   934.4M    76.0K   934.3M   4096\x0d\x0a/sys/fs/cgroup         934.4M    12.0K   934.4M   4096\x0d\x0a"

	entries := unpackDfV1([]byte(dfOutputV1_Android51))
	assert.Equal(t, len(entries), 2)
	for _, entry := range entries {
		s := fmt.Sprintf("%-20s %8s %8s %8s", entry.FileSystem,
			dfNumberToString(entry.Size), dfNumberToString(entry.Used), dfNumberToString(entry.Avail))
		fmt.Println(s)
		assert.Contains(t, dfOutputV1_Android51, s)
	}
}

func Test_unpackDfV1_Android6(t *testing.T) {
	var dfOutputV1_Android6 = `Filesystem               Size     Used     Free   Blksize
/dev                   956.5M   148.0K   956.3M   4096
/sys/fs/cgroup         956.5M    12.0K   956.5M   4096
/mnt                   956.5M     0.0K   956.5M   4096
/system                  2.9G     2.6G   333.5M   4096
/cache                 192.8M   156.0K   192.7M   4096
/protect_f               5.8M    60.0K     5.8M   4096
/protect_s               9.8M    56.0K     9.7M   4096
/nvdata                 27.5M     2.0M    25.5M   4096
/cust                   93.6M     3.1M    90.5M   4096
/log                    59.0M     9.4M    49.6M   4096
/storage               956.5M     0.0K   956.5M   4096
/data                   10.9G     6.3G     4.6G   4096
/mnt/runtime/default/emulated: Permission denied
/storage/emulated       10.9G     6.3G     4.6G   4096
/mnt/runtime/read/emulated: Permission denied
/mnt/runtime/write/emulated: Permission denied`
	entries := unpackDfV1([]byte(dfOutputV1_Android6))

	dfOutputV1Data := []struct {
		Name string
		Size string
		Used string
		Free string
	}{
		{"/dev", "956.5M", "148.0K", "956.3M"},
		{"/sys/fs/cgroup", "956.5M", "12.0K", "956.5M"},
		{"/mnt", "956.5M", "0.0K", "956.5M"},
		{"/system", "2.9G", "2.6G", "333.5M"},
		{"/cache", "192.8M", "156.0K", "192.7M"},
		{"/protect_f", "5.8M", "60.0K", "5.8M"},
		{"/protect_s", "9.8M", "56.0K", "9.7M"},
		{"/nvdata", "27.5M", "2.0M", "25.5M"},
		{"/cust", "93.6M", "3.1M", "90.5M"},
		{"/log", "59.0M", "9.4M", "49.6M"},
		{"/storage", "956.5M", "0.0K", "956.5M"},
		{"/data", "10.9G", "6.3G", "4.6G"},
		{"/storage/emulated", "10.9G", "6.3G", "4.6G"},
	}
	for i, entry := range entries {
		// fmt.Printf("%s:%s size=%s avail=%s, used=%s\n", entry.FileSystem, entry.MountedOn,
		// 	dfNumberToString(entry.Size), dfNumberToString(entry.Avail), dfNumberToString(entry.Used))
		assert.Equal(t, entry.FileSystem, dfOutputV1Data[i].Name)
		assert.Equal(t, dfNumberToString(entry.Size), dfOutputV1Data[i].Size)
		assert.Equal(t, dfNumberToString(entry.Used), dfOutputV1Data[i].Used)
		assert.Equal(t, dfNumberToString(entry.Avail), dfOutputV1Data[i].Free)
	}
}

// df on Android 7.0+
func num2str(num float64) string {
	if num == float64(int(num)) {
		return fmt.Sprintf("%.0f", num) // 输出804
	} else {
		return fmt.Sprintf("%.1f", num) // 输出804.2
	}
}

func dfV2NumberToString(number float64) string {
	if number >= 1024*1024*1024 {
		return fmt.Sprintf("%sG", num2str(number/1024/1024/1024))
	} else if number >= 1024*1024 {
		return fmt.Sprintf("%sM", num2str(number/1024/1024))
	} else if number > 0 {
		return fmt.Sprintf("%sK", num2str(number/1024))
	} else {
		return "0"
	}
}

func Test_unpackDfV2_Android7(t *testing.T) {
	var dfOutputV2_Android7 = `
PD1619:/ $ df -h
Filesystem                            Size  Used Avail Use% Mounted on
tmpfs                                 2.8G  376K  2.8G   1% /dev
tmpfs                                 2.8G     0  2.8G   0% /mnt
/dev/block/dm-0                       2.8G  2.8G   28M  100% /system
/dev/block/dm-1                        58M   14M   43M  25% /oem
/dev/block/bootdevice/by-name/apps    496M  394M   92M  82% /apps
/dev/block/bootdevice/by-name/cache   248M  8.5M  234M   4% /cache
/dev/block/bootdevice/by-name/persist  27M  344K   27M   2% /persist
/dev/block/bootdevice/by-name/dsp      12M  3.8M  7.5M  34% /dsp
/dev/block/bootdevice/by-name/modem    84M   73M   11M  88% /firmware
tmpfs                                 2.8G     0  2.8G   0% /storage
/dev/block/dm-2                        53G  3.3G   49G   7% /data
/data/media                            52G  3.4G   49G   7% /storage/emulated
`
	entries := unpackDfV2([]byte(dfOutputV2_Android7))
	assert.Equal(t, len(entries), 12)
	for _, entry := range entries {
		s := fmt.Sprintf("%-37s %4s %5s %5s", entry.FileSystem,
			dfV2NumberToString(entry.Size), dfV2NumberToString(entry.Used), dfV2NumberToString(entry.Avail) /*entry.MountedOn*/)
		fmt.Println(s)
		assert.Contains(t, dfOutputV2_Android7, s)
	}
}
func Test_unpackDfV2_Android9(t *testing.T) {
	var dfOutputV2_Android9 = `
Filesystem       Size  Used Avail Use% Mounted on
rootfs           2.6G  6.1M  2.6G   1% /
tmpfs            2.7G  804K  2.7G   1% /dev
tmpfs            2.7G     0  2.7G   0% /mnt
/dev/block/dm-0  4.8G  3.2G  1.6G  67% /system
none             2.7G     0  2.7G   0% /sys/fs/cgroup
/dev/block/sda17 110G   34G   76G  32% /data
/dev/block/sde37  12M   10M  1.4M  88% /system/vendor/dsp
/dev/block/sde42 806M  237M  553M  31% /cust
/dev/block/sda16 122M  1.8M  118M   2% /cache
/dev/block/sda10 4.9M  160K  4.8M   4% /dev/logfs
/data/media      110G   34G   76G  32% /storage/emulated`
	entries := unpackDfV2([]byte(dfOutputV2_Android9))
	dfOutputData := []struct {
		Filesystem string
		Size       string
		Used       string
		Avail      string
		UsedP      string
		MountedOn  string
	}{
		{"rootfs", "2.6G", "6.1M", "2.6G", "1%", "/"},
		{"tmpfs", "2.7G", "804K", "2.7G", "1%", "/dev"},
		{"tmpfs", "2.7G", "0", "2.7G", "0%", "/mnt"},
		{"/dev/block/dm-0", "4.8G", "3.2G", "1.6G", "67%", "/system"},
		{"none", "2.7G", "0", "2.7G", "0%", "/sys/fs/cgroup"},
		{"/dev/block/sda17", "110G", "34G", "76G", "32%", "/data"},
		{"/dev/block/sde37", "12M", "10M", "1.4M", "88%", "/system/vendor/dsp"},
		{"/dev/block/sde42", "806M", "237M", "553M", "31%", "/cust"},
		{"/dev/block/sda16", "122M", "1.8M", "118M", "2%", "/cache"},
		{"/dev/block/sda10", "4.9M", "160K", "4.8M", "4%", "/dev/logfs"},
		{"/data/media", "110G", "34G", "76G", "32%", "/storage/emulated"},
	}

	for i, entry := range entries {
		fmt.Printf("%s:%s size=%s avail=%s, used=%s\n", entry.FileSystem, entry.MountedOn,
			dfNumberToString(entry.Size), dfNumberToString(entry.Avail), dfNumberToString(entry.Used))
		assert.Equal(t, entry.FileSystem, dfOutputData[i].Filesystem)
		assert.Equal(t, dfV2NumberToString(entry.Size), dfOutputData[i].Size)
		assert.Equal(t, dfV2NumberToString(entry.Used), dfOutputData[i].Used)
		assert.Equal(t, dfV2NumberToString(entry.Avail), dfOutputData[i].Avail)
		assert.Equal(t, entry.MountedOn, dfOutputData[i].MountedOn)
	}
}

func Test_unpackDfV2_Android14(t *testing.T) {
	data := `PD2254:/ $ df -h
Filesystem        Size Used Avail Use% Mounted on
/dev/block/dm-16  5.9G 5.9G     0 100% /
tmpfs             7.4G 2.2M  7.4G   1% /dev
tmpfs             7.4G    0  7.4G   0% /mnt
/dev/block/dm-17  226M 226M     0 100% /system_ext
/dev/block/dm-18  314M 314M     0 100% /product
/dev/block/dm-19  1.9G 1.9G     0 100% /vendor
/dev/block/dm-24   12K  12K     0 100% /vendor/vgc
/dev/block/dm-20   67M  67M     0 100% /vendor_dlkm
/dev/block/dm-22  692K 692K     0 100% /odm
/dev/block/dm-23  3.0M 3.0M     0 100% /oem
tmpfs             7.4G  16K  7.4G   1% /apex
/dev/block/sda27  224M  41M  176M  19% /logdata
/dev/block/sde21  114M 112M     0 100% /vendor/vm-system
/dev/block/dm-59  457G  25G  432G   6% /data
/dev/block/loop5   42M  42M     0 100% /apex/com.android.vndk.v34@1
/dev/block/loop7  764K 736K   16K  98% /apex/com.android.tzdata@340090000
/dev/block/loop15  38M  38M     0 100% /apex/com.android.i18n@1
/dev/block/loop10  11M  11M     0 100% /apex/com.android.runtime@1
/dev/block/loop17 108M 108M     0 100% /apex/com.android.vndk.v30@1
/dev/block/loop6  7.4M 7.4M     0 100% /apex/com.android.healthfitness@340090000
/dev/block/loop18  40M  39M     0 100% /apex/com.android.vndk.v32@1
/dev/block/loop13 732K 704K   16K  98% /apex/com.android.sdkext@340090000
/dev/block/loop16 0.9M 984K  8.0K 100% /apex/com.android.rkpd@1
/dev/block/loop9   39M  39M     0 100% /apex/com.android.vndk.v31@1
/dev/block/loop8  3.2M 3.2M     0 100% /apex/com.android.os.statsd@340090000
/dev/block/loop31  45M  45M     0 100% /apex/com.android.vndk.v33@1
/dev/block/loop38 4.8M 4.8M     0 100% /apex/com.android.devicelock@1
/dev/block/loop21 232K  96K  132K  43% /apex/com.android.apex.cts.shim@1
/dev/block/loop23 312K 280K   28K  91% /apex/com.android.virt@2
/dev/block/dm-40   25M  25M     0 100% /apex/com.android.media.swcodec@340090000
/dev/block/dm-58  3.5M 3.5M     0 100% /apex/com.android.appsearch@340090000
/dev/block/dm-55   49M  49M     0 100% /apex/com.android.art@340090000
/dev/block/dm-28   22M  22M     0 100% /apex/com.android.tethering@340090000
/dev/block/dm-26   25M  25M     0 100% /apex/com.android.wifi@340090000
/dev/block/dm-51  232K 196K   32K  86% /apex/com.android.configinfrastructure@340090000
/dev/block/dm-48  232K 104K  124K  46% /apex/com.android.scheduling@340090000
/dev/block/dm-38  7.5M 7.5M     0 100% /apex/com.android.neuralnetworks@340090000
/dev/block/dm-53  3.6M 3.6M     0 100% /apex/com.android.uwb@340090000
/dev/block/dm-54   24M  24M     0 100% /apex/com.android.extservices@340090000
/dev/block/dm-39  8.2M 8.1M     0 100% /apex/com.android.adbd@340090000
/dev/block/dm-36  4.0M 4.0M     0 100% /apex/com.android.resolv@340090000
/dev/block/dm-41  8.7M 8.6M     0 100% /apex/com.android.mediaprovider@340090000
/dev/block/dm-52  9.4M 9.4M     0 100% /apex/com.android.permission@340090000
/dev/block/dm-50  5.8M 5.8M     0 100% /apex/com.android.conscrypt@340090000
/dev/block/dm-42  768K 740K   16K  98% /apex/com.android.ipsec@340090000
/dev/block/dm-46   18M  18M     0 100% /apex/com.android.adservices@340090000
/dev/block/dm-37  2.1M 2.1M     0 100% /apex/com.android.ondevicepersonalization@340090000
/dev/block/dm-33  6.2M 6.2M     0 100% /apex/com.android.media@340090000
/dev/block/dm-60  8.0K 8.0K     0 100% /system/dyn
/dev/fuse         457G  25G  432G   6% /storage/emulated
/dev/fuse         457G  25G  432G   6% /storage/emulated/999`
	entries := unpackDfV2([]byte(data))
	assert.Equal(t, len(entries), 51)
	for _, entry := range entries {
		s := fmt.Sprintf("%-17s %4s %4s %5s", entry.FileSystem,
			dfV2NumberToString(entry.Size), dfV2NumberToString(entry.Used), dfV2NumberToString(entry.Avail) /*entry.MountedOn*/)
		fmt.Println(s)
		// assert.Contains(t, data, s)
	}
}

func TestDevice_DF(t *testing.T) {
	d := adbclient.Device(AnyDevice())
	entries, err := d.DF()
	assert.Nil(t, err)
	for _, entry := range entries {
		fmt.Printf("%s:%s size=%s used=%s avail=%s\n", entry.FileSystem, entry.MountedOn,
			dfV2NumberToString(entry.Size), dfV2NumberToString(entry.Used), dfV2NumberToString(entry.Avail))
	}
}
