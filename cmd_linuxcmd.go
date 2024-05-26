package adb

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// The `/proc/uptime` file contains two values that represent the system's uptime and idle time (in seconds) since it started.
// In the output you provided, `52784.18 409860.90`:
// - The first value `52784.18` indicates that the system has been running for 52784.18 seconds (approximately 14.66 hours).
// - The second value `409860.90` indicates that the system's idle time during this period is 409860.90 seconds (approximately 113.85 hours).
// It's worth noting that the idle time may be greater than the actual uptime because each core of a multi-core processor calculates idle time.
// For example, a dual-core processor being idle for 1 second will count as 2 seconds of idle time.
func parseUptime(resp []byte) (uptime float64, err error) {
	list := bytes.Fields(resp)
	if len(list) != 2 {
		err = fmt.Errorf("invalid uptime:%s", resp)
		return
	}
	return strconv.ParseFloat(string(list[0]), 64)
}

func (d *Device) Uptime() (uptime float64, err error) {
	// detect wether support df -h or not
	resp, err := d.RunCommandToEnd(false, "cat", "/proc/uptime")
	if err != nil {
		return
	}
	return parseUptime(resp)
}

type LinuxVersion struct {
	Version string
	Built   time.Time
	Raw     []byte
}

var (
	kernelRegrex = regexp.MustCompile(`\d+\.\d+\.\d+`)
)

func parseUname(resp []byte) (info LinuxVersion, err error) {
	version := kernelRegrex.Find(resp)
	if version == nil {
		err = fmt.Errorf("version not found")
		return
	}

	sep := []byte("SMP PREEMPT")
	sepIndex := bytes.Index(resp, sep)
	if sepIndex < 0 {
		err = fmt.Errorf("%s not found", sep)
		return
	}

	const layout = "Mon Jan 2 15:04:05 MST 2006"
	time, err := time.Parse(layout, string(bytes.TrimSpace(resp[sepIndex+len(sep):])))
	if err != nil {
		return
	}
	info.Version = string(version)
	info.Built = time
	info.Raw = resp
	return
}

func (d *Device) Uname() (version LinuxVersion, err error) {
	// detect wether support df -h or not
	resp, err := d.RunCommandToEnd(false, "cat", "/proc/version")
	if err != nil {
		return
	}
	return parseUname(resp)
}
