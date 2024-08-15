package adb

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
)

var (
	activityRegrex = regexp.MustCompile(`\b(\w+(\.\w+)*)\/([\.\w]+)`)
)

type Activity struct {
	Fullname  string
	Package   string
	Component string
}

// UnpackActivity extract <package>/<component> from bytes
func UnpackActivity(resp []byte) (l []Activity) {
	matches := activityRegrex.FindAllSubmatch(resp, -1)
	m := make(map[string]interface{})
	for _, match := range matches {
		fullname := string(match[0])
		if _, ok := m[fullname]; !ok {
			m[fullname] = struct{}{}
			l = append(l, Activity{Fullname: fullname, Package: string(match[1]), Component: string(match[3])})
		}
	}
	return
}

// $ adb shell am
// force-stop [--user <USER_ID> | all | current] <PACKAGE>
//     Completely stop the given application package.
// stop-app [--user <USER_ID> | all | current] <PACKAGE>
//     Stop an app and all of its services.  Unlike `force-stop` this does
//     not cancel the app's scheduled alarms and jobs.
// kill [--user <USER_ID> | all | current] <PACKAGE>
//     Kill all background processes associated with the given application.
//
// $ adb shell 'dumpsys activity activities | grep ResumedActivity'
// Android 14 [passed]
//     topResumedActivity=ActivityRecord{18aea91 u0 com.android.settings/.Settings t84}
//   ResumedActivity: ActivityRecord{18aea91 u0 com.android.settings/.Settings t84}
//
// Android 5.1 [passed]
//   mResumedActivity: ActivityRecord{2f5cd8d4 u0 com.oppo.launcher/.Launcher t9}

// GetCurrentActivity get current focused activity
// TODO: may not support multi display.
// references:
// https://stackoverflow.com/questions/13193592/getting-the-name-of-the-current-activity-via-adb
func (d *Device) GetCurrentActivity() (app []Activity, err error) {
	resp, err := d.RunCommandTimeout(d.CmdTimeoutLong, "dumpsys activity activities | grep ResumedActivity")
	if err != nil {
		return // tcp error
	}

	// err maybe nil, check response to determine error
	if len(resp) > 0 {
		app = UnpackActivity(resp)
		if len(app) == 0 {
			return nil, fmt.Errorf("can't found current activity")
		}
	}
	return
}

// LaunchAppByMonkey launch app by it's package name with monkey
//
// $ adb shell monkey -p com.android.settings 1
// Android 5.1
// Events injected: 1
// ## Network stats: elapsed time=50ms (0ms mobile, 0ms wifi, 50ms not connected)
//
// Android 14
// bash arg: -p
// bash arg: com.android.settings
// bash arg: 1
// args: [-p, com.android.settings, 1]
// arg: "-p"
// arg: "com.android.settings"
// arg: "1"
// data="com.android.settings"
// Events injected: 1
// ## Network stats: elapsed time=56ms (0ms mobile, 0ms wifi, 56ms not connected)
//
// $ adb shell monkey -p com.android.settings1111 1
// ...
// ** No activities found to run, monkey aborted.
func (d *Device) LaunchAppByMonkey(packageName string) (resp []byte, err error) {
	// https://stackoverflow.com/questions/4567904/how-to-start-an-application-using-android-adb-tools
	cmd := "monkey -p " + packageName + " 1"
	resp, err = d.RunCommandTimeout(d.CmdTimeoutLong, cmd)
	if err != nil {
		return // tcp error
	}

	// err maybe nil, check response to determine error
	if bytes.Contains(resp, []byte("Events injected: ")) {
		return
	} else if bytes.Contains(resp, []byte("No activities found to run, monkey aborted")) {
		err = errors.New("no activities found")
		return
	}
	err = errors.New("unrecognized error")
	return
}

// Android 12
// HWNOH:/ $ am start -n com.EpicLRT.ActionRPGSample/com.epicgames.ue4.SplashActivity
// Starting: Intent { cmp=com.EpicLRT.ActionRPGSample/com.epicgames.ue4.SplashActivity }
// HWNOH:/ $ am start -n com.EpicLRT.ActionRPGSample/com.epicgames.ue4.SplashActivity1
// Starting: Intent { cmp=com.EpicLRT.ActionRPGSample/com.epicgames.ue4.SplashActivity1 }
// Error type 3
// Error: Activity class {com.EpicLRT.ActionRPGSample/com.epicgames.ue4.SplashActivity1} does not exist.
func (d *Device) AmStart(pkgActivityName string) error {
	resp, err := d.RunCommandTimeout(d.CmdTimeoutLong, "am start -n "+pkgActivityName)
	if err != nil {
		return err // tcp error
	}
	// err maybe nil, check response to determine error
	if bytes.Contains(resp, []byte("Error: ")) {
		return errors.New(string(resp))
	} else {
		return nil
	}
}

// ForceStopPackage force-stop app
// Android 14: don't need permission
func (d *Device) AmForceStop(packageName string) (err error) {
	resp, err := d.RunCommandTimeout(d.CmdTimeoutLong, "am force-stop "+packageName)
	if err != nil {
		return err // tcp error
	}

	// err maybe nil, check response to determine error
	if len(resp) == 0 {
		return
	}

	err = errors.New(string(resp))
	return
}
