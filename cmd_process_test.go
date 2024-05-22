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
	l := unpackProccess([]byte(str))
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
	l = unpackProccess([]byte(andriod14))
	assert.Equal(t, len(l), 4)
	assert.Equal(t, l[3].Uid, "root")
	assert.Equal(t, l[3].Pid, 845)
	assert.Equal(t, l[3].PPid, 2)
	assert.Equal(t, l[3].Name, "[irq/227-q6v5 wdog]")
}

func TestDevice_ListProcesses(t *testing.T) {
	assert.NotNil(t, adbclient)
	d := adbclient.Device(AnyDevice())
	list, err := d.ListProcesses()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range list {
		fmt.Printf("%-12s%6d%49s%s\n", name.Uid, name.Pid, " ", name.Name)
	}
}
