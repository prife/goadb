package adb

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
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

func filterFileExistedError(resp []byte) (errs []error) {
	lines := bytes.Split(resp, []byte("\n"))
	for _, line := range lines {
		line := bytes.TrimSpace(line)
		if len(line) > 0 && !bytes.HasSuffix(line, []byte(": File exists")) {
			errs = append(errs, errors.New(string(line)))
		}
	}
	return
}

func (c *Device) Mkdirs(list []string) error {
	var commonds []string
	var commandsLen int

	var errs []error
	for _, l := range list {
		if commandsLen+len(l) > 32768 {
			resp, err := c.RunCommand("mkdir", commonds...)
			if err != nil {
				return err
			}

			if len(resp) > 0 {
				errs = filterFileExistedError(resp)
			}
			commonds = make([]string, 0)
			commandsLen = 0
		}

		commonds = append(commonds, l)
		commandsLen = commandsLen + len(l) + 1 // and one space
	}

	if commandsLen > 0 {
		resp, err := c.RunCommand("mkdir", commonds...)
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

func (c *Device) PushFile(local, remote string, handler func(totoal, sent int64, percent, speedMBPerSecond float64)) error {
	linfo, err := os.Lstat(local)
	if err != nil {
		return err
	}
	if !linfo.Mode().IsRegular() {
		return fmt.Errorf("not regular file: %s", local)
	}

	fconn, err := c.getSyncConn()
	if err != nil {
		return err
	}
	defer fconn.Close()

	total := linfo.Size()
	sent := float64(0)
	startTime := time.Now()
	err = fconn.PushFile(local, remote, func(n uint64) {
		sent = sent + float64(n)
		percent := float64(sent) / float64(total)
		speedMBPerSecond := float64(sent) * float64(time.Second) / 1024.0 / 1024.0 / (float64(time.Since(startTime)))
		handler(total, int64(sent), percent, speedMBPerSecond)
	})
	return err
}

// Push support push file or dir
// push 文件夹:
// adb push src-dir dest-dir具有两种行为，与cp命令效果一致
// 1.如果'dest-dir'路径不存在，会创建'dest-dir'，其内容与`src-dir`完全一致
// 2.如果'dest-dir'路径存在，会创建'dest-dir/src-dir'，其内容与`src-dir`完全一致
//
// 本函数只支持情况2，既永远会在手机上创建src-dir
func (c *Device) PushDir(local, remote string, onlySubFiles bool, handler SyncHandler) (err error) {
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
	}

	// push files
	fconn, err := c.getSyncConn()
	if err != nil {
		return err
	}
	defer fconn.Close()
	return fconn.PushDir(onlySubFiles, local, remote, handler)
}
