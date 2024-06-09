package adb

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
)

type DfEntry struct {
	FileSystem string  // v1: mount-point, v2: device node
	Size       float64 //bytes
	Used       float64 //bytes
	Avail      float64
	MountedOn  string // v1: mount-point, v2: mount-point
}

// parseDfNumber 0, 0.0K, 956.5M, 2.7G
func parseDfNumber(element string) (float64, error) {
	size := len(element)
	switch element[size-1] {
	case 'K':
		value, err := strconv.ParseFloat(string(element[:size-1]), 64)
		if err != nil {
			return 0, err
		}
		return value * 1024, nil
	case 'M':
		value, err := strconv.ParseFloat(string(element[:size-1]), 64)
		if err != nil {
			return 0, err
		}
		return value * 1024 * 1024, nil
	case 'G':
		value, err := strconv.ParseFloat(string(element[:size-1]), 64)
		if err != nil {
			return 0, err
		}
		return value * 1024 * 1024 * 1024, nil
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return strconv.ParseFloat(string(element), 64)
	default:
		return 0, fmt.Errorf("unknown suffix: %s", element)
	}
}

/*
Android 6.0
-----------

shell@HWMYA-L6737:/ $ df -h
Filesystem               Size     Used     Free   Blksize
-h: No such file or directory
1|shell@HWMYA-L6737:/ $

shell@HWMYA-L6737:/ $ df
Filesystem               Size     Used     Free   Blksize
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
/mnt/runtime/write/emulated: Permission denied
*/
var (
	//                                  Filesystem  Size     Used     Free   Blksize
	//                                  /dev      956.5M   148.0K   956.3M   4096
	//                                  /storage  956.5M     0.0K   956.5M   4096
	dfV1Regrex = regexp.MustCompile(`(?m)^(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s+\d+\s*$`)
)

// unpackDfV1
func unpackDfV1(resp []byte) (names []DfEntry) {
	matches := dfV1Regrex.FindAllSubmatch(resp, -1)
	for _, match := range matches {
		size, err := parseDfNumber(string(match[2]))
		if err != nil {
			continue
		}
		used, err := parseDfNumber(string(match[3]))
		if err != nil {
			continue
		}
		avail, err := parseDfNumber(string(match[4]))
		if err != nil {
			continue
		}

		d := DfEntry{
			FileSystem: string(match[1]),
			Size:       size,
			Used:       used,
			Avail:      avail,
			MountedOn:  string(match[1]),
		}
		names = append(names, d)
	}
	return
}

/*
Android 9
---------
sagit:/ $ df --help
usage: df [-HPkhi] [-t type] [FILESYSTEM ...]

The "disk free" command shows total/used/available disk space for
each filesystem listed on the command line, or all currently mounted
filesystems.

-a	Show all (including /proc and friends)
-P	The SUSv3 "Pedantic" option
-k	Sets units back to 1024 bytes (the default without -P)
-h	Human readable output (K=1024)
-H	Human readable output (k=1000)
-i	Show inodes instead of blocks
-t type	Display only filesystems of this type

Pedantic provides a slightly less useful output format dictated by Posix,
and sets the units to 512 bytes instead of the default 1024 bytes.

sagit:/ $ df
Filesystem       1K-blocks     Used Available Use% Mounted on
rootfs             2828340     6328   2822012   1% /
tmpfs              2912464      804   2911660   1% /dev
tmpfs              2912464        0   2912464   0% /mnt
/dev/block/dm-0    5079888  3378472   1685032  67% /system
none               2912464        0   2912464   0% /sys/fs/cgroup
/dev/block/sda17 115609024 35907960  79553608  32% /data
/dev/block/sde37     12016    10256      1436  88% /system/vendor/dsp
/dev/block/sde42    825240   242940    565916  31% /cust
/dev/block/sda16    124912     1936    120356   2% /cache
/dev/block/sda10      5092      160      4932   4% /dev/logfs
/data/media      115609024 35907960  79553608  32% /storage/emulated

sagit:/ $ df -h
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
/data/media      110G   34G   76G  32% /storage/emulated
*/

var (
	//                                  Filesystem       Size  Used Avail Use% Mounted on
	//                                  /dev/block/sda17 110G   34G   76G  32% /data
	dfV2Regrex = regexp.MustCompile(`(?m)^(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s*$`)
)

// unpackDfV2
func unpackDfV2(resp []byte) (names []DfEntry) {
	matches := dfV2Regrex.FindAllSubmatch(resp, -1)
	for _, match := range matches {
		size, err := parseDfNumber(string(match[2]))
		if err != nil {
			continue
		}
		used, err := parseDfNumber(string(match[3]))
		if err != nil {
			continue
		}
		avail, err := parseDfNumber(string(match[4]))
		if err != nil {
			continue
		}

		d := DfEntry{
			FileSystem: string(match[1]),
			Size:       size,
			Used:       used,
			Avail:      avail,
			MountedOn:  string(match[6]),
		}
		names = append(names, d)
	}
	return
}

// DF adb shell df
// After Android 6+, /storage/emulated and /data are on the same partation, they have same information
// Android 5.x, they may be not same
// please see comments in cmd_df_test.go
// in general, check '/data' in MountedOn, which is supported on Android 5.x ~ Android14
func (d *Device) DF() (list []DfEntry, err error) {
	// detect wether support df -h or not
	resp, err := d.RunCommand("df", "-h")
	if err != nil {
		return
	}

	// if received too few bytes, means 'df -h' is not supported
	if len(resp) < 128 {
		// <= Android 6.x
		resp, err = d.RunCommand("df")
		if err != nil {
			return
		}
		list = unpackDfV1(resp)
	} else {
		list = unpackDfV2(resp)
	}

	if len(list) == 0 {
		return nil, errors.New(string(resp))
	}
	return
}
