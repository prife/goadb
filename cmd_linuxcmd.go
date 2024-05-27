package adb

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
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

type GpuInfo struct {
	Vendor        string
	Model         string
	OpenGLVersion string
}

var (
	// "GLES: Qualcomm, Adreno (TM) 618, OpenGL ES 3.2 V@415.0 (GIT@663be55, I724753c5e3, 1573037262) (Date:11/06/19)"
	// "GLES: ARM, Mali-G78, OpenGL ES 3.2 v1.r34p0-01eac0.a1b116bd871d46ef040e8feef9ed691e"
	gpuRegrex = regexp.MustCompile(`GLES:\s*(\w+),\s*([^,]+),\s*(OpenGL ES [0-9.]+)`)
)

func parseGpu(resp []byte) (info GpuInfo, err error) {
	match := gpuRegrex.FindSubmatch(resp)
	if len(match) == 0 {
		err = fmt.Errorf("can't found GLES: %s", resp)
	}

	info.Vendor = string(match[1])
	info.Model = string(match[2])
	info.OpenGLVersion = string(match[3])
	return
}

func (d *Device) GetGpuAndOpenGL() (des GpuInfo, err error) {
	glstr, err := d.RunCommand("dumpsys SurfaceFlinger | grep GLES")
	if err != nil {
		return
	}

	return parseGpu(glstr)
}

var (
	devicePropertyRegex = regexp.MustCompile(`(?m)\[(\S+)\]:\s*\[(.*)\]\s*$`)
	// devicePropertyRegex = regexp.MustCompile(`\[([\s\S]*?)\]: \[([\s\S]*?)\]\r?`)
)

type PropertiesFilter func(k, v string) bool

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
func (d *Device) GetProperites(filter PropertiesFilter) (properties map[string]string, err error) {
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

var (
	etherRegex = regexp.MustCompile(`\s*link/ether\s+(\S+)`)
	inetRegex  = regexp.MustCompile(`\s*(inet6?)\s+(\S+)`)
)

type EtherInfo struct {
	Name     string
	LinkAddr string // mac
	Ipv4     []byte
	Ipv6     []byte
}

func (e EtherInfo) String() string {
	var a strings.Builder
	a.Write([]byte(e.Name))
	a.WriteString(" link/addr=" + e.LinkAddr)
	if e.Ipv4 != nil {
		a.WriteString(" inet=" + string(e.Ipv4))
	}
	if e.Ipv6 != nil {
		a.WriteString(" inet6=" + string(e.Ipv6))
	}
	return a.String()
}

func parseIpAddressWlan0(resp []byte) (info EtherInfo, err error) {
	match := etherRegex.FindSubmatch(resp)
	if len(match) == 0 {
		err = fmt.Errorf("no linkaddr found")
		return
	}
	info.LinkAddr = string(match[1])

	matches := inetRegex.FindAllSubmatch(resp, -1)
	if len(matches) == 0 {
		return
	}
	for _, match := range matches {
		if string(match[1]) == "inet" {
			info.Ipv4 = bytes.Clone(match[2])
		} else if string(match[1]) == "inet6" {
			info.Ipv6 = bytes.Clone(match[2])
		}
	}
	return
}

// GetWlanInfo adb shell ip address show wlan0
func (d *Device) GetWlanInfo() (info EtherInfo, err error) {
	resp, err := d.RunCommand("ip address show wlan0")
	if err != nil {
		return
	}

	info, err = parseIpAddressWlan0(resp)
	info.Name = "wlan0"
	return
}
