package wire

import (
	"io"
	"net"
	"os"
	"time"
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

type ISyncConn interface {
	io.Closer
	StatusReader
	ReadInt32() (int32, error)
	ReadFileMode() (os.FileMode, error)
	ReadTime() (time.Time, error)

	// Reads an octet length, followed by length bytes.
	ReadString() (string, error)

	// Reads an octet length, and returns a reader that will read length
	// bytes (see io.LimitReader). The returned reader should be fully
	// read before reading anything off the Scanner again.
	ReadBytes() (io.Reader, error)

	// SendOctetString sends a 4-byte string.
	SendOctetString(string) error
	SendInt32(int32) error
	SendFileMode(os.FileMode) error
	SendTime(time.Time) error
	// Sends len(data) as an octet, followed by the bytes.
	// If data is bigger than SyncMaxChunkSize, it returns an assertion error.
	SendBytes(data []byte) error
}

type SyncConn struct {
	net.Conn
	rbuf []byte
	wbuf []byte
}

func NewSyncConn(r net.Conn) *SyncConn {
	return &SyncConn{r, make([]byte, 4), make([]byte, 4)}
}
