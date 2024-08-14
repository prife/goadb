package adb_test

import (
	"fmt"
	"testing"

	adb "github.com/prife/goadb"
	"github.com/stretchr/testify/assert"
)

func TestDevice_GetCurrentActivity(t *testing.T) {
	assert.NotNil(t, adbclient)
	d := adbclient.Device(adb.AnyDevice())
	list, err := d.GetCurrentActivity()
	assert.Nil(t, err)
	for _, l := range list {
		fmt.Println(l)
	}
}

func TestDevice_ForceStopApp(t *testing.T) {
	assert.NotNil(t, adbclient)
	d := adbclient.Device(adb.AnyDevice())
	list, err := d.GetCurrentActivity()
	assert.Nil(t, err)
	err = d.ForceStopApp(list[0].Package)
	assert.Nil(t, err)
}

func TestDevice_StartApp(t *testing.T) {
	assert.NotNil(t, adbclient)
	d := adbclient.Device(adb.AnyDevice())
	_, err := d.LaunchAppByMonkey("com.android.settings")
	assert.Nil(t, err)
}

func TestDevice_StartApp2(t *testing.T) {
	assert.NotNil(t, adbclient)
	d := adbclient.Device(adb.AnyDevice())
	_, err := d.LaunchAppByMonkey("com.EpicLRT.ActionRPGSample")
	assert.Nil(t, err)
}

func TestUnpackActivity(t *testing.T) {
	str := `
topResumedActivity=ActivityRecord{18aea91 u0 com.android.settings/.Settings t84}
ResumedActivity: ActivityRecord{18aea91 u0 com.android.settings/.Settings t84}
`
	l := adb.UnpackActivity([]byte(str))
	assert.Equal(t, len(l), 1)
	assert.Equal(t, l[0].Fullname, "com.android.settings/.Settings")
	assert.Equal(t, l[0].Package, "com.android.settings")
	assert.Equal(t, l[0].Component, ".Settings")

	str = `
topResumedActivity=ActivityRecord{18aea91 u0 com.android.settings/com.android.settings.Settings t84}
ResumedActivity: ActivityRecord{18aea91 u0 com.android.settings/com.android.settings.Settings t84}
`
	l = adb.UnpackActivity([]byte(str))
	assert.True(t, len(l) == 1)
	assert.Equal(t, l[0].Fullname, "com.android.settings/com.android.settings.Settings")
	assert.Equal(t, l[0].Package, "com.android.settings")
	assert.Equal(t, l[0].Component, "com.android.settings.Settings")

	str = `
ResumedActivity: ActivityRecord{18aea91 u0 ab/cd  t84}
`
	l = adb.UnpackActivity([]byte(str))
	assert.True(t, len(l) == 1)
	assert.Equal(t, l[0].Fullname, "ab/cd")
	assert.Equal(t, l[0].Package, "ab")
	assert.Equal(t, l[0].Component, "cd")

	str = `
ResumedActivity: ActivityRecord{18aea91 u0 a/b  t84}
ResumedActivity: ActivityRecord{18aea91 u0 c/d  t84}
`
	l = adb.UnpackActivity([]byte(str))
	assert.True(t, len(l) == 2)
	assert.Equal(t, l[0].Fullname, "a/b")
	assert.Equal(t, l[0].Package, "a")
	assert.Equal(t, l[0].Component, "b")

	assert.Equal(t, l[1].Fullname, "c/d")
	assert.Equal(t, l[1].Package, "c")
	assert.Equal(t, l[1].Component, "d")
}
