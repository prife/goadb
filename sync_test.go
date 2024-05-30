package adb_test

import (
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	adb "github.com/prife/goadb"
	"github.com/stretchr/testify/assert"
)

func newFs() (svr *adb.FileService, err error) {
	adbclient, err := adb.NewWithConfig(adb.ServerConfig{})
	if err != nil {
		return
	}

	d := adbclient.Device(adb.AnyDevice())
	svr, err = d.NewFileService()
	return
}

func TestFileService_PushFile(t *testing.T) {
	fs, err := newFs()
	if err != nil {
		t.Fatal(err)
	}
	defer fs.Close()

	f := "/Users/wetest/Downloads/RTR4-CN.pdf"
	info, err := os.Stat(f)
	if err != nil {
		t.Fatal(t)
	}

	total := float64(info.Size())
	sent := float64(0)
	startTime := time.Now()
	err = fs.PushFile(f, "/sdcard/RTR4-CN.pdf",
		func(n uint64) {
			sent = sent + float64(n)
			percent := float64(sent) / float64(total) * 100
			speedMBPerSecond := float64(sent) * float64(time.Second) / 1024.0 / 1024.0 / (float64(time.Since(startTime)))
			fmt.Printf("push %.02f%% %f Bytes, %.02f MB/s\n", percent, sent, speedMBPerSecond)
		})
	if err != nil {
		t.Fatal(err)
	}
}

func TestFileService_PullFile(t *testing.T) {
	fs, err := newFs()
	if err != nil {
		t.Fatal(err)
	}
	defer fs.Close()

	err = fs.PullFile("/sdcard/WeChatMac.dmg", "WeChatMac.dmg",
		func(total, sent int64, duration time.Duration, status string) {
			percent := float64(sent) / float64(total) * 100
			speedKBPerSecond := float64(sent) * float64(time.Second) / 1024.0 / 1024.0 / float64(duration)
			fmt.Printf("pull %.02f%% %d Bytes / %d, %.02f MB/s\n", percent, sent, total, speedKBPerSecond)
		})
	if err != nil {
		t.Fatal(err)
	}
}

func TestFileService_PushDir(t *testing.T) {
	fs, err := newFs()
	if err != nil {
		t.Fatal(err)
	}
	defer fs.Close()

	pwd, _ := os.Getwd()

	fmt.Println("workdir: ", pwd)

	err = fs.PushDir(false, "/Users/wetest/workplace/udt/goadb/wire", "/sdcard/test/",
		func(totalFiles, sentFiles uint64, current string, percent, speed float64, err error) {
			if err != nil {
				fmt.Printf("[%d/%d] pushing %s, %%%.2f, err:%s\n", sentFiles, totalFiles, current, percent, err.Error())
			} else {
				fmt.Printf("[%d/%d] pushing %s, %%%.2f, %.02f MB/s\n", sentFiles, totalFiles, current, percent, speed)
			}
		})
	if err != nil {
		t.Fatal(err)
	}
}

func TestDeviceFeatures(t *testing.T) {
	adbclient, err := adb.NewWithConfig(adb.ServerConfig{})
	if err != nil {
		t.Fatal(err)
	}

	features, err := adbclient.HostFeatures()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("host features: ", string(features))
	d := adbclient.Device(adb.AnyDevice())

	// Android 14
	// shell_v2,cmd,stat_v2,ls_v2,fixed_push_mkdir,apex,abb,fixed_push_symlink_timestamp,abb_exec,remount_shell,track_app,sendrecv_v2,sendrecv_v2_brotli,sendrecv_v2_lz4,sendrecv_v2_zstd,sendrecv_v2_dry_run_send,openscreen_mdns,delayed_ack
	fmt.Println(d.DeviceFeatures())
}

func TestForwardPort(t *testing.T) {
	adbclient, err := adb.NewWithConfig(adb.ServerConfig{})
	if err != nil {
		t.Fatal(err)
	}
	d := adbclient.Device(adb.AnyDevice())
	conn, err := d.ForwardPort(50000)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	go func() {
		io.Copy(os.Stdout, conn)
	}()

	for i := 0; i < 10; i++ {
		_, err := conn.Write([]byte("hello, world\n"))
		if err != nil {
			return
		}
		time.Sleep(time.Second * 1)
	}
}

func Test_listAllSubDirs(t *testing.T) {
	gotList, err := adb.ListAllSubDirs("cmd")
	assert.Nil(t, err)

	for _, l := range gotList {
		fmt.Println(l)
	}

	_, err = adb.ListAllSubDirs("non-exsited")
	assert.True(t, os.IsNotExist(err))
}
