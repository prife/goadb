// Copyright 2024 The ChromiumOS Authors
// Use of this source code is governed by a MIT License that can be
// found in the LICENSE file.

package adb_test

import (
	"fmt"
	"os"
	"testing"

	adb "github.com/prife/goadb"
	"github.com/stretchr/testify/assert"
)

func TestDevice_Session(t *testing.T) {
	d := adbclient.Device(adb.AnyDevice())
	session, err := d.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	out, err := session.CombinedOutput("date")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(string(out))
}

func TestDevice_SessionNonExistedCommand(t *testing.T) {
	d := adbclient.Device(adb.AnyDevice())
	session, err := d.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	out, err := session.CombinedOutput("non-existed-cmd")
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), "unexpected error code 127")
	fmt.Println(string(out))
}

func TestDevice_SessionLogcat(t *testing.T) {
	d := adbclient.Device(adb.AnyDevice())
	session, err := d.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr
	err = session.Run("logcat")
	if err != nil {
		t.Fatal(err)
	}
}
