package adb

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
)

// IsGmEmu check if the device is a GameMatrix device
// kalama:/ # getprop ro.product.model
// gamematrix
func (d *Device) IsGmEmu() bool {
	model, err := d.GetProperty("ro.product.model")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(model), "gamematrix")
}

// GameMatrix PmListPackages adb shell pm
// 130|kalama:/ # pm_android list package
// PackageName:com.android.internal.display.cutout.emulation.double;
// PackageName:com.qti.pasrservice;
// PackageName:com.android.cts.priv.ctsshim;
// PackageName:com.android.hotspot2.osulogin;
// PackageName:com.android.smspush;
// PackageName:com.android.captiveportallogin;
// PackageName:com.android.server.telecom.overlay.commo
// kalama:/ # pm_android list package -3
// PackageName:com.tencent.wetest.softkeyboard; Activity:com.android.inputmethod.pinyin.SettingsActivity; Label:Wetest拼音输入法; Size:2309632; VersionName:2.1.2-中文; Uid:10195;
// PackageName:com.EpicLRT.ActionRPGSample; Activity:com.epicgames.ue4.SplashActivity; Label:ActionRPG; Size:560297984; VersionName:1.0; Uid:10117;
// PackageName:com.tencent.mho; Activity:com.epicgames.ue4.SplashActivity; Label:Main-Development-978; Size:2547077120; VersionName:20.0.0.0.0; Uid:10120;
// PackageName:com.tencent.weautomator; Activity:com.tencent.weautomator.ui.LauncherUI; Label:com.tencent.weautomator.ui.AtStubApplication; Size:3618304; VersionName:1.0; Uid:10194;
// PackageName:com.antutu.ABenchMark; Activity:com.android.module.app.ui.start.ABenchMarkStart; Label:安兔兔评测; Size:141087232; VersionName:10.0.4-OB4; Uid:10107;
// PackageName:com.tencent.cginput; Activity:NULL; Label:CloudGameVMKeyboard; Size:143872; VersionName:1.0.0.6261655; Uid:10106;
func (d *Device) GMPmListPackages(thirdParty bool) (names []string, err error) {
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
		pkgInfo := bytes.Split(line, []byte(";"))
		if len(pkgInfo) == 0 {
			continue
		}
		rawPkgName := pkgInfo[0]
		pos := bytes.Index(rawPkgName, []byte("PackageName:"))
		if pos >= 0 {
			l := bytes.TrimSpace(rawPkgName[pos+12:]) // cut `PackageName:`
			names = append(names, string(l))
		}
	}
	return
}

// GameMatrix Android 13
//
// HWNOH:/data/local/tmp $ pm install multi-touch.apk
// com.tencent.weautomator/com.tencent.weautomator.ui.LauncherUI
// Success

func (d *Device) GMPmInstall(ctx context.Context, apkPath string, reinstall bool, grantPermission bool) error {
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
	if bytes.Contains(resp, []byte("Success")) {
		return nil
	}
	return errors.New(string(resp))
}
