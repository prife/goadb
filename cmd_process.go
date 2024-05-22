package adb

import (
	"bytes"
	"errors"
	"regexp"
	"strconv"
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

type Process struct {
	Uid  string
	Pid  int
	PPid int
	Name string
}

var (
	//                                   root    845       2      0     0     0     0     S    [irq/227-q6v5 wdog]`
	psRegrex = regexp.MustCompile(`(?m)^(\S+)\s+(\d+)\s+(\d+)\s+\S+\s+\S+\s+\S+\s+\S+\s+\S+\s+(.*)$`)
)

func unpackProccess(resp []byte) (names []Process) {
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
		names = append(names, Process{Uid: string(match[1]), Pid: pid, PPid: ppid, Name: string(match[4])})
	}
	return
}

// ListProcesses run adb shell ps
func (d *Device) ListProcesses() (names []Process, err error) {
	// detect wether support ps -A or not
	resp, err := d.RunCommandToEnd(false, "ps", "-A")
	if err != nil {
		return
	}

	lines := bytes.Split(resp, []byte("\n"))
	// running processes of Android must > 10, if the number < 10 means 'ps -A' is not supported
	if len(lines) < 10 {
		// <= Android 7.0
		resp, err = d.RunCommandToEnd(false, "ps")
		if err != nil {
			return
		}
	}

	names = unpackProccess(resp)
	if len(names) == 0 {
		return nil, errors.New(string(resp))
	}
	return
}

func (d *Device) KillPid(pid int) (err error) {
	return
}

// KillPidGroup kill process and it's children processes
func (d *Device) KillPidGroup(pid int) (killList []Process, err error) {
	return
}
