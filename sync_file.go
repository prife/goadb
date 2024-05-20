package adb

import (
	"fmt"
	"io"
	"time"

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

// syncFileReader should not close underlying syncConn
func (r *syncFileReader) Close() error {
	return nil
}

// syncFileWriter wraps a SyncConn that has requested to send a file.
type syncFileWriter struct {
	// The modification time to write in the footer.
	// If 0, use the current time.
	mtime time.Time

	// Reader used to read data from the adb connection.
	syncConn *wire.SyncConn
}

var _ io.WriteCloser = &syncFileWriter{}

func newSyncFileWriter(s *wire.SyncConn, mtime time.Time) io.WriteCloser {
	return &syncFileWriter{
		mtime:    mtime,
		syncConn: s,
	}
}

// Write writes the min of (len(buf), 64k).
func (w *syncFileWriter) Write(buf []byte) (n int, err error) {
	written := 0

	// If buf > 64k we'll have to send multiple chunks.
	// TODO Refactor this into something that can coalesce smaller writes into a single chukn.
	for len(buf) > 0 {
		// Writes < 64k have a one-to-one mapping to chunks.
		// If buffer is larger than the max, we'll return the max size and leave it up to the
		// caller to handle correctly.
		partialBuf := buf
		if len(partialBuf) > wire.SyncMaxChunkSize {
			partialBuf = partialBuf[:wire.SyncMaxChunkSize]
		}

		if err := w.syncConn.SendOctetString(wire.StatusSyncData); err != nil {
			return written, err
		}
		if err := w.syncConn.SendBytes(partialBuf); err != nil {
			return written, err
		}

		written += len(partialBuf)
		buf = buf[len(partialBuf):]
	}

	return written, nil
}

func (w *syncFileWriter) Close() error {
	if w.mtime.IsZero() {
		w.mtime = time.Now()
	}

	if err := w.syncConn.SendOctetString(wire.StatusSyncDone); err != nil {
		return fmt.Errorf("error sending done chunk to close stream: %w", err)
	}
	if err := w.syncConn.SendTime(w.mtime); err != nil {
		return fmt.Errorf("error writing file modification time: %w", err)
	}

	if status, err := w.syncConn.ReadStatus(""); err != nil || status != wire.StatusSuccess {
		return fmt.Errorf("error reading status, should receive 'ID_OKAY': %w", err)
	}
	return nil
}
