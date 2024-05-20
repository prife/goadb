package adb

import (
	"fmt"
	"io"
	"strings"

	"github.com/prife/goadb/wire"
)

// syncFileReader wraps a SyncConn that has requested to receive a file.
type syncFileReader struct {
	// Reader used to read data from the adb connection.
	syncConn *wire.SyncConn
	toRead   int
}

var _ io.ReadCloser = &syncFileReader{}

func newSyncFileReader(s *wire.SyncConn) (r io.ReadCloser, err error) {
	r = &syncFileReader{
		syncConn: s,
	}
	return
}

func (r *syncFileReader) Read(buf []byte) (n int, err error) {
	var length int32
	if r.toRead == 0 {
		length, err = readNextChunk(r.syncConn)
		if err != nil {
			return 0, err
		} else {
			r.toRead = int(length)
		}
	}

	// need read `r.toRead` bytes
	if len(buf) >= int(r.toRead) {
		n, err = io.ReadFull(r.syncConn, buf[:r.toRead])
	} else {
		n, err = io.ReadFull(r.syncConn, buf)
	}

	r.toRead = r.toRead - n
	return
}

func (r *syncFileReader) Close() error {
	return r.syncConn.Close()
}

// readNextChunk read the 4-bytes length of next chunk of data,
// returns io.EOF if the last chunk has been read.
func readNextChunk(r wire.ISyncConn) (int32, error) {
	status, err := r.ReadStatus("read-chunk")
	if err != nil {
		if strings.Contains(err.Error(), "No such file or directory") {
			err = fmt.Errorf("%w: no such file or directory", wire.ErrFileNoExist)
		}
		return 0, err
	}

	switch status {
	case wire.StatusSyncData:
		return r.ReadInt32()
	case wire.StatusSyncDone:
		return 0, io.EOF
	default:
		return 0, fmt.Errorf("%w: expected chunk id '%s' or '%s', but got '%s'",
			wire.ErrAssertion, wire.StatusSyncData, wire.StatusSyncDone, []byte(status))
	}
}
