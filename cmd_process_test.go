package adb

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	adbclient, _ = NewWithConfig(ServerConfig{})
)

func Test_unpackProccess(t *testing.T) {
	str := `USER      PID   PPID  VSIZE  RSS   WCHAN              PC  NAME
root      1     0     17096  932   ffffffff 00000000 S /init
root      2     0     0      0     ffffffff 00000000 S kthreadd
root      3     2     0      0     ffffffff 00000000 S ksoftirqd/0
root      5     2     0      0     ffffffff 00000000 S kworker/0:0H`
	l := unpackProccess([]byte(str), nil)
	assert.Equal(t, len(l), 4)
	assert.Equal(t, l[0].Uid, "root")
	assert.Equal(t, l[0].Pid, 1)
	assert.Equal(t, l[0].PPid, 0)
	assert.Equal(t, l[0].Name, "/init")

	assert.Equal(t, l[3].Uid, "root")
	assert.Equal(t, l[3].Pid, 5)
	assert.Equal(t, l[3].PPid, 2)
	assert.Equal(t, l[3].Name, "kworker/0:0H")

	// Android14
	andriod14 := `USER           PID  PPID     VSZ    RSS WCHAN            ADDR S NAME
root             1     0   19652   1700 SyS_epoll_wait      0 S init
root             2     0       0      0 kthreadd            0 S [kthreadd]
root             3     2       0      0 smpboot_thread_fn   0 S [ksoftirqd/0]
root           845     2          0      0 0                   0 S [irq/227-q6v5 wdog]`
	l = unpackProccess([]byte(andriod14), nil)
	assert.Equal(t, len(l), 4)
	assert.Equal(t, l[3].Uid, "root")
	assert.Equal(t, l[3].Pid, 845)
	assert.Equal(t, l[3].PPid, 2)
	assert.Equal(t, l[3].Name, "[irq/227-q6v5 wdog]")

	// only non root processes
	l = unpackProccess([]byte(andriod14), func(p Process) bool {
		return p.Name != "root"
	})
	assert.Equal(t, len(l), 4)

	// only
	l = unpackProccess([]byte(andriod14), func(p Process) bool {
		return p.Pid == 845
	})
	assert.Equal(t, len(l), 1)
	fmt.Println("filter pid:", l)

	l = unpackProccess([]byte(andriod14), func(p Process) bool {
		return p.PPid == 2
	})
	assert.Equal(t, len(l), 2)
	fmt.Println("filter ppid:", l)
}

func TestDevice_ListProcesses(t *testing.T) {
	assert.NotNil(t, adbclient)
	d := adbclient.Device(AnyDevice())
	list, err := d.ListProcesses(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range list {
		fmt.Printf("%-12s%6d%49s%s\n", name.Uid, name.Pid, " ", name.Name)
	}
}

// Android 5.1 passed
func TestDevice_PidGroupOf(t *testing.T) {
	assert.NotNil(t, adbclient)
	d := adbclient.Device(DeviceWithSerial("4d639ca1"))
	list, err := d.PidGroupOf("zygote", true)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(list), 1)
	for p, pl := range list {
		assert.Greater(t, len(pl), 1)

		fmt.Printf("%-12s%6d   %s\n", p.Uid, p.Pid, p.Name)
		for _, pp := range pl {
			fmt.Printf("   %-12s%6d   %s\n", pp.Uid, pp.Pid, pp.Name)
		}
	}
}

func TestDevice_PidGroupOfAndroid14(t *testing.T) {
	assert.NotNil(t, adbclient)
	d := adbclient.Device(DeviceWithSerial("79f63fb7"))
	list, err := d.PidGroupOf("zygote", false) // zygote64, zygote, webview_zygote
	if err != nil {
		t.Fatal(err)
	}

	assert.Greater(t, len(list), 0)
	for p, pl := range list {
		fmt.Printf("%-12s%6d   %s\n", p.Uid, p.Pid, p.Name)
		for _, pp := range pl {
			fmt.Printf("   %-12s%6d   %s\n", pp.Uid, pp.Pid, pp.Name)
		}
	}
}
