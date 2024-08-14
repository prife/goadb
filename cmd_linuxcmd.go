package adb

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
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
	resp, err := d.RunCommand("cat", "/proc/uptime")
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
	resp, err := d.RunCommand("cat", "/proc/version")
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

var (
	meminfoRegex = regexp.MustCompile(`(?m)\s*(\S+)\:\s*(\d+)\s*kB$`)
)

// in kB
func parseMemoryInfo(resp []byte) (info map[string]uint64, err error) {
	matches := meminfoRegex.FindAllSubmatch(resp, -1)
	if len(matches) == 0 {
		return
	}

	info = make(map[string]uint64)
	for _, match := range matches {
		v, err := strconv.ParseInt(string(match[2]), 0, 64)
		if err != nil {
			continue
		}
		info[string(match[1])] = uint64(v)
	}
	return
}

// GetMemoryTotal
func (d *Device) GetMemoryTotal() (totalInKb uint64, err error) {
	resp, err := d.RunCommand("cat /proc/meminfo")
	if err != nil {
		return
	}

	info, err := parseMemoryInfo(resp)
	if err != nil {
		return
	}
	totalInKb, ok := info["MemTotal"]
	if !ok {
		err = fmt.Errorf("no MemTotal found")
		return
	}
	return
}

type Screen struct {
	Width  int
	Height int
}

type DisplaySizeInfo struct {
	Physical Screen
	Override Screen
}

var (
	rectRegex = regexp.MustCompile(`(\d+)x(\d+)`)
)

// HWALP:/ $ wm size
// Physical size: 1440x2560
// Override size: 720x1280
func parseDisplaySize(resp []byte) (display DisplaySizeInfo, err error) {
	lines := bytes.Split(resp, []byte("\n"))

	var found bool
	for _, line := range lines {
		matches := rectRegex.FindSubmatch(line)
		if len(matches) == 0 {
			continue
		}
		width, _ := strconv.ParseInt(string(matches[1]), 0, 32)
		Height, _ := strconv.ParseInt(string(matches[2]), 0, 32)
		if bytes.Contains(line, []byte("Physical")) {
			display.Physical.Width = int(width)
			display.Physical.Height = int(Height)
			found = true
		} else if bytes.Contains(line, []byte("Override")) {
			display.Override.Width = int(width)
			display.Override.Height = int(Height)
			found = true
		}
	}

	if !found {
		err = fmt.Errorf("parse failed")
	}
	return
}

// GetDisplayDefault wm size
func (d *Device) GetDefaultDisplaySize() (display DisplaySizeInfo, err error) {
	resp, err := d.RunCommand("wm size")
	if err != nil {
		return
	}
	return parseDisplaySize(resp)
}

type CpuInfo struct {
	// Name is the product name of this CPU.
	Name string
	// Vendor is the vendor of this CPU.
	Vendor string
	// Architecture is the architecture that this CPU implements.
	Architecture string
	// Cores is the number of cores in this CPU.
	Cores     uint32
	Frequency float64
}

var (
	cpuInfoRegex = regexp.MustCompile(`(?m)(\w+\s*\w+)\s*:\s*(.*)$`)
)

type CpuInfoProp struct {
	Key   string
	Value string
}

func parseCpuInfo(resp []byte) (cpuInfo []CpuInfoProp, err error) {
	matches := cpuInfoRegex.FindAllSubmatch(resp, -1)
	if len(matches) == 0 {
		err = fmt.Errorf("invalid data")
		return
	}

	for _, match := range matches {
		// fmt.Printf("[%s] : [%s]\n", match[1], match[2])
		cpuInfo = append(cpuInfo, CpuInfoProp{Key: string(match[1]), Value: string(match[2])})
	}
	return
}

// GetCpuInfo get cpu information
func (d *Device) GetCpuInfo() (cpuInfo CpuInfo, err error) {
	resp, err := d.RunCommand("cat /proc/cpuinfo")
	if err != nil {
		return
	}
	infos, err := parseCpuInfo(resp)
	if err != nil {
		return
	}
	for _, kv := range infos {
		switch kv.Key {
		case "Hardware": // optional
			cpuInfo.Name = kv.Value
		case "processor":
			number, _ := strconv.Atoi(kv.Value)
			cpuInfo.Cores = uint32(number) + 1
		case "CPU architecture":
			arch, _ := strconv.Atoi(kv.Value)
			if arch >= 8 {
				cpuInfo.Architecture = "arm64"
			}
		}
	}

	// get cores
	// coreInfo, err := device.RunCommand("ls", "/sys/devices/system/cpu/")

	// get frequency
	freqInfo, err := d.RunCommand("cat /sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_max_freq")
	if err != nil {
		return
	}
	freqTotal, _ := strconv.ParseUint(string(bytes.TrimSpace(freqInfo)), 10, 32)
	freq := float64(freqTotal/100000) / 10.0
	cpuInfo.Frequency = freq
	return
}

// Reboot the device
func (d *Device) Reboot(ctx context.Context, waitToBootCompleted bool) error {
	_, err := d.RunCommand("reboot")
	if errors.Is(err, os.ErrDeadlineExceeded) {
		// pass
		// err is "read tcp 127.0.0.1:65357->127.0.0.1:5037: i/o timeout"
	} else if err == io.EOF {
		// pass
	} else if err != nil {
		return fmt.Errorf("reboot failed: %w", err)
	}

	// make sure adb disconnected
	ctx1, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()
	for {
		if state, _ := d.State(); state == StateInvalid || state == StateOffline {
			break
		}

		select {
		case <-ctx1.Done():
			return fmt.Errorf("reboot check disconnected failed: %w", ctx1.Err())
		case <-time.After(2 * time.Second):
		}
	}

	if !waitToBootCompleted {
		return nil
	}

	// wait to boot complete
	ctx2, cancel := context.WithTimeout(ctx, time.Second*90)
	defer cancel()
	for {
		if state, _ := d.State(); state == StateOnline {
			if booted, _ := d.BootCompleted(); booted {
				return nil
			}
		}

		select {
		case <-ctx2.Done():
			return fmt.Errorf("reboot check booted failed: %w", ctx2.Err())
		case <-time.After(2 * time.Second):
		}
	}
}
