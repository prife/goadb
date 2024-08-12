package adb

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/prife/goadb/wire"
)

func ListAllSubDirs(localDir string) (list []string, err error) {
	err = filepath.WalkDir(localDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == localDir {
			return nil
		}
		// ignore special file
		if d.IsDir() {
			relativePath, _ := filepath.Rel(localDir, path)
			list = append(list, relativePath)
		}
		return nil
	})
	return
}

// adb shell mkdir
// # Android 14
//
// OP5929L1:/ $ mkdir /a /b /c
// mkdir: '/a': Read-only file system
// mkdir: '/b': Read-only file system
// mkdir: '/c': Read-only file system
// 1|OP5929L1:/ $
//
// OP5929L1:/ $ mkdir /sdcard/a /sdcard/b /sdcard/c
// OP5929L1:/ $ mkdir /sdcard/a /sdcard/b /sdcard/c
// mkdir: '/sdcard/a': File exists
// mkdir: '/sdcard/b': File exists
// mkdir: '/sdcard/c': File exists
// 1|OP5929L1:/ $
//
// OP5929L1:/ $ mkdir /data/a /data/b /data/c
// mkdir: '/data/a': Permission denied
// mkdir: '/data/b': Permission denied
// mkdir: '/data/c': Permission denied
// 1|OP5929L1:/ $
//
// OP5929L1:/ $ mkdir /sd/a /sd/b /sd/c
// mkdir: '/sd/a': No such file or directory
// mkdir: '/sd/b': No such file or directory
// mkdir: '/sd/c': No such file or directory
//
// # Android 5.1
//
// shell@A33:/ $ mkdir /sdcard/a /sdcard/b /sdcard/c
// shell@A33:/ $ mkdir /sdcard/a /sdcard/b /sdcard/c
// mkdir failed for /sdcard/a, File exists
// 255|shell@A33:/ $
//
// shell@A33:/ $ mkdir /sd/a /sd/b /sd/c
// mkdir failed for /sd/a, No such file or directory
// 255|shell@A33:/ $
//
// shell@A33:/ $ mkdir /a /b /c
// mkdir failed for /a, Read-only file system
// 255|shell@A33:/ $
//
// shell@A33:/ $ mkdir /data/a /data/b /data/c
// mkdir failed for /data/a, Permission denied
// 255|shell@A33:/ $
func filterFileExistedError(resp []byte) (errs []error) {
	lines := bytes.Split(resp, []byte("\n"))
	for _, line := range lines {
		line := bytes.TrimSpace(line)
		if len(line) > 0 && !bytes.HasSuffix(line, []byte("File exists")) {
			errs = append(errs, errors.New(string(line)))
		}
	}
	return
}

func (c *Device) Mkdirs(list []string) error {
	return c.MkdirsWithParent(list, false)
}

// adb shell mkdir [-p] <dir1> <dir2> ...
func (c *Device) MkdirsWithParent(list []string, withParent bool) error {
	var commands []string
	var commandsLen int

	var errs []error
	if withParent {
		commands = append(commands, "-p")
	}
	for _, l := range list {
		if commandsLen+len(l) > 32768 {
			resp, err := c.RunCommand("mkdir", commands...)
			if err != nil {
				return err
			}

			if len(resp) > 0 {
				errs = filterFileExistedError(resp)
			}
			commands = make([]string, 0)
			if withParent {
				commands = append(commands, "-p")
			}
			commandsLen = 0
		}

		commands = append(commands, l)
		commandsLen = commandsLen + len(l) + 1 // and one space
	}

	if commandsLen > 0 {
		resp, err := c.RunCommand("mkdir", commands...)
		if err != nil {
			return err
		}
		if len(resp) > 0 {
			errs2 := filterFileExistedError(resp)
			errs = append(errs, errs2...)
		}
	}
	return errors.Join(errs...)
}

// Rm run `adb shell rm -rf xx xx`
// it returns is meaning less in most cases, so just ignore error is ok
func (c *Device) Rm(list []string) error {
	var commands []string
	var commandsLen int

	var errs []error

	commands = append(commands, "-rf")
	for _, l := range list {
		if commandsLen+len(l) > (32768 - 7) { // len('rm -rf ') == 6
			resp, err := c.RunCommandTimeout(-1, "rm", commands...)
			if err != nil {
				return err
			}

			if len(resp) > 0 {
				errs = append(errs, errors.New(string(resp)))
			}

			// reset commands
			commands = make([]string, 0)
			commands = append(commands, "-rf")
			commandsLen = 0
		}

		commands = append(commands, l)
		commandsLen = commandsLen + len(l) + 1 // and one space
	}

	if commandsLen > 0 {
		resp, err := c.RunCommandTimeout(-1, "rm", commands...)
		if err != nil {
			return err
		}
		if len(resp) > 0 {
			errs = append(errs, errors.New(string(resp)))
		}
	}
	return errors.Join(errs...)
}

func (c *Device) PushFile(localPath, remotePath string, handler func(totalSize, sentSize int64, percent, speedMBPerSecond float64)) error {
	linfo, err := os.Lstat(localPath)
	if err != nil {
		return err
	}
	if !linfo.Mode().IsRegular() {
		return fmt.Errorf("not regular file: %s", localPath)
	}

	// features, err := c.DeviceFeatures()
	// if err != nil {
	// 	return fmt.Errorf("get device features: %w", err)
	// }

	fconn, err := c.NewSyncConn()
	if err != nil {
		return err
	}
	defer fconn.Close()

	// if remotePath is dir, just append src file name
	rinfo, err := fconn.Stat(remotePath)
	if err == nil && rinfo.Mode.IsDir() {
		remotePath = remotePath + "/" + linfo.Name()
	}

	total := linfo.Size()
	sent := float64(0)
	startTime := time.Now()

	var syncHandler func(n uint64)
	if handler != nil {
		syncHandler = func(n uint64) {
			sent = sent + float64(n)
			percent := float64(sent) / float64(total) * 100
			speedMBPerSecond := float64(sent) * float64(time.Second) / 1024.0 / 1024.0 / (float64(time.Since(startTime)))
			handler(total, int64(sent), percent, speedMBPerSecond)
		}
	}

	err = fconn.PushFile(localPath, remotePath, syncHandler)
	return err
}

// Push support push file or dir
// push 文件夹:
// adb push src-dir dest-dir具有两种行为，与cp命令效果一致
// 1.如果'dest-dir'路径不存在，会创建'dest-dir'，其内容与`src-dir`完全一致
// 2.如果'dest-dir'路径存在，会创建'dest-dir/src-dir'，其内容与`src-dir`完全一致
//
// 本函数只支持情况2，既永远会在手机上创建src-dir
func (c *Device) PushDir(local, remote string, withSrcDir bool, handler wire.SyncHandler) (err error) {
	linfo, err := os.Lstat(local)
	if err != nil {
		return err
	}

	if !linfo.IsDir() {
		return fmt.Errorf("not dir: %s", local)
	}

	// mkdir sub dirs
	subdirs, err := ListAllSubDirs(local)
	if err != nil {
		return
	}
	remoteSubDirs := make([]string, len(subdirs))
	for i, d := range subdirs {
		remoteSubDirs[i] = remote + "/" + d
	}
	err = c.Mkdirs(remoteSubDirs)
	if err != nil {
		// don't return, just log error
		fmt.Printf("mkdir failed: %s\n", err.Error())
		return err
	}

	// push files
	fconn, err := c.NewSyncConn()
	if err != nil {
		return err
	}
	defer fconn.Close()
	return fconn.PushDir(withSrcDir, local, remote, handler)
}
