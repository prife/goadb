package wire

import (
	"bytes"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var (
	someTime = time.Date(2015, 04, 12, 20, 7, 51, 0, time.UTC)
	// The little-endian encoding of someTime.Unix()
	someTimeEncoded = []byte{151, 208, 42, 85}
)

func TestSyncReadOctetString(t *testing.T) {
	s := NewSyncScanner(strings.NewReader("helo"))
	str, err := s.ReadOctetString()
	assert.NoError(t, err)
	assert.Equal(t, "helo", str)
}

func TestSyncSendOctetString(t *testing.T) {
	var buf bytes.Buffer
	s := NewSyncSender(&buf)
	err := s.SendOctetString("helo")
	assert.NoError(t, err)
	assert.Equal(t, "helo", buf.String())
}

func TestSyncSendOctetStringTooLong(t *testing.T) {
	var buf bytes.Buffer
	s := NewSyncSender(&buf)
	err := s.SendOctetString("hello")
	assert.EqualError(t, err, "octet string must be exactly 4 bytes: 'hello'")
}

func TestSyncReadTime(t *testing.T) {
	s := NewSyncScanner(bytes.NewReader(someTimeEncoded))
	decoded, err := s.ReadTime()
	assert.NoError(t, err)
	assert.Equal(t, someTime, decoded)
}

func TestSyncSendTime(t *testing.T) {
	var buf bytes.Buffer
	s := NewSyncSender(&buf)
	err := s.SendTime(someTime)
	assert.NoError(t, err)
	assert.Equal(t, someTimeEncoded, buf.Bytes())
}

func TestSyncReadString(t *testing.T) {
	s := NewSyncScanner(strings.NewReader("\005\000\000\000hello"))
	str, err := s.ReadString()
	assert.NoError(t, err)
	assert.Equal(t, "hello", str)
}

func TestSyncReadStringTooShort(t *testing.T) {
	s := NewSyncScanner(strings.NewReader("\005\000\000\000h"))
	_, err := s.ReadString()
	assert.EqualError(t, err, "incomplete bytes: read 1 bytes, expecting 5")
}

func TestSyncSendString(t *testing.T) {
	var buf bytes.Buffer
	s := NewSyncSender(&buf)
	err := s.SendString("hello")
	assert.NoError(t, err)
	assert.Equal(t, "\005\000\000\000hello", buf.String())
}

func TestSyncReadBytes(t *testing.T) {
	s := NewSyncScanner(strings.NewReader("\005\000\000\000helloworld"))

	reader, err := s.ReadBytes()
	assert.NoError(t, err)
	assert.NotNil(t, reader)

	str, err := ioutil.ReadAll(reader)
	assert.NoError(t, err)
	assert.Equal(t, "hello", string(str))
}