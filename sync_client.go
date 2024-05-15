package adb

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/prife/goadb/wire"
)

var zeroTime = time.Unix(0, 0).UTC()

const (
	ID_LSTAT_V1 = "STAT"
	ID_STAT_V2  = "STA2"
	ID_LSTAT_V2 = "LST2"

	ID_LIST_V1 = "LIST"
	ID_LIST_V2 = "LIS2"
	ID_DENT_V1 = "DENT"
	ID_DENT_V2 = "DNT2"

	ID_SEND = "SEND"
	ID_RECV = "RECV"
	ID_DONE = "DONE"
	ID_OKAY = "OKAY"
	ID_FAIL = "FAIL"
	ID_QUIT = "QUIT"
)

type FileService struct {
	*wire.SyncConn
}

func (conn *FileService) stat(path string) (*DirEntry, error) {
	if err := conn.SendOctetString("STAT"); err != nil {
		return nil, err
	}
	if err := conn.SendBytes([]byte(path)); err != nil {
		return nil, err
	}

	id, err := conn.ReadStatus("stat")
	if err != nil {
		return nil, err
	}
	if id != "STAT" {
		return nil, fmt.Errorf("%w: expected stat ID 'STAT', but got '%s'", wire.ErrAssertion, id)
	}

	return readStat(conn)
}

func (conn *FileService) listDirEntries(path string) (entries *DirEntries, err error) {
	if err = conn.SendOctetString("LIST"); err != nil {
		return
	}
	if err = conn.SendBytes([]byte(path)); err != nil {
		return
	}

	return &DirEntries{scanner: conn}, nil
}

func (conn *FileService) receiveFile(path string) (io.ReadCloser, error) {
	if err := conn.SendOctetString("RECV"); err != nil {
		return nil, err
	}
	if err := conn.SendBytes([]byte(path)); err != nil {
		return nil, err
	}
	return newSyncFileReader(conn)
}

// sendFile returns a WriteCloser than will write to the file at path on device.
// The file will be created with permissions specified by mode.
// The file's modified time will be set to mtime, unless mtime is 0, in which case the time the writer is
// closed will be used.
func (conn *FileService) sendFile(path string, mode os.FileMode, mtime time.Time) (io.WriteCloser, error) {
	if err := conn.SendOctetString("SEND"); err != nil {
		return nil, err
	}

	pathAndMode := encodePathAndMode(path, mode)
	if err := conn.SendBytes(pathAndMode); err != nil {
		return nil, err
	}

	return newSyncFileWriter(conn.SyncConn, mtime), nil
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
	writer, err := s.sendFile(remotePath, perms, mtime)
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

func (s *FileService) PushDir(localDir, remotePath string, handler func(total, sent int64, duration time.Duration, status string)) (err error) {
	info, err := os.Lstat(localDir)
	if err != nil {
		return err
	}

	err = filepath.Walk(localDir,
		func(path string, info os.FileInfo, err error) error {
			if path == localDir {
				return nil
			}
			if err != nil {
				return err
			}

			if info.IsDir() {
				panic("not support dir")
			}

			fmt.Println(path, info.Name())

			localFile, err := os.Open(path)
			if err != nil {
				panic(err)
			}
			defer localFile.Close()

			target := remotePath + "/" + info.Name()
			writer, err := s.sendFile(target, info.Mode().Perm(), info.ModTime())
			if err != nil {
				panic(err)
			}
			n, err := io.Copy(writer, localFile)
			if err != nil {
				panic(err)
			}

			fmt.Println("--> write to :", info.Name(), " n:", n)
			writer.Close()
			return nil
		})
	if err != nil {
		return err
	}
	_ = info
	return
}

func readStat(s wire.SyncScanner) (entry *DirEntry, err error) {
	mode, err := s.ReadFileMode()
	if err != nil {
		err = fmt.Errorf("error reading file mode: %w", err)
		return
	}
	size, err := s.ReadInt32()
	if err != nil {
		err = fmt.Errorf("error reading file size: %w", err)
		return
	}
	mtime, err := s.ReadTime()
	if err != nil {
		err = fmt.Errorf("error reading file time: %w", err)
		return
	}

	// adb doesn't indicate when a file doesn't exist, but will return all zeros.
	// Theoretically this could be an actual file, but that's very unlikely.
	if mode == os.FileMode(0) && size == 0 && mtime == zeroTime {
		return nil, fmt.Errorf("%w: file doesn't exist", wire.ErrFileNoExist)
	}

	entry = &DirEntry{
		Mode:       mode,
		Size:       size,
		ModifiedAt: mtime,
	}
	return
}
