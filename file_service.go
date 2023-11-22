package adb

import (
	"fmt"
	"io"
	"os"
	"time"
)

type FileService struct {
	device *Device
	adb    *Adb
	// device adb.DeviceDescriptor
}

func NewFileService(client *Adb, serial string) (f *FileService) {
	return &FileService{
		device: client.Device(DeviceWithSerial(serial)),
		adb:    client,
	}
}

func (s *FileService) PushFile(localPath, remotePath string, handler func(total, sent int64, duration time.Duration, status string)) (err error) {
	info, err := os.Lstat(localPath)
	if err != nil {
		return err
	}
	size := int(info.Size())
	perms := info.Mode().Perm()
	mtime := info.ModTime()

	// open src reader
	localFile, err := os.Open(localPath)
	if err != nil {
		return
	}
	defer localFile.Close()

	// open remote writer
	writer, err := s.device.OpenWrite(remotePath, perms, mtime)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening remote file %s: %s\n", remotePath, err)
		return
	}
	defer writer.Close()

	// copy with progress
	// NOTE: optimize memory cost
	var maxWriteSize int
	if size < 1024*1024 {
		maxWriteSize = 128 * 1024
	} else {
		maxWriteSize = 1024 * 1024
	}

	chunk := make([]byte, maxWriteSize)
	startTime := time.Now()
	var sent int64
	for {
		n, err := localFile.Read(chunk)
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 {
			break
		}
		if handler != nil {
			sent += int64(n)
			handler(info.Size(), sent, time.Since(startTime), "pushing")
		}
		_, err = writer.Write(chunk[0:n])
		if err != nil {
			return err
		}
	}
	return
}
