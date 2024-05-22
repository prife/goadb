package adb

import (
	"bytes"
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

	list, err := d.RunCommand("pm", args...)
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
	args := []string{"clear", packageName}
	resp, err := d.RunCommand("pm", args...)
	if err != nil {
		return err // always tcp error
	}

	// TODO: for shell_v2, should trim prefix and suffix chars
	resp = bytes.TrimSpace(resp)
	// err maybe nil, check response to determin error
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
