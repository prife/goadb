package adb

import (
	"testing"

	"github.com/prife/goadb/wire"
	"github.com/stretchr/testify/assert"
)

func TestGetServerVersion(t *testing.T) {
	s := &MockServer{
		Status:   wire.StatusSuccess,
		Messages: []string{"000a"},
	}
	client := &Adb{s}

	v, err := client.ServerVersion()
	assert.Equal(t, "host:version", s.Requests[0])
	assert.NoError(t, err)
	assert.Equal(t, 10, v)
}

func TestAdb_ListForward(t *testing.T) {
	_, err := adbclient.ListForward()
	if err != nil {
		t.Fatal(err)
	}
}

func TestAdb_parseForwardList(t *testing.T) {
	resp := "PQY0220A15002880 tcp:5000 tcp:5000\nPQY0220A15002880 tcp:6000 tcp:6000\n"
	list := parseForwardList([]byte(resp))
	assert.Len(t, list, 2)
	assert.Equal(t, list[0].Serial, "PQY0220A15002880")
	assert.Equal(t, list[0].Local, "tcp:5000")
	assert.Equal(t, list[0].Remote, "tcp:5000")
	assert.Equal(t, list[1].Serial, "PQY0220A15002880")
	assert.Equal(t, list[1].Local, "tcp:6000")
	assert.Equal(t, list[1].Remote, "tcp:6000")
}

func TestAdb_RemoveAllForward(t *testing.T) {
	// clear all forwards
	err := adbclient.RemoveAllForward()
	if err != nil {
		t.Fatal(err)
	}
}

func TestDevice_DoForward(t *testing.T) {
	d := adbclient.Device(AnyDevice())

	// clear all forwards
	err := adbclient.RemoveAllForward()
	if err != nil {
		t.Fatal(err)
	}

	// forward
	err = d.DoForward("tcp:700001", "tcp:7001", false)
	assert.Contains(t, err.Error(), "server error: cannot bind listener: bad port number '700001'")

	err = d.DoForward("tcp:5000", "tcp:6000", false)
	if err != nil {
		t.Fatal(err)
	}

	err = d.DoForward("tcp:5001", "tcp:6000", false)
	if err != nil {
		t.Fatal(err)
	}

	list, err := d.DoListForward()
	if err != nil {
		t.Fatal(err)
	}
	assert.Len(t, list, 2)
	assert.Equal(t, list[0].Local, "tcp:5000")
	assert.Equal(t, list[0].Remote, "tcp:6000")

	// remove
	d.DoRemoveForward(list[0].Local)
	list, err = d.DoListForward()
	if err != nil {
		t.Fatal(err)
	}
	assert.Len(t, list, 1)
	assert.Equal(t, list[0].Local, "tcp:5001")
	assert.Equal(t, list[0].Remote, "tcp:6000")
}

func TestAdb_Disconnect(t *testing.T) {
	err := adbclient.Disconnect("192.168.1.100:5000")
	assert.Contains(t, err.Error(), "no such device '192.168.1.100:5000'")
}

func TestAdb_DisconnectAll(t *testing.T) {
	err := adbclient.DisconnectAll()
	assert.Nil(t, err)
}
