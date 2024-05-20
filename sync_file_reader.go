package adb

import (
	"io"

	"github.com/prife/goadb/wire"
)

// syncFileReader wraps a SyncConn that has requested to receive a file.
type syncFileReader struct {
	// Reader used to read data from the adb connection.
	syncConn *wire.SyncConn
	toRead   int
}

var _ io.ReadCloser = &syncFileReader{}

func newSyncFileReader(s *wire.SyncConn) (r io.ReadCloser) {
	r = &syncFileReader{
		syncConn: s,
	}
	return
}

func (r *syncFileReader) Read(buf []byte) (n int, err error) {
	var length int32
	if r.toRead == 0 {
		length, err = r.syncConn.ReadNextChunkSize()
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
