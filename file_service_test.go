package adb_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	adb "github.com/zach-klippenstein/goadb"
)

func GetDevice(client *adb.Adb, serial string) (d adb.DeviceInfo, err error) {
	infos, err := client.ListDevices()
	if err != nil {
		return
	}

	for _, info := range infos {
		if info.State == adb.StateOnline.String() {
			d = *info
			return
		}
	}

	err = errors.New("no device connected")
	return
}

func TestFileService_PushFile(t *testing.T) {
	client, err := adb.NewWithConfig(adb.ServerConfig{})
	if err != nil {
		t.Fatal(err)
	}

	device, err := GetDevice(client, "")
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("%+v", device)
	svr := adb.NewFileService(client, device.Serial)

	err = svr.PushFile("/Users/wetest/Downloads/Keka-1.2.57.dmg", "/sdcard/Keka-1.2.57.dmg",
		func(total, sent int64, duration time.Duration, status string) {
			percent := float64(sent) / float64(total) * 100
			speedKBPerSecond := float64(sent) / 1024.0 / 1024.0 / (float64(duration) / float64(time.Second))
			fmt.Printf("push %.02f%% %d Bytes, %.02f MB/s\n", percent, sent, speedKBPerSecond)
		})
	if err != nil {
		t.Fatal(err)
	}
}
