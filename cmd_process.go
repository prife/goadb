package adb

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Android 14: adb shell,v2:ps == adb shell ps -A
// $ adb shell ps | head -n  5
// USER           PID  PPID        VSZ    RSS WCHAN            ADDR S NAME
// root          8350  1428    6065932 108244 0                   0 S com.oplus.ndsf
// root             1     0    2334848  15576 0                   0 S init
// root             2     0          0      0 0                   0 S [kthreadd]
// root             3     2          0      0 0                   0 I [rcu_gp]
//
// Android 8.0 (From now, support `ps -A`)
// $ adb shell ps --help
// usage: ps [-AadefLlnwZ] [-gG GROUP,] [-k FIELD,] [-o FIELD,] [-p PID,] [-t TTY,] [-uU USER,]
// ...
// $ adb shell,v2:ps == adb shell ps -A
// ct@ubuntu:~$ adb shell ps | head -n 5
// USER           PID  PPID     VSZ    RSS WCHAN            ADDR S NAME
// root             1     0   19652   1700 SyS_epoll_wait      0 S init
// root             2     0       0      0 kthreadd            0 S [kthreadd]
// root             3     2       0      0 smpboot_thread_fn   0 S [ksoftirqd/0]
// root             6     2       0      0 diag_socket_read    0 S [kworker/u8:0]
//
// $ adb shell
// OP5929L1:/ $ ps # only a few processes on current session
// USER           PID  PPID        VSZ    RSS WCHAN            ADDR S NAME
// shell        30463 15653    2130476   6084 __arm64_s+          0 S sh
// shell        30540 30463    2153560   5924 0                   0 R ps
//
//
// Android 7.0
// $ adb shell ps --help
// bad pid '--help'
//
// $ adb shell ps -A
// bad pid '-A'
//
// $ adb shell ps | head -n 5
// USER      PID   PPID  VSIZE  RSS   WCHAN              PC  NAME
// root      1     0     16332  1348  SyS_epoll_ 0000000000 S /init
// root      2     0     0      0       kthreadd 0000000000 S kthreadd
// root      3     2     0      0     smpboot_th 0000000000 S ksoftirqd/0
// root      5     2     0      0     worker_thr 0000000000 S kworker/0:0H

// Android 5.1
// $ adb shell ps --help
// USER     PID   PPID  VSIZE  RSS     WCHAN    PC          NAM
//
// $ adb shell ps -A
// USER     PID   PPID  VSIZE  RSS     WCHAN    PC          NAM
//
// $ adb shell ps
// USER     PID   PPID  VSIZE  RSS     WCHAN    PC          NAME
// root      1     0     17096  932   ffffffff 00000000 S /init
// root      2     0     0      0     ffffffff 00000000 S kthreadd
// root      3     2     0      0     ffffffff 00000000 S ksoftirqd/0
// root      5     2     0      0     ffffffff 00000000 S kworker/0:0H

var (
	ErrNotPermitted  = errors.New("NotPermitted")
	ErrNoSuchProcess = errors.New("NoSuchProcess")
)

type Process struct {
	Uid  string
	Pid  int
	PPid int
	Name string
}

type ProcessFilter func(p Process) bool

var (
	// android 8+                        root    845       2      0     0     0     0     S    [irq/227-q6v5 wdog]
	// android 5.1                       root    845       2      0     0     0     0     S    kworker/2:0H^M
	psRegrex = regexp.MustCompile(`(?m)^(\S+)\s+(\d+)\s+(\d+)\s+\S+\s+\S+\s+\S+\s+\S+\s+\S+\s+(\S+\s*\S+)\s*$`)
)

func unpackProccess(resp []byte, filter ProcessFilter) (names []Process) {
	matches := psRegrex.FindAllSubmatch(resp, -1)
	for _, match := range matches {
		pid, err := strconv.Atoi(string(match[2]))
		if err != nil {
			continue
		}
		ppid, err := strconv.Atoi(string(match[3]))
		if err != nil {
			continue
		}

		p := Process{Uid: string(match[1]), Pid: pid, PPid: ppid, Name: string(match[4])}
		if filter == nil || filter(p) {
			names = append(names, p)
		}
	}
	return
}

// ListProcesses run adb shell ps
func (d *Device) ListProcesses(filter ProcessFilter) (names []Process, err error) {
	// detect wether support ps -A or not
	resp, err := d.RunCommandToEnd(false, "ps", "-A")
	if err != nil {
		return
	}

	// Android 5.1: USER     PID   PPID  VSIZE  RSS     WCHAN    PC          NAM
	// Android 7.1: bad pid '-A'
	// if received too few bytes, means 'ps -A' is not supported
	if len(resp) < 256 {
		// <= Android 7.x
		resp, err = d.RunCommandToEnd(false, "ps")
		if err != nil {
			return
		}
	}

	names = unpackProccess(resp, filter)
	if len(names) == 0 {
		return nil, errors.New(string(resp))
	}
	return
}

func (d *Device) ListProcessGroup(filter ProcessFilter) (list map[Process][]Process, err error) {
	l, err := d.ListProcesses(nil)
	if err != nil {
		return
	}

	list = make(map[Process][]Process)
	for _, p := range l {
		if filter(p) {
			list[p] = make([]Process, 0)
		}
	}

	for _, p := range l {
		for key, value := range list {
			if p.PPid == key.Pid {
				list[key] = append(value, p)
			}
		}
	}
	return
}

// Android 7.x
// PD1619:/ $ pidof --help
// usage: pidof [-s] [-o omitpid[,omitpid...]] [NAME]...
// Print the PIDs of all processes with the given names.
// -s      single shot, only return one pid.
// -o      omit PID(s)
//
// PD1619:/ $ pidof 555555
// 1|PD1619:/ $
//
// Android 5.1
// shell@A33:/ $ pidof 55555
// system/bin/sh: pidof: not found

// FindPids find pid
func (d *Device) PidOf(name string, match bool) (list []Process, err error) {
	return d.ListProcesses(func(p Process) bool {
		return (match && p.Name == name) || (!match && strings.Contains(p.Name, name))
	})
}

func (d *Device) PidGroupOf(name string, match bool) (list map[Process][]Process, err error) {
	return d.ListProcessGroup(func(p Process) bool {
		return (match && p.Name == name) || (!match && strings.Contains(p.Name, name))
	})
}

// Android 14
// $ kill 584
// /system/bin/sh: kill: 584: Operation not permitted
//
// OP5929L1:/ $ kill 12006
// /system/bin/sh: kill: 12006: No such process

// KillPidGroup kill process and it's children processes
func (d *Device) KillPids(list []int, signal int) (err error) {
	args := make([]string, len(list))
	if signal > 0 {
		args = append(args, "-"+strconv.Itoa(signal))
	}
	for _, pid := range list {
		args = append(args, strconv.Itoa(pid))
	}

	resp, err := d.RunCommandToEnd(false, "kill", args...)
	if len(resp) > 0 {
		err = errors.New(string(resp))
		if bytes.Contains(resp, []byte("Operation not permitted")) {
			err = fmt.Errorf("%w: %w", ErrNotPermitted, err)
		} else if bytes.Contains(resp, []byte("No such process")) {
			err = fmt.Errorf("%w: %w", ErrNoSuchProcess, err)
		}
	}
	return
}

// KillPidGroup kill process and it's children processes
func (d *Device) KillPidGroupOf(name string, match bool) (killed map[Process][]Process, err error) {
	killed, err = d.ListProcessGroup(func(p Process) bool {
		return (match && p.Name == name) || (!match && strings.Contains(p.Name, name))
	})
	if err != nil {
		return
	}

	var pids []int
	for p, pl := range killed {
		pids = append(pids, p.Pid)
		for _, child := range pl {
			pids = append(pids, child.Pid)
		}
	}
	err = d.KillPids(pids, 9)
	return
}
