package adb

import (
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

// UnpackActivity extract <pacakge>/<component> from bytes
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
	args := []string{"activity", "activities", "|", "grep", "ResumedActivity"}
	resp, err := d.RunCommand("dumpsys", args...)
	if err != nil {
		return // tcp error
	}

	// err maybe nil, check response to determin error
	if len(resp) > 0 {
		app = UnpackActivity(resp)
		if len(app) == 0 {
			return nil, fmt.Errorf("can't found current activity")
		}
	}
	return
}

// ForceStopPackage force-stop app
// Android 14: don't need permission
func (d *Device) ForceStopApp(packageName string) (err error) {
	args := []string{"force-stop", packageName}
	resp, err := d.RunCommand("am", args...)
	if err != nil {
		return err // tcp error
	}

	// err maybe nil, check response to determin error
	if len(resp) == 0 {
		return
	}

	err = errors.New(string(resp))
	return
}
