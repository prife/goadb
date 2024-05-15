package services

import (
	"context"

	adb "github.com/prife/goadb"
	"github.com/prife/gomlib"
	log "github.com/sirupsen/logrus"
)

func InitAdb() (cli *adb.Adb, err error) {
	serverConfig := adb.ServerConfig{
		AutoStart: true,
		Host:      "127.0.0.1",
		Port:      5037,
	}

	/*
		out, err := exec.Command("adb", "start-server").CombinedOutput()
		if err != nil {
			panic(out)
		}
	*/

	cli, err = adb.NewWithConfig(serverConfig)
	if err != nil {
		log.Errorln(err)
		return
	}

	err = cli.StartServer()
	if err != nil {
		log.Errorln(err)
		return
	}
	return
}

func Monitor(ctx context.Context, r *gomlib.Registry) (err error) {
	client, err := InitAdb()
	if err != nil {
		return
	}

	watcher := client.NewDeviceWatcher()
	for event := range watcher.C() {
		log.Infof("adb-monitor: %+v", event)
		switch event.NewState {
		case adb.StateOnline:
		case adb.StateInvalid,
			adb.StateUnauthorized,
			adb.StateAuthorizing,
			adb.StateDisconnected,
			adb.StateOffline,
			adb.StateHost:
		default:
			log.Fatalf("adb-monitor: unknown listen message type: %#v", event)
		}
	}

	return watcher.Err()
}
