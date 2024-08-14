package adb_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	adb "github.com/prife/goadb"
	"github.com/stretchr/testify/assert"
)

var (
	adbclient, _ = adb.NewWithConfig(adb.ServerConfig{})
)

func TestListPackages(t *testing.T) {
	assert.NotNil(t, adbclient)
	d := adbclient.Device(adb.AnyDevice())
	list, err := d.ListPackages(true)
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range list {
		fmt.Println(name)
	}
}

// TestDevice_ClearPackageData
/*
Exception occurred while executing 'clear':
java.lang.SecurityException: PID 6593 does not have permission android.permission.CLEAR_APP_USER_DATA to clear data of package com.heytap.smarthome
	at com.android.server.am.ActivityManagerService.clearApplicationUserData(ActivityManagerService.java:3986)
	at com.android.server.pm.PackageManagerShellCommand.runClear(PackageManagerShellCommand.java:2473)
	at com.android.server.pm.PackageManagerShellCommand.onCommand(PackageManagerShellCommand.java:277)
	at com.android.modules.utils.BasicShellCommandHandler.exec(BasicShellCommandHandler.java:97)
	at android.os.ShellCommand.exec(ShellCommand.java:38)
	at com.android.server.pm.PackageManagerService$IPackageManagerImpl.onShellCommand(PackageManagerService.java:6823)
	at android.os.Binder.shellCommand(Binder.java:1092)
	at android.os.Binder.onTransact(Binder.java:912)
	at android.content.pm.IPackageManager$Stub.onTransact(IPackageManager.java:4352)
	at com.android.server.pm.PackageManagerService$IPackageManagerImpl.onTransact(PackageManagerService.java:6807)
	at android.os.Binder.execTransactInternal(Binder.java:1392)
	at android.os.Binder.execTransact(Binder.java:1299)
*/
func TestDevice_ClearPackageData(t *testing.T) {
	assert.NotNil(t, adbclient)
	d := adbclient.Device(adb.AnyDevice())
	list, err := d.ListPackages(true)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, len(list) > 0)

	packageName := list[0]
	fmt.Println("pm clear", packageName)

	err = d.ClearPackageData(list[0])
	if err != nil {
		assert.ErrorIs(t, err, adb.ErrSecurityException)
		fmt.Println(err)
	}
}

func TestDevice_UninstallPackage(t *testing.T) {
	assert.NotNil(t, adbclient)
	d := adbclient.Device(adb.AnyDevice())
	err := d.UninstallPackage("non-existed-app")
	assert.True(t, strings.Contains(err.Error(), "DELETE_FAILED_INTERNAL_ERROR"))

	err = d.UninstallPackage("com.tencent.wetestdemo")
	assert.Nil(t, err)
}

func TestDevice_PmInstall(t *testing.T) {
	assert.NotNil(t, adbclient)
	d := adbclient.Device(adb.AnyDevice())
	err := d.PmInstall(context.TODO(), "/data/local/tmp/WeTestDemo.apk", true, true)
	assert.Nil(t, err)
}
