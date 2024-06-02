package wire

import (
	"fmt"
	"io"
	"os"
	"time"
)

// DirEntry holds information about a directory entry on a device.
type DirEntry struct {
	Name       string
	Mode       os.FileMode
	Size       int32
	ModifiedAt time.Time
}

func (entry DirEntry) String() string {
	return fmt.Sprintf("%s %12d %v %s", entry.Mode.String(), entry.Size, entry.ModifiedAt, entry.Name)
}

// SyncDirReader iterates over directory entries.
type SyncDirReader struct {
	syncConn *SyncConn
	eof      bool
}

// ReadDir work same as os.ReadDir
// if n = -1, reads all the remaining directory entries into a slice
// If err is non-nil, result will contain any entries read until the error occurred.
// At the end of a directory, the error is io.EOF.
func (dr *SyncDirReader) ReadDir(n int) (entries []*DirEntry, err error) {
	if dr.eof {
		return nil, io.EOF
	}

	for i := 0; i < n; i++ {
		entry, done, err2 := dr.syncConn.readDentV1()
		if err2 != nil {
			err = err2
			return
		}
		if done {
			dr.eof = true
			return
		}

		entries = append(entries, entry)
	}

	return
}

// Close closes the connection to the adb.
// Next() will call Close() before returning false.
func (entries *SyncDirReader) Close() error {
	return entries.syncConn.Close()
}
