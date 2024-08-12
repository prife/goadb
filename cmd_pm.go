package adb

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrSecurityException = errors.New("JavaSecurityException")
)

// ListPackages adb shell pm
// list packages [-f] [-d] [-e] [-s] [-3] [-i] [-l] [-u] [-U]
//
//		[--show-versioncode] [--apex-only] [--factory-only]
//		[--uid UID] [--user USER_ID] [FILTER]
//	Prints all packages; optionally only those whose name contains
//	the text in FILTER.  Options are:
//		-f: see their associated file
//		-a: all known packages (but excluding APEXes)
//		-d: filter to only show disabled packages
//		-e: filter to only show enabled packages
//		-s: filter to only show system packages
//		-3: filter to only show third party packages
//		-i: see the installer for the packages
//		-l: ignored (used for compatibility with older releases)
//		-U: also show the package UID
//		-u: also include uninstalled packages
//		--show-versioncode: also show the version code
//		--apex-only: only show APEX packages
//		--factory-only: only show system packages excluding updates
//		--uid UID: filter to only show packages with the given UID
//		--user USER_ID: only list packages belonging to the given user
//		--match-libraries: include packages that declare static shared and SDK libraries
func (d *Device) ListPackages(thirdParty bool) (names []string, err error) {
	args := []string{"list", "packages"}
	if thirdParty {
		args = append(args, "-3")
	}

	list, err := d.RunCommandTimeout(d.CmdTimeoutLong, "pm", args...)
	if err != nil {
		return nil, fmt.Errorf("pm "+strings.Join(args, " ")+": %w", err)
	}

	lines := bytes.Split(list, []byte("\n"))
	for _, line := range lines {
		pos := bytes.Index(line, []byte("package:"))
		if pos >= 0 {
			l := bytes.TrimSpace(line[pos+8:]) // cut `package:`
			names = append(names, string(l))
		}
	}
	return
}

// ClearPackageData clear app
// Android 5.1
// shell:pm clear <package>
// 00000000  53 75 63 63 65 73 73 0d  0a                       |Success..|
func (d *Device) ClearPackageData(packageName string) (err error) {
	resp, err := d.RunCommandTimeout(d.CmdTimeoutLong, "pm", "clear", packageName)
	if err != nil {
		return err // always tcp error
	}

	resp = bytes.TrimSpace(resp)
	// err maybe nil, check response to determine error
	if bytes.Equal(resp, []byte("Success")) {
		return nil
	}

	err = errors.New(string(resp))
	if bytes.Contains(resp, []byte("does not have permission android.permission.CLEAR_APP_USER_DATA to clear data of package")) {
		// https://blog.csdn.net/shandong_chu/article/details/105144785
		// 关闭开发者选项中“权限监控”可消除此错误
		return fmt.Errorf("%w: %w", ErrSecurityException, err)
	}
	return
}

// UninstallPackage uninstall package
// HWALP:/ $ pm uninstall com.tencent.wetest.demo
// Success
// HWALP:/ $ pm uninstall non-existed-app
// Failure [DELETE_FAILED_INTERNAL_ERROR]
func (d *Device) UninstallPackage(packageName string) (err error) {
	resp, err := d.RunCommandTimeout(d.CmdTimeoutLong, "pm", "uninstall", packageName)
	if err != nil {
		return err // always tcp error
	}

	resp = bytes.TrimSpace(resp)
	// err maybe nil, check response to determine error
	if bytes.Equal(resp, []byte("Success")) {
		return nil
	} else if bytes.Contains(resp, []byte("Failure")) {
		return errors.New(string(resp))
	}
	return errors.New("unknown error: " + string(resp))
}

// Android 12 / Harmony OS 4
//
// HWNOH:/data/local/tmp $ pm install multi-touch.apk
// Success
//
// HWNOH:/sdcard $ pm install multi-touch.apk
// avc:  denied  { read } for  scontext=u:r:system_server:s0 tcontext=u:object_r:fuse:s0 tclass=file permissive=0
// System server has no access to read file context u:object_r:fuse:s0 (from path /storage/emulated/0/multi-touch.apk, context u:r:system_server:s0)
// Error: Unable to open file: multi-touch.apk
// Consider using a file under /data/local/tmp/
// Error: Can't open file: multi-touch.apk
// Exception occurred while executing 'install':
// java.lang.IllegalArgumentException: Error: Can't open file: multi-touch.apk
// 	at com.android.server.pm.PackageManagerShellCommand.setParamsSize(PackageManagerShellCommand.java:604)
// 	at com.android.server.pm.PackageManagerShellCommand.doRunInstall(PackageManagerShellCommand.java:1427)
// 	at com.android.server.pm.PackageManagerShellCommand.runInstall(PackageManagerShellCommand.java:1393)
// 	at com.android.server.pm.PackageManagerShellCommand.onCommand(PackageManagerShellCommand.java:209)
// 	at com.android.modules.utils.BasicShellCommandHandler.exec(BasicShellCommandHandler.java:97)
// 	at android.os.ShellCommand.exec(ShellCommand.java:38)
// 	at com.android.server.pm.PackageManagerService.onShellCommand(PackageManagerService.java:29176)
// 	at android.os.Binder.shellCommand(Binder.java:962)
// 	at android.os.Binder.onTransact(Binder.java:846)
// 	at android.content.pm.IPackageManager$Stub.onTransact(IPackageManager.java:5090)
// 	at com.android.server.pm.PackageManagerService.onTransact(PackageManagerService.java:10014)
// 	at com.android.server.pm.HwPackageManagerService.onTransact(HwPackageManagerService.java:527)
// 	at android.os.Binder.execTransactInternal(Binder.java:1197)
// 	at android.os.Binder.execTransact(Binder.java:1156)
// 255|HWNOH:/sdcard $

func (d *Device) PmInstall(ctx context.Context, apkPath string, reinstall bool, grantPermission bool) error {
	var args string
	if reinstall {
		args += "-r "
	}
	if grantPermission {
		args += "-g "
	}

	resp, err := d.RunCommandOutputCtx(ctx, "pm install "+args+apkPath)
	if err != nil {
		return fmt.Errorf("'pm install %s' failed: %w", args+apkPath, err)
	}

	resp = bytes.TrimSpace(resp)
	// err maybe nil, check response to determine error
	if bytes.Equal(resp, []byte("Success")) {
		return nil
	}
	return errors.New(string(resp))
}
