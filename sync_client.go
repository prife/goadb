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

func (conn *FileService) Stat(path string) (*DirEntry, error) {
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

	mode, err := conn.ReadFileMode()
	if err != nil {
		return nil, fmt.Errorf("error reading file mode: %w", err)
	}
	size, err := conn.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("error reading file size: %w", err)
	}
	mtime, err := conn.ReadTime()
	if err != nil {
		err = fmt.Errorf("error reading file time: %w", err)
		return nil, err
	}

	// adb doesn't indicate when a file doesn't exist, but will return all zeros.
	// Theoretically this could be an actual file, but that's very unlikely.
	if mode == os.FileMode(0) && size == 0 && mtime == zeroTime {
		return nil, fmt.Errorf("%w: file doesn't exist", wire.ErrFileNoExist)
	}

	return &DirEntry{Mode: mode, Size: size, ModifiedAt: mtime}, nil
}

func (conn *FileService) ListDirEntries(path string) (entries *DirEntries, err error) {
	if err = conn.SendOctetString("LIST"); err != nil {
		return
	}
	if err = conn.SendBytes([]byte(path)); err != nil {
		return
	}

	return &DirEntries{syncConn: conn.SyncConn}, nil
}

func (conn *FileService) ReceiveFile(path string) (io.ReadCloser, error) {
	if err := conn.SendOctetString("RECV"); err != nil {
		return nil, err
	}
	if err := conn.SendBytes([]byte(path)); err != nil {
		return nil, err
	}
	return newSyncFileReader(conn.SyncConn), nil
}

// SendFile returns a WriteCloser than will write to the file at path on device.
// The file will be created with permissions specified by mode.
// The file's modified time will be set to mtime, unless mtime is 0, in which case the time the writer is
// closed will be used.
func (conn *FileService) SendFile(path string, mode os.FileMode, mtime time.Time) (io.WriteCloser, error) {
	if err := conn.SendOctetString("SEND"); err != nil {
		return nil, err
	}

	// encodes a path and file mode as required for starting a send file stream.
	// From https://android.googlesource.com/platform/system/core/+/master/adb/SYNC.TXT:
	//	The remote file name is split into two parts separated by the last
	//	comma (","). The first part is the actual path, while the second is a decimal
	//	encoded file mode containing the permissions of the file on device.
	pathAndMode := []byte(fmt.Sprintf("%s,%d", path, uint32(mode.Perm())))

	if err := conn.SendBytes(pathAndMode); err != nil {
		return nil, err
	}

	return newSyncFileWriter(conn.SyncConn, mtime), nil
}

func (s *FileService) PullFile(remotePath, localPath string, handler func(total, sent int64, duration time.Duration, status string)) (err error) {
	info, err := s.Stat(remotePath)
	if err != nil {
		return err
	}
	size := info.Size

	// FIXME: need support Dir or file
	writer, err := os.Create(localPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening local file %s: %s\n", localPath, err)
		return err
	}
	defer writer.Close()

	// open remote reader
	reader, err := s.ReceiveFile(remotePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening remote file %s: %s\n", remotePath, err)
		return
	}
	defer reader.Close()

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
		n, err := reader.Read(chunk)
		// fmt.Println("----", n, err)
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
		if n > 0 {
			if handler != nil {
				sent += int64(n)
				handler(int64(size), sent, time.Since(startTime), "pull")
			}
			_, err = writer.Write(chunk[0:n])
			if err != nil {
				return err
			}
		}

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
	writer, err := s.SendFile(remotePath, perms, mtime)
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
			writer, err := s.SendFile(target, info.Mode().Perm(), info.ModTime())
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
