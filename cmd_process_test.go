package adb

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	adbclient, _ = NewWithConfig(ServerConfig{})
)

func Test_unpackProccessAndroid7(t *testing.T) {
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
root             6     2       0      0 diag_socket_read    0 S [kworker/u8:0]`
	assert.Equal(t, len(l), 4)
	l = unpackProccess([]byte(andriod14))
	assert.Equal(t, l[3].Uid, "root")
	assert.Equal(t, l[3].Pid, 6)
	assert.Equal(t, l[3].PPid, 2)
	assert.Equal(t, l[3].Name, "[kworker/u8:0]")
}

func Test2(t *testing.T) {
	str := `root      1     0     17096  932   ffffffff 00000000 S /init
root      2     0     0      0     ffffffff 00000000 S kthreadd
root      3     2     0      0     ffffffff 00000000 S ksoftirqd/0
root      5     2     0      0     ffffffff 00000000 S kworker/0:0H`
	re := regexp.MustCompile(`(?m)^(\S+)\s+(\d+)\s+\d+\s+\S+\s+\S+\s+\S+\s+\S+\s+\S+\s+(\S+)$`)
	// re := regexp.MustCompile(`^(\S+)\s+`)
	matches := re.FindAllStringSubmatch(str, -1)
	for _, match := range matches {
		fmt.Printf("Column 1: %s\n", match[1])
		fmt.Printf("Column 2: %s\n", match[2])
		fmt.Printf("Last Column: %s\n", match[3])
	}
}
