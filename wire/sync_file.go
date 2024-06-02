package wire

import (
	"fmt"
	"io"
	"time"
)

// syncFileReader wraps a SyncConn that has requested to receive a file.
type syncFileReader struct {
	// Reader used to read data from the adb connection.
	syncConn *SyncConn
	toRead   int
	eof      bool
}

var _ io.ReadCloser = &syncFileReader{}

func newSyncFileReader(s *SyncConn) (r io.ReadCloser) {
	r = &syncFileReader{
		syncConn: s,
	}
	return
}

func (r *syncFileReader) Read(buf []byte) (n int, err error) {
	if r.eof {
		return 0, io.EOF
	}

	var length int32
	if r.toRead == 0 {
		length, err = r.syncConn.ReadNextChunkSize()
		if err == io.EOF {
			r.eof = true
			return 0, err
		} else if err != nil {
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

// SyncFileWriter wraps a SyncConn that has requested to send a file.
type SyncFileWriter struct {
	// The modification time to write in the footer.
	// If 0, use the current time.
	mtime time.Time

	// Reader used to read data from the adb connection.
	syncConn *SyncConn
}

func newSyncFileWriter(s *SyncConn, mtime time.Time) *SyncFileWriter {
	return &SyncFileWriter{
		mtime:    mtime,
		syncConn: s,
	}
}

// Write writes the min of (len(buf), 64k).
func (w *SyncFileWriter) Write(buf []byte) (n int, err error) {
	written := 0

	// If buf > 64k we'll have to send multiple chunks.
	// TODO Refactor this into something that can coalesce smaller writes into a single chukn.
	for len(buf) > 0 {
		// Writes < 64k have a one-to-one mapping to chunks.
		// If buffer is larger than the max, we'll return the max size and leave it up to the
		// caller to handle correctly.
		partialBuf := buf
		if len(partialBuf) > SyncMaxChunkSize {
			partialBuf = partialBuf[:SyncMaxChunkSize]
		}

		if err := w.syncConn.SendRequest([]byte(ID_DATA), partialBuf); err != nil {
			return written, err
		}
		written += len(partialBuf)
		buf = buf[len(partialBuf):]
	}

	return written, nil
}

func (w *SyncFileWriter) CopyDone() error {
	if w.mtime.IsZero() {
		w.mtime = time.Now()
	}

	if err := w.syncConn.SendDone(w.mtime); err != nil {
		return fmt.Errorf("error sending done chunk to close stream: %w", err)
	}

	if status, err := w.syncConn.ReadStatus(""); err != nil {
		return fmt.Errorf("error reading status, should receive 'ID_OKAY': %w", err)
	} else if status == ID_OKAY {
		return nil
	} else {
		fmt.Println("sync-send with resp status: ", status)
	}

	return nil
}
