package adb_test

import (
	"context"
	"fmt"
	"testing"

	adb "github.com/prife/goadb"
	"github.com/stretchr/testify/assert"
)

func TestDevice_RunCommandCtx(t *testing.T) {
	assert.NotNil(t, adbclient)
	d := adbclient.Device(adb.AnyDevice())
	out, err := d.RunCommandOutputCtx(context.TODO(), "/data/local/tmp/test")
	assert.Nil(t, err)
	fmt.Printf("out:[%s]\n", out)
}
