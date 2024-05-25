package adb

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

var dfOutputV1 = `Filesystem               Size     Used     Free   Blksize
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

func Test_unpackDfV1(t *testing.T) {
	entries := unpackDfV1([]byte(dfOutputV1))

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

var dfOutputV2 = `Filesystem       Size  Used Avail Use% Mounted on
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
func Test_unpackDfV2(t *testing.T) {
	entries := unpackDfV2([]byte(dfOutputV2))
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
