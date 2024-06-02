package wire

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	someTime = time.Date(2015, 04, 12, 20, 7, 51, 0, time.UTC)
	// The little-endian encoding of someTime.Unix()
	someTimeEncoded = []byte{151, 208, 42, 85}
)

func packLstatV1(mode os.FileMode, size int32, mtime time.Time) []byte {
	var b bytes.Buffer
	b.Write([]byte("STAT"))
	binary.Write(&b, binary.LittleEndian, mode)
	binary.Write(&b, binary.LittleEndian, size)
	binary.Write(&b, binary.LittleEndian, mtime.Unix())
	return b.Bytes()
}

func TestStatValid(t *testing.T) {
	var buf bytes.Buffer
	conn := NewSyncConn(makeMockConnBuf(&buf))

	var mode os.FileMode = 0777

	statV1 := packLstatV1(mode, 4, someTime)
	conn.Write(statV1)
	entry, err := conn.Stat("/thing")
	assert.NoError(t, err)
	require.NotNil(t, entry)
	assert.Equal(t, mode, entry.Mode, "expected os.FileMode %s, got %s", mode, entry.Mode)
	assert.Equal(t, int32(4), entry.Size)
	assert.Equal(t, someTime, entry.ModifiedAt)
	assert.Equal(t, "", entry.Name)
}

func TestStatBadResponse(t *testing.T) {
	var buf bytes.Buffer
	conn := NewSyncConn(makeMockConnBuf(&buf))
	conn.SendRequest([]byte("SPAT"), nil)
	entry, err := conn.Stat("/")
	assert.Nil(t, entry)
	assert.Error(t, err)
}

func TestStatNoExist(t *testing.T) {
	var buf bytes.Buffer
	conn := NewSyncConn(makeMockConnBuf(&buf))
	statV1 := packLstatV1(0, 0, time.Unix(0, 0).UTC())
	conn.Write(statV1)
	entry, err := conn.Stat("/")
	assert.Nil(t, entry)
	assert.True(t, errors.Is(err, ErrFileNoExist))
}

func TestSyncSendOctetString(t *testing.T) {
	var buf bytes.Buffer
	s := NewSyncConn(makeMockConnBuf(&buf))
	err := s.SendRequest([]byte("helo"), nil)
	assert.NoError(t, err)
	assert.Equal(t, "helo\x00\x00\x00\x00", buf.String())
}

func TestSyncSendOctetStringTooLong(t *testing.T) {
	var buf bytes.Buffer
	s := NewSyncConn(makeMockConnBuf(&buf))
	err := s.SendRequest([]byte("hello"), nil)
	assert.EqualError(t, err, "AssertionError: octet string must be exactly 4 bytes: 'hello'")
}

func TestSyncReadString(t *testing.T) {
	s := NewSyncConn(makeMockConnStr("\005\000\000\000hello"))
	str, err := s.ReadBytes(nil)
	assert.NoError(t, err)
	assert.Equal(t, "hello", string(str))
}

func TestSyncReadStringTooShort(t *testing.T) {
	s := NewSyncConn(makeMockConnStr("\005\000\000\000h"))
	_, err := s.ReadBytes(nil)
	assert.Equal(t, errIncompleteMessage("bytes", 1, 5), err)
}

func TestSyncSendBytes(t *testing.T) {
	var buf bytes.Buffer
	s := NewSyncConn(makeMockConnBuf(&buf))
	err := s.SendRequest([]byte(ID_DATA), []byte("hello"))
	assert.NoError(t, err)
	assert.Equal(t, "DATA\005\000\000\000hello", buf.String())
}

func TestSyncReadBytes(t *testing.T) {
	s := NewSyncConn(makeMockConnStr("\005\000\000\000helloworld"))

	buf, err := s.ReadBytes(nil)
	assert.NoError(t, err)
	assert.Equal(t, "hello", string(buf))
}
