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
	syncConn wire.ISyncConn

	// Reader for the current chunk only.
	chunkReader io.Reader

	// False until the DONE chunk is encountered.
	eof bool
}

var _ io.ReadCloser = &syncFileReader{}

func newSyncFileReader(s wire.ISyncConn) (r io.ReadCloser, err error) {
	r = &syncFileReader{
		syncConn: s,
	}

	// Read the header for the first chunk to consume any errors.
	if _, err = r.Read([]byte{}); err != nil {
		if err == io.EOF {
			// EOF means the file was empty. This still means the file was opened successfully,
			// and the next time the caller does a read they'll get the EOF and handle it themselves.
			err = nil
		} else {
			r.Close()
			return nil, err
		}
	}
	return
}

func (r *syncFileReader) Read(buf []byte) (n int, err error) {
	if r.eof {
		return 0, io.EOF
	}

	if r.chunkReader == nil {
		chunkReader, err := readNextChunk(r.syncConn)
		if err != nil {
			if err == io.EOF {
				// We just read the last chunk, set our flag before passing it up.
				r.eof = true
			}
			return 0, err
		}
		r.chunkReader = chunkReader
	}

	if len(buf) == 0 {
		// Read can be called with an empty buffer to read the next chunk and check for errors.
		// However, net.Conn.Read seems to return EOF when given an empty buffer, so we need to
		// handle that case ourselves.
		return 0, nil
	}

	n, err = r.chunkReader.Read(buf)
	if err == io.EOF {
		// End of current chunk, don't return an error, the next chunk will be
		// read on the next call to this method.
		r.chunkReader = nil
		return n, nil
	}

	return n, err
}

func (r *syncFileReader) Close() error {
	return r.syncConn.Close()
}

// readNextChunk creates an io.LimitedReader for the next chunk of data,
// and returns io.EOF if the last chunk has been read.
func readNextChunk(r wire.ISyncConn) (io.Reader, error) {
	status, err := r.ReadStatus("read-chunk")
	if err != nil {
		if strings.Contains(err.Error(), "No such file or directory") {
			err = fmt.Errorf("%w: no such file or directory", wire.ErrFileNoExist)
		}
		return nil, err
	}

	switch status {
	case wire.StatusSyncData:
		return r.ReadBytes()
	case wire.StatusSyncDone:
		return nil, io.EOF
	default:
		return nil, fmt.Errorf("%w: expected chunk id '%s' or '%s', but got '%s'",
			wire.ErrAssertion, wire.StatusSyncData, wire.StatusSyncDone, []byte(status))
	}
}
