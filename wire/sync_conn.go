package wire

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"io/fs"
	"math"
	"net"
	"os"
	"path/filepath"
	"time"
)

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
	ID_DATA = "DATA"
	ID_OKAY = "OKAY"
	ID_FAIL = "FAIL"
	ID_QUIT = "QUIT"
)

var (
	zeroTime = time.Unix(0, 0).UTC()
)

// SyncConn is a connection to the adb server in sync mode.
// Assumes the connection has been put into sync mode (by sending "sync" in transport mode).
// The adb sync protocol is defined at
// https://android.googlesource.com/platform/system/core/+/master/adb/SYNC.TXT.
// Unlike the normal adb protocol (implemented in Conn), the sync protocol is binary.
// Lengths are binary-encoded (little-endian) instead of hex.
// Length headers and other integers are encoded in little-endian, with 32 bits.
// File mode seems to be encoded as POSIX file mode.
// Modification time seems to be the Unix timestamp format, i.e. seconds since Epoch UTC.
type SyncConn struct {
	net.Conn
	rbuf []byte
	wbuf []byte
}

func NewSyncConn(r net.Conn) *SyncConn {
	return &SyncConn{r, make([]byte, 8), make([]byte, 8)}
}

// ReadStatus reads a 4-byte status string and returns it.
func (s *SyncConn) ReadStatus(req string) (string, error) {
	return readSyncStatusFailureAsError(s, s.rbuf, req)
}

func (s *SyncConn) ReadInt32() (int32, error) {
	if _, err := io.ReadFull(s, s.rbuf[:4]); err != nil {
		return 0, err
	}
	return int32(binary.LittleEndian.Uint32(s.rbuf)), nil
}

// Reads an octet length, and then the length of bytes.
func (s *SyncConn) ReadBytes(buf []byte) (out []byte, err error) {
	length, err := s.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("error reading bytes from sync scanner: %w", err)
	}
	if len(buf) < int(length) {
		buf = make([]byte, length)
	}
	n, err := io.ReadFull(s, buf[:length])
	if err == io.ErrUnexpectedEOF {
		return nil, errIncompleteMessage("bytes", n, int(length))
	} else if err != nil {
		return nil, fmt.Errorf("error reading string from sync scanner: %w", err)
	}

	return buf[:n], err
}

//	struct __attribute__((packed)) {
//		uint32_t id;
//		uint32_t mode;
//		uint32_t size;
//		uint32_t mtime;
//	} stat_v1;
func unpackLstatV1(rbuf []byte) (d *DirEntry, err error) {
	id := rbuf[:4]
	if string(id) != ID_LSTAT_V1 {
		err = fmt.Errorf("%w: expected stat ID 'STAT', but got '%s'", ErrAssertion, id)
		return
	}
	mode_ := binary.LittleEndian.Uint32(rbuf[4:8])
	mode := ParseFileModeFromAdb(mode_)
	size := int32(binary.LittleEndian.Uint32(rbuf[8:12]))
	mtime_ := int32(binary.LittleEndian.Uint32(rbuf[12:16]))
	mtime := time.Unix(int64(mtime_), 0).UTC()
	// adb doesn't indicate when a file doesn't exist, but will return all zeros.
	// Theoretically this could be an actual file, but that's very unlikely.
	if mode == os.FileMode(0) && size == 0 && mtime == zeroTime {
		err = fmt.Errorf("%w: file doesn't exist", ErrFileNoExist)
		return
	}

	d = &DirEntry{Mode: mode, Size: size, ModifiedAt: mtime}
	return
}

func (conn *SyncConn) finishLstatV1() (d *DirEntry, err error) {
	var rbuf [16]byte
	_, err = io.ReadFull(conn, rbuf[:])
	if err != nil {
		return nil, err
	}
	return unpackLstatV1(rbuf[:])
}

//	struct __attribute__((packed)) {
//		uint32_t id;
//		uint32_t mode;
//		uint32_t size;
//		uint32_t mtime;
//		uint32_t namelen;
//	} dent_v1; // followed by `namelen` bytes of the name.
func (s *SyncConn) readDentV1() (entry *DirEntry, done bool, err error) {
	var buf [20]byte
	_, err = io.ReadFull(s.Conn, buf[:])
	if err != nil {
		err = fmt.Errorf("read dir entry header failed: %w", err)
		return
	}

	id := string(buf[:4])
	mode_ := binary.LittleEndian.Uint32(buf[4:8])
	mode := ParseFileModeFromAdb(mode_)
	size := int32(binary.LittleEndian.Uint32(buf[8:12]))
	mtime_ := int32(binary.LittleEndian.Uint32(buf[12:16]))
	mtime := time.Unix(int64(mtime_), 0).UTC()
	namelen := binary.LittleEndian.Uint32(buf[16:20])

	var name []byte
	if namelen > 0 {
		name = make([]byte, namelen)
		if _, err = io.ReadFull(s, name); err != nil {
			err = fmt.Errorf("read dir entry name failed: %w", err)
		}
	}

	if id == ID_DONE {
		done = true
		return
	} else if id != ID_DENT_V1 {
		err = fmt.Errorf("error reading dir entries: expected dir entry ID 'DENT', but got '%s'", id)
		return
	}

	done = false
	entry = &DirEntry{
		Name:       string(name),
		Mode:       mode,
		Size:       size,
		ModifiedAt: mtime,
	}
	return
}

func (s *SyncConn) Stat(path string) (*DirEntry, error) {
	if err := s.SendRequest([]byte(ID_LSTAT_V1), []byte(path)); err != nil {
		return nil, err
	}
	return s.finishLstatV1()
}

// SendList
// Android 5.1上，打开一个不存在文件夹，List协议并不会报错，且对其获取DENT时直接返回DONE
// 为了确保函数行为正常，先执行STAT
func (s *SyncConn) SendList(path string) (dr *SyncDirReader, err error) {
	_, err = s.Stat(path)
	if err != nil {
		return
	}
	/*
		// 'd' maybe a soft link. for example: /sdcard -> /storage/emulated/
		if !d.Mode.IsDir() {
			err = fmt.Errorf("not dir: %s", path)
			return
		}
	*/

	if err = s.SendRequest([]byte(ID_LIST_V1), []byte(path)); err != nil {
		return
	}
	return &SyncDirReader{syncConn: s}, nil
}

func (s *SyncConn) Recv(path string) (*SyncFileReader, error) {
	if err := s.SendRequest([]byte(ID_RECV), []byte(path)); err != nil {
		return nil, err
	}
	return newSyncFileReader(s), nil
}

// Send returns a WriteCloser than will write to the file at path on device.
// The file will be created with permissions specified by mode.
// The file's modified time will be set to mtime, unless mtime is 0, in which case the time the writer is
// closed will be used.
func (s *SyncConn) Send(path string, mode os.FileMode, mtime time.Time) (*SyncFileWriter, error) {
	// encodes a path and file mode as required for starting a send file stream.
	// From https://android.googlesource.com/platform/system/core/+/master/adb/SYNC.TXT:
	//	The remote file name is split into two parts separated by the last
	//	comma (","). The first part is the actual path, while the second is a decimal
	//	encoded file mode containing the permissions of the file on device.
	pathAndMode := []byte(fmt.Sprintf("%s,%d", path, uint32(mode.Perm())))
	if err := s.SendRequest([]byte(ID_SEND), pathAndMode); err != nil {
		return nil, err
	}

	return newSyncFileWriter(s, mtime), nil
}

// ReadNextChunkSize read the 4-bytes length of next chunk of data,
// returns io.EOF if the last chunk has been read.
//
//	struct __attribute__((packed)) {
//		uint32_t id;
//		uint32_t size;
//	} data; // followed by `size` bytes of data.
//
//  struct __attribute__((packed)) {
// 	    uint32_t id;
// 	    uint32_t msglen;
//  } status; // followed by `msglen` bytes of error message, if id == ID_FAIL.

func (s *SyncConn) ReadNextChunkSize() (int32, error) {
	_, err := io.ReadFull(s, s.rbuf[:8])
	if err != nil {
		return 0, fmt.Errorf("sync read: %w", err)
	}

	id := string(s.rbuf[:4])
	size := int32(binary.LittleEndian.Uint32(s.rbuf[4:8]))

	switch id {
	case ID_DATA:
		return size, nil
	case ID_DONE:
		return 0, io.EOF
	case ID_FAIL:
		buf := make([]byte, size)
		_, err := io.ReadFull(s, buf[:size])
		if err != nil {
			return 0, fmt.Errorf("sync read: %w", err)
		}
		if bytes.Contains(buf[:size], []byte("No such file or directory")) {
			err = fmt.Errorf("%w: no such file or directory", ErrFileNoExist)
		} else {
			err = adbServerError("read-chunk", string(buf[:size]))
		}
		return 0, err
	default:
		return 0, fmt.Errorf("%w: expected chunk id '%s' or '%s', but got '%s'",
			ErrAssertion, ID_DATA, ID_DONE, []byte(id))
	}
}

// Reads the status, and if failure, reads the message and returns it as an error.
// If the status is success, doesn't read the message.
// req is just used to populate the AdbError, and can be nil.
func readSyncStatusFailureAsError(r io.Reader, buf []byte, req string) (string, error) {
	// read 8 bytes
	if len(buf) < 8 {
		buf = make([]byte, 8)
	}

	n, err := io.ReadFull(r, buf[0:8])
	if err == io.ErrUnexpectedEOF {
		return "", fmt.Errorf("error reading status for %s: %w", req, errIncompleteMessage(req, n, 4))
	} else if err != nil {
		return "", fmt.Errorf("error reading status for %s: %w", req, err)
	}

	status := string(buf[:4])
	// fmt.Println("<---status: ", status)
	if status == StatusSuccess {
		return status, nil
	}

	// reads a 4-byte length from r, then reads length bytes
	length := binary.LittleEndian.Uint32(buf[4:8])
	if length > 0 {
		if length > uint32(len(buf)) {
			buf = make([]byte, length)
		}
		_, err = io.ReadFull(r, buf[:length])
		if err != nil {
			return status, fmt.Errorf("read status body error: %w", err)
		}
	}

	if status == ID_FAIL {
		return status, adbServerError(req, string(buf[:length]))
	}

	return status, fmt.Errorf("unknown reason %s", status)
}

func (s *SyncConn) SendDone(t time.Time) error {
	copy(s.wbuf[:4], []byte(ID_DONE))
	binary.LittleEndian.PutUint32(s.wbuf[4:8], uint32(t.Unix()))
	_, err := s.Write(s.wbuf[:8])
	return err
}

// SendRequest send id, then len(data) as an octet, followed by the bytes.
// if data is bigger than SyncMaxChunkSize, it returns an assertion error.
func (s *SyncConn) SendRequest(id []byte, data []byte) error {
	if len(id) != 4 {
		return fmt.Errorf("%w: octet string must be exactly 4 bytes: '%s'", ErrAssertion, id)
	}

	data_len := len(data)
	if data_len > SyncMaxChunkSize {
		// This limit might not apply to filenames, but it's big enough
		// that I don't think it will be a problem.
		return fmt.Errorf("%w: data must be <= %d in length", ErrAssertion, SyncMaxChunkSize)
	}

	if len(s.wbuf) < (8 + data_len) {
		s.wbuf = make([]byte, 8+data_len)
	}

	copy(s.wbuf[:4], id)
	binary.LittleEndian.PutUint32(s.wbuf[4:8], uint32(data_len))
	copy(s.wbuf[8:8+data_len], data)
	if n, err := s.Write(s.wbuf[:8+data_len]); err != nil {
		return fmt.Errorf("error send bytes: %w, sent %d", err, n)
	}
	return nil
}

func (s *SyncConn) PullFile(remotePath, localPath string, handler func(total, sent int64, duration time.Duration)) (err error) {
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

	// copy with progress
	// NOTE: optimize memory cost
	// tested on Android 5.1~9, use trunk size >= 64KB, lead to a very fast but invalid push
	// but on Android 14, test with 1MB chunk size is fine, a litter faster than 64KB
	maxWriteSize := SyncMaxChunkSize
	/*
		if size < 1024*1024 {
			maxWriteSize = 128 * 1024
		} else {
			maxWriteSize = 1024 * 1024
		}
	*/

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
				handler(int64(size), sent, time.Since(startTime))
			}
			_, err = writer.Write(chunk[0:n])
			if err != nil {
				return err
			}
		}
	}
}

func (s *SyncConn) PushFile(localPath, remotePath string, handler func(n uint64)) (err error) {
	linfo, err := os.Lstat(localPath)
	if err != nil {
		return fmt.Errorf("stat %s: %w", localPath, err)
	}
	// size := int(info.Size())
	perms := linfo.Mode().Perm()
	mtime := linfo.ModTime()

	// open src reader
	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", localPath, err)
	}
	defer localFile.Close()

	// open remote writer
	writer, err := s.Send(remotePath, perms, mtime)
	if err != nil {
		return fmt.Errorf("open write %s: %w", remotePath, err)
	}

	// copy with progress
	// NOTE: optimize memory cost
	maxWriteSize := SyncMaxChunkSize
	/*
		if size < 1024*1024 {
			maxWriteSize = 128 * 1024
		} else {
			maxWriteSize = 1024 * 1024
		}
	*/

	chunk := make([]byte, maxWriteSize)
	for {
		n, err := localFile.Read(chunk)
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 {
			return writer.CopyDone()
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

// PushDir push dir to android
// 如果localDir含有子目录，该Api不能保证一定将子目录push到目的地址，该API不会创建目录
func (s *SyncConn) PushDir(withSrcDir bool, localDir, remotePath string, handler SyncHandler) (err error) {
	info, err := os.Lstat(localDir)
	if err != nil {
		return err
	}
	_ = info

	// Count the total amount of regular files in localDir
	var totalFiles uint64
	err = filepath.WalkDir(localDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == localDir {
			return nil
		}
		// ignore special file
		if d.Type().IsRegular() {
			totalFiles++
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk dir %s failed: %w", localDir, err)
	}

	remotePath = trimSuffixSlash(remotePath)
	localDir = trimSuffixSlash(localDir)

	if withSrcDir {
		remotePath = remotePath + "/" + filepath.Base(localDir)
	}

	var sentFiles uint64
	err = filepath.WalkDir(localDir,
		func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if path == localDir {
				return nil
			}
			// ignore special files
			if !d.Type().IsRegular() {
				return nil
			}

			finfo, err := d.Info()
			if err != nil {
				return nil
			}

			sentFiles++
			relativePath, _ := filepath.Rel(localDir, path)
			target := remotePath + "/" + relativePath
			totalSize := finfo.Size()
			sentSize := float64(0)
			startTime := time.Now()
			percent := 0
			err = s.PushFile(path, target, func(n uint64) {
				sentSize = sentSize + float64(n)
				percent = int(float64(sentSize) / float64(totalSize) * 100)
				// invoke callback
				speedMBPerSecond := sentSize * float64(time.Second) / 1024 / 1024 / float64(time.Since(startTime))
				if speedMBPerSecond == math.Inf(+1) {
					if handler != nil {
						handler(totalFiles, sentFiles, target, float64(percent), 100, nil) // as 100MB/s
					}
				} else {
					if handler != nil {
						handler(totalFiles, sentFiles, target, float64(percent), speedMBPerSecond, nil)
					}
				}
				// fmt.Printf("push %.02f%% %d Bytes, %.02f MB/s\n", percent, uint64(sentSize), speedKBPerSecond)
			})
			if err != nil {
				if handler != nil {
					handler(totalFiles, sentFiles, target, float64(percent), 0, err)
				}
			}
			return nil
		})
	return
}

func trimSuffixSlash(p string) string {
	if len(p) > 1 && p[len(p)-1] == '/' {
		p = p[:len(p)-1]
	}
	return p
}
