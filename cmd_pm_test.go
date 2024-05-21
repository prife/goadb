package adb_test

import (
	"fmt"
	"testing"

	adb "github.com/prife/goadb"
)

func TestListPackages(t *testing.T) {
	adbclient, err := adb.NewWithConfig(adb.ServerConfig{})
	if err != nil {
		t.Fatal(err)
	}
	d := adbclient.Device(adb.AnyDevice())

	list, err := d.ListPackages(true)
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range list {
		fmt.Println(name)
	}
}
