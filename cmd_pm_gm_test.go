package adb_test

import (
	"context"
	"fmt"
	"testing"

	adb "github.com/prife/goadb"
	"github.com/stretchr/testify/assert"
)

func TestDevice_IsGmPm(t *testing.T) {
	assert.NotNil(t, adbclient)
	d := adbclient.Device(adb.AnyDevice())
	isGm := d.IsGmEmu()
	fmt.Println("IsGmEmu:", isGm)
}

func TestDevice_GMPmListPackages(t *testing.T) {
	assert.NotNil(t, adbclient)
	d := adbclient.Device(adb.AnyDevice())
	list, err := d.GMPmListPackages(true)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range list {
		fmt.Println(name)
	}

	list, err = d.GMPmListPackages(false)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range list {
		fmt.Println(name)
	}
}

func TestDevice_GMPmInstall(t *testing.T) {
	assert.NotNil(t, adbclient)
	d := adbclient.Device(adb.AnyDevice())
	err := d.GMPmInstall(context.TODO(), "/data/local/tmp/WeTestDemo.apk", true, true)
	assert.Nil(t, err)
}
