package adb

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/prife/goadb/wire"
)

// DirEntry holds information about a directory entry on a device.
type DirEntry struct {
	Name       string
	Mode       os.FileMode
	Size       int32
	ModifiedAt time.Time
}

// DirEntries iterates over directory entries.
type DirEntries struct {
	syncConn     *wire.SyncConn
	currentEntry *DirEntry
	err          error
}

func (entry DirEntry) String() string {
	return fmt.Sprintf("%s %12d %v %s", entry.Mode.String(), entry.Size, entry.ModifiedAt, entry.Name)
}

// ReadAllDirEntries reads all the remaining directory entries into a slice,
// closes self, and returns any error.
// If err is non-nil, result will contain any entries read until the error occurred.
func (entries *DirEntries) ReadAll() (result []*DirEntry, err error) {
	defer entries.Close()

	for entries.Next() {
		result = append(result, entries.Entry())
	}
	err = entries.Err()

	return
}

func (entries *DirEntries) Next() bool {
	if entries.err != nil {
		return false
	}

	entry, done, err := readNextDirListEntry(entries.syncConn)
	if err != nil {
		entries.err = err
		entries.Close()
		return false
	}

	entries.currentEntry = entry
	if done {
		entries.Close()
		return false
	}

	return true
}

func (entries *DirEntries) Entry() *DirEntry {
	return entries.currentEntry
}

func (entries *DirEntries) Err() error {
	return entries.err
}

// Close closes the connection to the adb.
// Next() will call Close() before returning false.
func (entries *DirEntries) Close() error {
	return entries.syncConn.Close()
}

//	struct __attribute__((packed)) {
//		uint32_t id;
//		uint32_t mode;
//		uint32_t size;
//		uint32_t mtime;
//		uint32_t namelen;
//	} dent_v1; // followed by `namelen` bytes of the name.
func readNextDirListEntry(s *wire.SyncConn) (entry *DirEntry, done bool, err error) {
	var rbuf [16]byte
	_, err = io.ReadFull(s.Conn, rbuf[:])
	if err != nil {
		err = fmt.Errorf("read dentv1 failed: %w", err)
	}

	id := string(rbuf[:4])
	mode_ := binary.LittleEndian.Uint32(rbuf[4:8])
	mode := wire.ParseFileModeFromAdb(mode_)
	size := int32(binary.LittleEndian.Uint32(rbuf[8:12]))
	mtime_ := int32(binary.LittleEndian.Uint32(rbuf[12:16]))
	mtime := time.Unix(int64(mtime_), 0).UTC()

	if id == ID_DONE {
		done = true
		return
	} else if id != ID_DENT_V1 {
		err = fmt.Errorf("error reading dir entries: expected dir entry ID 'DENT', but got '%s'", id)
		return
	}

	name, err := s.ReadString()
	if err != nil {
		err = fmt.Errorf("error reading dir entries: error reading file name: %v", err)
		return
	}

	done = false
	entry = &DirEntry{
		Name:       name,
		Mode:       mode,
		Size:       size,
		ModifiedAt: mtime,
	}
	return
}
