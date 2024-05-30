package adb

import (
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path"
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
	if err := conn.SendOctetString(ID_LSTAT_V1); err != nil {
		return nil, err
	}
	if err := conn.SendBytes([]byte(path)); err != nil {
		return nil, err
	}

	id, err := conn.ReadStatus("stat")
	if err != nil {
		return nil, err
	}
	if id != ID_LSTAT_V1 {
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

func (conn *FileService) List(path string) (entries *DirEntries, err error) {
	if err = conn.SendOctetString(ID_LIST_V1); err != nil {
		return
	}
	if err = conn.SendBytes([]byte(path)); err != nil {
		return
	}

	return &DirEntries{syncConn: conn.SyncConn}, nil
}

func (conn *FileService) Recv(path string) (io.ReadCloser, error) {
	if err := conn.SendOctetString(ID_RECV); err != nil {
		return nil, err
	}
	if err := conn.SendBytes([]byte(path)); err != nil {
		return nil, err
	}
	return newSyncFileReader(conn.SyncConn), nil
}

// Send returns a WriteCloser than will write to the file at path on device.
// The file will be created with permissions specified by mode.
// The file's modified time will be set to mtime, unless mtime is 0, in which case the time the writer is
// closed will be used.
func (conn *FileService) Send(path string, mode os.FileMode, mtime time.Time) (io.WriteCloser, error) {
	if err := conn.SendOctetString(ID_SEND); err != nil {
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
		return fmt.Errorf("stat remote file %s: %w", remotePath, err)
	}
	size := info.Size

	// FIXME: need support Dir or file
	writer, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("open local file %s: %w", localPath, err)
	}
	defer writer.Close()

	// open remote reader
	reader, err := s.Recv(remotePath)
	if err != nil {
		return fmt.Errorf("recv remote file %s: %w", remotePath, err)
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
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 {
			return nil
		}
		if n > 0 {
			if handler != nil {
				sent += int64(n)
				handler(int64(size), sent, time.Since(startTime), "pulling")
			}
			_, err = writer.Write(chunk[0:n])
			if err != nil {
				return err
			}
		}
	}
}

func (s *FileService) PushFile(localPath, remotePath string, handler func(n uint64)) (err error) {
	info, err := os.Lstat(localPath)
	if err != nil {
		return fmt.Errorf("stat remote file %s: %w", localPath, err)
	}
	size := int(info.Size())
	perms := info.Mode().Perm()
	mtime := info.ModTime()

	// open src reader
	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open local file %s: %w", localPath, err)
	}
	defer localFile.Close()

	// open remote writer
	writer, err := s.Send(remotePath, perms, mtime)
	if err != nil {
		return fmt.Errorf("write remote file %s: %w", remotePath, err)
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
	for {
		n, err := localFile.Read(chunk)
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 {
			return nil
		}
		_, err = writer.Write(chunk[0:n])
		if err != nil {
			return err
		}
		if handler != nil {
			handler(uint64(n))
		}
	}
}

type SyncHandler func(totalFiles, sentFiles uint64, current string, percent, speed float64, err error)

func (s *FileService) PushDir(onlySubFiles bool, localDir, remotePath string, handler SyncHandler) (err error) {
	info, err := os.Lstat(localDir)
	if err != nil {
		return err
	}
	_ = info

	// Count the total amount of regular files in localDir
	var totalFiles uint64
	err = filepath.WalkDir(localDir, func(path string, d fs.DirEntry, err error) error {
		if path == localDir {
			return nil
		}
		if err != nil {
			return err
		}
		// ignore special file
		if d.Type().IsRegular() || d.IsDir() {
			totalFiles++
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk dir %s failed: %w", localDir, err)
	}

	remotePath = trimSuffixSlash(remotePath)
	localDir = trimSuffixSlash(localDir)

	if !onlySubFiles {
		remotePath = remotePath + "/" + path.Base(localDir)
	}

	var sentFiles uint64
	err = filepath.WalkDir(localDir,
		func(path string, d fs.DirEntry, err error) error {
			if path == localDir {
				return nil
			}
			if err != nil {
				return err
			}
			// ignore special files
			if !d.Type().IsRegular() {
				return nil
			}

			sentFiles++
			relativePath, _ := filepath.Rel(localDir, path)
			target := remotePath + "/" + relativePath
			totalSize := float64(info.Size())
			sentSize := float64(0)
			startTime := time.Now()
			percent := float64(0)
			err = s.PushFile(path, target, func(n uint64) {
				percent = float64(sentSize) / float64(totalSize) * 100
				sentSize = sentSize + float64(n)
				speedMBPerSecond := sentSize * float64(time.Second) / 1024 / 1024 / float64(time.Since(startTime))
				// fmt.Printf("push %.02f%% %d Bytes, %.02f MB/s\n", percent, uint64(sentSize), speedKBPerSecond)
				if speedMBPerSecond == math.Inf(+1) {
					handler(totalFiles, sentFiles, target, percent, 100, nil) // as 100MB/s
				} else {
					handler(totalFiles, sentFiles, target, percent, speedMBPerSecond, nil)
				}
			})
			if err != nil {
				handler(totalFiles, sentFiles, target, percent, 0, err)
			}
			return nil
		})
	return
}

func trimSuffixSlash(p string) string {
	if len(p) > 1 && p[:len(p)-1] == "/" {
		p = p[:len(p)-1]
	}
	return p
}
