package adb_test

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	adb "github.com/prife/goadb"
	"github.com/prife/goadb/wire"
	"github.com/stretchr/testify/assert"
)

var (
	testZip string
)

func init() {
	homePath, _ := os.UserHomeDir()
	testZip = path.Join(homePath, "Downloads/test.zip")
}

func TestDeviceFeatures(t *testing.T) {
	features, err := adbclient.HostFeatures()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("host features: ", features)
	d := adbclient.Device(adb.AnyDevice())
	fmt.Println(d.DeviceFeatures())
}

func TestPath(t *testing.T) {
	winPath := "E:\\Documents\\WXWork\\Cache\\File\\test.apk"
	basePath := path.Base(winPath)
	assert.Equal(t, basePath, winPath)

	if runtime.GOOS == "windows" {
		basePath2 := filepath.Base(winPath)
		assert.Equal(t, basePath2, "test.apk")
	}
}

func TestForwardPort(t *testing.T) {
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

// $ adb shell mkdir /sdcard/a/ /sdcard/a/b /sdcard/a/b/c
// $ adb shell mkdir /sdcard/a/ /sdcard/a/b /sdcard/a/b/c
// mkdir: '/sdcard/a/': File exists
// mkdir: '/sdcard/a/b': File exists
// mkdir: '/sdcard/a/b/c': File exists
func TestDevice_Mkdirs(t *testing.T) {
	d := adbclient.Device(adb.AnyDevice())
	_, err := d.RunCommand("rm", "-rf", "/sdcard/a")
	assert.Nil(t, err)

	err = d.Mkdirs([]string{"/sdcard/a/", "/sdcard/a/b", "/sdcard/a/b/c"})
	assert.Nil(t, err)
	err = d.Mkdirs([]string{"/sdcard/a/", "/sdcard/a/b", "/sdcard/a/b/c"})
	assert.Nil(t, err)
}

// $ adb shell mkdir /sd/a/ /sd/a/b /sd/a/b/c
// mkdir: '/sd/a/': No such file or directory
// mkdir: '/sd/a/b': No such file or directory
// mkdir: '/sd/a/b/c': No such file or directory
func TestDevice_Mkdirs_NonExsit(t *testing.T) {
	d := adbclient.Device(adb.AnyDevice())
	err := d.Mkdirs([]string{"/sd/a/", "/sd/a/b", "/sd/a/b/c"})
	fmt.Println(err)
	assert.NotNil(t, err)
	lines := strings.Split(err.Error(), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			return
		}
		assert.Contains(t, line, "No such file or directory")
	}
}

func TestDevice_Mkdirs_ReadOnly(t *testing.T) {
	d := adbclient.Device(adb.AnyDevice())
	err := d.Mkdirs([]string{"/a", "/b", "/c"})
	fmt.Println(err)
	assert.NotNil(t, err)
	lines := strings.Split(err.Error(), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			return
		}
		assert.Contains(t, line, "Read-only file system")
	}
}

func TestDevice_Mkdirs_PermissionDeny(t *testing.T) {
	d := adbclient.Device(adb.AnyDevice())
	err := d.Mkdirs([]string{"/data/a", "/data/b", "/data/c"})
	fmt.Println(err)
	assert.NotNil(t, err)
	lines := strings.Split(err.Error(), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			return
		}
		assert.Contains(t, line, "Permission denied")
	}
}

func TestDevice_Rm_NonExistAndPermissionDeny(t *testing.T) {
	d := adbclient.Device(adb.AnyDevice())
	err := d.Rm([]string{"/data/a", "/data/b", "/data/c"})
	if err != nil {
		fmt.Println(err)
	}
	assert.Nil(t, err)

	err = d.Rm([]string{"/a", "/b", "/c"})
	if err != nil {
		fmt.Println(err)
	}
	assert.Nil(t, err)
}

func listDir(d *adb.Device, path string) error {
	sc, dr, err := d.OpenDirReader(path)
	if err != nil {
		fmt.Println("list dir: ", err)
		return err
	}
	defer sc.Close()

	list, err := dr.ReadDir(-1)
	if err != nil {
		return err
	}

	for _, l := range list {
		fmt.Println(l)
	}
	return nil
}

func TestDeviceOpenDirReader(t *testing.T) {
	d := adbclient.Device(adb.AnyDevice())
	sc, dr, err := d.OpenDirReader("/sdcard")
	if err != nil {
		t.Fatal(err)
	}

	defer sc.Close()
	list, err := dr.ReadDir(-1)
	assert.Equal(t, err, io.EOF)
	for _, l := range list {
		fmt.Println(l)
	}
}

func TestDeviceOpenDirReader_NonExisted(t *testing.T) {
	d := adbclient.Device(adb.AnyDevice())
	_, dr, err := d.OpenDirReader("/non-exsited")
	assert.ErrorIs(t, err, wire.ErrFileNoExist)
	fmt.Println(dr, err)
}

func TestFileService_PushFile_LargeFile(t *testing.T) {
	d := adbclient.Device(adb.AnyDevice())
	fs, err := d.NewSyncConn()
	if err != nil {
		t.Fatal(err)
	}
	defer fs.Close()

	info, err := os.Stat(testZip)
	if err != nil {
		t.Fatal(err)
	}

	total := float64(info.Size())
	sent := float64(0)
	startTime := time.Now()
	err = fs.PushFile(testZip, "/sdcard/test.zip",
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

func TestFileService_PushFile_ToDir(t *testing.T) {
	d := adbclient.Device(adb.AnyDevice())
	fs, err := d.NewSyncConn()
	if err != nil {
		t.Fatal(err)
	}
	defer fs.Close()

	pwd, _ := os.Getwd()
	localfile := path.Join(pwd, "sync.go")
	_, err = os.Stat(localfile)
	assert.Nil(t, err)

	err = fs.PushFile(localfile, "/sdcard/", func(n uint64) {})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Is a directory")
	fmt.Println(err)
}

func TestDevice_PushFile(t *testing.T) {
	d := adbclient.Device(adb.AnyDevice())
	pwd, _ := os.Getwd()

	// push to dir
	err := d.PushFile(path.Join(pwd, "sync.go"), "/sdcard/",
		func(totalSize, sentSize int64, percent, speedMBPerSecond float64) {
			fmt.Printf("%d/%d bytes, %.02f%%, %.02f MB/s\n", sentSize, totalSize, percent, speedMBPerSecond)
		})
	if err != nil {
		t.Fatal(err)
	}

	// push to file
	err = d.PushFile(testZip, "/sdcard/test.zip",
		func(totalSize, sentSize int64, percent, speedMBPerSecond float64) {
			fmt.Printf("%d/%d bytes, %.02f%%, %.02f MB/s\n", sentSize, totalSize, percent, speedMBPerSecond)
		})
	if err != nil {
		t.Fatal(err)
	}
}

func TestFileService_PushDir(t *testing.T) {
	pwd, _ := os.Getwd()
	fmt.Println("workdir: ", pwd)

	// clear remote dir
	d := adbclient.Device(adb.AnyDevice())
	_ = d.Rm([]string{"/sdcard/wire"})
	// listDir(d, "/sdcard/")

	// create connection
	fs, err := d.NewSyncConn()
	if err != nil {
		t.Fatal(err)
	}
	defer fs.Close()

	// push directory
	err = fs.PushDir(true, path.Join(pwd, "wire/"), "/sdcard/",
		func(totalFiles, sentFiles uint64, current string, percent, speed float64, err error) {
			if err != nil {
				fmt.Printf("[%d/%d] pushing %s, %.2f%%, err:%s\n", sentFiles, totalFiles, current, percent, err.Error())
			} else {
				fmt.Printf("[%d/%d] pushing %s, %.2f%%, %.02f MB/s\n", sentFiles, totalFiles, current, percent, speed)
			}
		})
	if err != nil {
		t.Fatal(err)
	}
}

func TestDevice_PushDir(t *testing.T) {
	pwd, _ := os.Getwd()
	fmt.Println("workdir: ", pwd)

	// clear remote dir
	d := adbclient.Device(adb.AnyDevice())
	err := d.Mkdirs([]string{"/sdcard/test"})
	if err != nil {
		t.Fatal(err)
	}
	// listDir(d, "/sdcard/")

	// push all files and subdirs under wire/ to /sdcard/test
	err = d.PushDir(path.Join(pwd, "wire/"), "/sdcard/test", false,
		func(totalFiles, sentFiles uint64, current string, percent, speed float64, err error) {
			if err != nil {
				fmt.Printf("[%d/%d] pushing %s, %.2f%%, err:%s\n", sentFiles, totalFiles, current, percent, err.Error())
			} else {
				fmt.Printf("[%d/%d] pushing %s, %.2f%%, %.02f MB/s\n", sentFiles, totalFiles, current, percent, speed)
			}
		})
	if err != nil {
		t.Fatal(err)
	}

	// push all files and subdirs under wire/ to /sdcard/test
	err = d.PushDir(path.Join(pwd, "wire/"), "/sdcard/test", true,
		func(totalFiles, sentFiles uint64, current string, percent, speed float64, err error) {
			if err != nil {
				fmt.Printf("[%d/%d] pushing %s, %.2f%%, err:%s\n", sentFiles, totalFiles, current, percent, err.Error())
			} else {
				fmt.Printf("[%d/%d] pushing %s, %.2f%%, %.02f MB/s\n", sentFiles, totalFiles, current, percent, speed)
			}
		})
	if err != nil {
		t.Fatal(err)
	}
}

func TestFileService_PullFile(t *testing.T) {
	d := adbclient.Device(adb.AnyDevice())
	fs, err := d.NewSyncConn()
	if err != nil {
		t.Fatal(err)
	}
	defer fs.Close()

	err = fs.PullFile("/sdcard/test.zip", "test.zip",
		func(total, sent int64, duration time.Duration) {
			percent := float64(sent) / float64(total) * 100
			speedKBPerSecond := float64(sent) * float64(time.Second) / 1024.0 / 1024.0 / float64(duration)
			fmt.Printf("pull %.02f%% %d Bytes / %d, %.02f MB/s\n", percent, sent, total, speedKBPerSecond)
		})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPercentPrint(t *testing.T) {
	totalSize := 512 * 1024 * 1024
	sent := 0
	i := 0

	percent := 0
	for {
		if (sent + 64*1024) < totalSize {
			sent += 64 * 1024
		} else {
			sent = totalSize
		}

		percent2 := int(float64(sent) / float64(totalSize) * 100)
		// 过滤器，减少打印次数
		if percent2 == 100 {
			// pass
		} else if totalSize < 1024*1024 && percent2 < 100 {
			continue
		} else if (percent2 - percent) < 11 {
			continue
		}
		i++
		percent = percent2
		fmt.Printf("[%3d] percent:%d\n", i, percent)

		if sent == 0 {
			break
		}

		if percent == 100 {
			break
		}
	}
}
