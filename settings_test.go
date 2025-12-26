package adb_test

import (
	"fmt"
	"testing"

	adb "github.com/prife/goadb"
	"github.com/stretchr/testify/assert"
)

func TestDevice_GetMarketName(t *testing.T) {
	assert.NotNil(t, adbclient)
	d := adbclient.Device(adb.DeviceWithSerial("10AC9720FD003L4"))
	name, err := d.GetMarketName()
	assert.Nil(t, err)
	fmt.Printf("market name:[%s]\n", name)
}
