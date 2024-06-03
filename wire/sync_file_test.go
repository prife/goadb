package wire

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestReadNextChunk(t *testing.T) {
	s := NewSyncConn(makeMockConnStr(
		"DATA\006\000\000\000hello DATA\005\000\000\000worldDONE\000\000\000\000"))

	// Read 1st chunk
	reader, err := s.ReadNextChunkSize()
	assert.NoError(t, err)
	assert.Equal(t, int32(6), reader)
	buf := make([]byte, 10)
	n, err := s.Read(buf[:reader])
	assert.NoError(t, err)
	assert.Equal(t, 6, n)
	assert.Equal(t, "hello ", string(buf[:6]))

	// Read 2nd chunk
	reader, err = s.ReadNextChunkSize()
	assert.NoError(t, err)
	assert.Equal(t, int32(5), reader)
	buf = make([]byte, 10)
	n, err = s.Read(buf[:reader])
	assert.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, "world", string(buf[:5]))

	// Read DONE
	_, err = s.ReadNextChunkSize()
	assert.Equal(t, io.EOF, err)
}
func TestReadNextChunkInvalidChunkId(t *testing.T) {
	s := NewSyncConn(makeMockConnStr(
		"ATAD\006\000\000\000hello "))

	// Read 1st chunk
	_, err := s.ReadNextChunkSize()
	assert.EqualError(t, err, "AssertionError: expected chunk id 'DATA' or 'DONE', but got 'ATAD'")
}

func TestReadMultipleCalls(t *testing.T) {
	s := NewSyncConn(makeMockConnStr(
		"DATA\006\000\000\000hello DATA\005\000\000\000worldDONE\000\000\000\000"))
	reader := newSyncFileReader(s)

	firstByte := make([]byte, 1)
	_, err := io.ReadFull(reader, firstByte)
	assert.NoError(t, err)
	assert.Equal(t, "h", string(firstByte))

	restFirstChunkBytes := make([]byte, 5)
	_, err = io.ReadFull(reader, restFirstChunkBytes)
	assert.NoError(t, err)
	assert.Equal(t, "ello ", string(restFirstChunkBytes))

	secondChunkBytes := make([]byte, 5)
	_, err = io.ReadFull(reader, secondChunkBytes)
	assert.NoError(t, err)
	assert.Equal(t, "world", string(secondChunkBytes))

	_, err = io.ReadFull(reader, make([]byte, 5))
	assert.Equal(t, io.EOF, err)
}

func TestReadAll(t *testing.T) {
	s := NewSyncConn(makeMockConnStr(
		"DATA\006\000\000\000hello DATA\005\000\000\000worldDONE\000\000\000\000"))
	reader := newSyncFileReader(s)
	buf := make([]byte, 20)
	_, err := io.ReadFull(reader, buf)
	assert.Equal(t, io.ErrUnexpectedEOF, err)
	assert.Equal(t, "hello world\000", string(buf[:12]))
}

func TestReadError(t *testing.T) {
	s := NewSyncConn(makeMockConnStr(
		"FAIL\004\000\000\000fail"))
	r := newSyncFileReader(s)
	_, err := r.Read(nil)
	assert.EqualError(t, err, "AdbError: request read-chunk, server error: fail")
}

func TestReadEmpty(t *testing.T) {
	s := NewSyncConn(makeMockConnStr("DONE\000\000\000\000"))
	r := newSyncFileReader(s)
	data, err := io.ReadAll(r)
	assert.NoError(t, err)
	assert.Empty(t, data)
	// Multiple read calls that return EOF is a valid case.
	for i := 0; i < 5; i++ {
		data, err := io.ReadAll(r)
		assert.NoError(t, err) // io.ReadAll treat an EOF to nil
		assert.Empty(t, data)
	}
}

func TestReadErrorNotFound(t *testing.T) {
	s := NewSyncConn(makeMockConnStr(
		"FAIL\031\000\000\000No such file or directory"))
	r := newSyncFileReader(s)
	_, err := r.Read(nil)
	assert.True(t, errors.Is(err, ErrFileNoExist))
	assert.EqualError(t, err, "FileNoExist: no such file or directory")
}

////////////////////////////////////////////////////////////////////
// writer

func TestFileWriterWriteSingleChunk(t *testing.T) {
	var buf bytes.Buffer
	syncConn := NewSyncConn(makeMockConn2("OKAY\x00\x00\x00\x00", &buf))
	writer := newSyncFileWriter(syncConn, time.Time{})

	n, err := writer.Write([]byte("hello"))
	assert.NoError(t, err)
	assert.Equal(t, 5, n)

	assert.Equal(t, "DATA\005\000\000\000hello", buf.String())
}

func TestFileWriterWriteMultiChunk(t *testing.T) {
	var buf bytes.Buffer
	syncConn := NewSyncConn(makeMockConn2("OKAY\x00\x00\x00\x00", &buf))
	writer := newSyncFileWriter(syncConn, time.Time{})

	n, err := writer.Write([]byte("hello"))
	assert.NoError(t, err)
	assert.Equal(t, 5, n)

	n, err = writer.Write([]byte(" world"))
	assert.NoError(t, err)
	assert.Equal(t, 6, n)

	assert.Equal(t, "DATA\005\000\000\000helloDATA\006\000\000\000 world", buf.String())
}

func TestFileWriterWriteLargeChunk(t *testing.T) {
	var buf bytes.Buffer
	syncConn := NewSyncConn(makeMockConn2("OKAY\x00\x00\x00\x00", &buf))
	writer := newSyncFileWriter(syncConn, time.Time{})

	// Send just enough data to get 2 chunks.
	data := make([]byte, SyncMaxChunkSize+1)
	n, err := writer.Write(data)

	assert.NoError(t, err)
	assert.Equal(t, SyncMaxChunkSize+1, n)
	assert.Equal(t, 8+8+SyncMaxChunkSize+1, buf.Len())

	// First header.
	chunk := buf.Bytes()[:8+SyncMaxChunkSize]
	expectedHeader := []byte("DATA----")
	binary.LittleEndian.PutUint32(expectedHeader[4:], SyncMaxChunkSize)
	assert.Equal(t, expectedHeader, chunk[:8])
	assert.Equal(t, data[:SyncMaxChunkSize], chunk[8:])

	// Second header.
	chunk = buf.Bytes()[SyncMaxChunkSize+8 : SyncMaxChunkSize+8+1]
	expectedHeader = []byte("DATA\000\000\000\000")
	binary.LittleEndian.PutUint32(expectedHeader[4:], 1)
	assert.Equal(t, expectedHeader, chunk[:8])
}

func TestFileWriterCloseEmpty(t *testing.T) {
	var buf bytes.Buffer
	mtime := time.Unix(1, 0)
	syncConn := NewSyncConn(makeMockConn2("OKAY\x00\x00\x00\x00", &buf))
	writer := newSyncFileWriter(syncConn, mtime)

	assert.NoError(t, writer.CopyDone())

	assert.Equal(t, "DONE\x01\x00\x00\x00", buf.String())
}

func TestFileWriterWriteClose(t *testing.T) {
	var buf bytes.Buffer
	mtime := time.Unix(1, 0)
	syncConn := NewSyncConn(makeMockConn2("OKAY\x00\x00\x00\x00", &buf))
	writer := newSyncFileWriter(syncConn, mtime)

	writer.Write([]byte("hello"))

	assert.NoError(t, writer.CopyDone())
	assert.Equal(t, "DATA\005\000\000\000helloDONE\x01\x00\x00\x00", buf.String())
}

func TestFileWriterCloseAutoMtime(t *testing.T) {
	var buf bytes.Buffer
	syncConn := NewSyncConn(makeMockConn2("OKAY\x00\x00\x00\x00", &buf))
	writer := newSyncFileWriter(syncConn, time.Time{})

	assert.NoError(t, writer.CopyDone())
	assert.Len(t, buf.String(), 8)
	assert.True(t, strings.HasPrefix(buf.String(), ID_DONE))

	mtimeBytes := buf.Bytes()[4:]
	mtimeActual := time.Unix(int64(binary.LittleEndian.Uint32(mtimeBytes)), 0)

	// Delta has to be a whole second since adb only supports second granularity for mtimes.
	assert.WithinDuration(t, time.Now(), mtimeActual, 1*time.Second)
}
