package adb_test

import (
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	adb "github.com/prife/goadb"
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

	err = fs.PushFile("/Users/wetest/Downloads/RTR4-CN.pdf", "/sdcard/RTR4-CN.pdf",
		func(total, sent int64, duration time.Duration, status string) {
			percent := float64(sent) / float64(total) * 100
			speedKBPerSecond := float64(sent) / 1024.0 / 1024.0 / (float64(duration) / float64(time.Second))
			fmt.Printf("push %.02f%% %d Bytes, %.02f MB/s\n", percent, sent, speedKBPerSecond)
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
			speedKBPerSecond := float64(sent) / 1024.0 / 1024.0 / (float64(duration) / float64(time.Second))
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

	err = fs.PushDir("doc", "/sdcard/test/",
		func(total, sent int64, duration time.Duration, status string) {})
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
