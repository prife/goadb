package adb

import (
	"errors"
	"io"
	"testing"

	"github.com/prife/goadb/wire"
	"github.com/stretchr/testify/assert"
)

func TestReadNextChunk(t *testing.T) {
	s := wire.NewSyncConn(makeMockConnStr(
		"DATA\006\000\000\000hello DATA\005\000\000\000worldDONE"))

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
	s := wire.NewSyncConn(makeMockConnStr(
		"ATAD\006\000\000\000hello "))

	// Read 1st chunk
	_, err := s.ReadNextChunkSize()
	assert.EqualError(t, err, "AssertionError: expected chunk id 'DATA' or 'DONE', but got 'ATAD'")
}

func TestReadMultipleCalls(t *testing.T) {
	s := wire.NewSyncConn(makeMockConnStr(
		"DATA\006\000\000\000hello DATA\005\000\000\000worldDONE"))
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
	s := wire.NewSyncConn(makeMockConnStr(
		"DATA\006\000\000\000hello DATA\005\000\000\000worldDONE"))
	reader := newSyncFileReader(s)
	buf := make([]byte, 20)
	_, err := io.ReadFull(reader, buf)
	assert.Equal(t, io.ErrUnexpectedEOF, err)
	assert.Equal(t, "hello world\000", string(buf[:12]))
}

func TestReadError(t *testing.T) {
	s := wire.NewSyncConn(makeMockConnStr(
		"FAIL\004\000\000\000fail"))
	r := newSyncFileReader(s)
	_, err := r.Read(nil)
	assert.EqualError(t, err, "AdbError: request read-chunk, server error: fail")
}

func TestReadEmpty(t *testing.T) {
	s := wire.NewSyncConn(makeMockConnStr(
		"DONE"))
	r := newSyncFileReader(s)
	data, err := io.ReadAll(r)
	assert.NoError(t, err)
	assert.Empty(t, data)
	// Multiple read calls that return EOF is a valid case.
	for i := 0; i < 5; i++ {
		data, err := io.ReadAll(r)
		assert.ErrorIs(t, err, io.EOF)
		assert.Empty(t, data)
	}
}

func TestReadErrorNotFound(t *testing.T) {
	s := wire.NewSyncConn(makeMockConnStr(
		"FAIL\031\000\000\000No such file or directory"))
	r := newSyncFileReader(s)
	_, err := r.Read(nil)
	assert.True(t, errors.Is(err, wire.ErrFileNoExist))
	assert.EqualError(t, err, "FileNoExist: no such file or directory")
}
