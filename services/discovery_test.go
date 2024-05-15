package services_test

import (
	"context"
	"testing"

	"github.com/prife/goadb/services"
	"github.com/prife/gomlib"
	log "github.com/sirupsen/logrus"
)

func TestMonitor(t *testing.T) {
	// ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	// defer cancel()
	ctx := context.Background()

	registry := gomlib.NewRegistry()
	registry.Listen(gomlib.NewDeviceListener(
		func(ctx context.Context, d gomlib.DeviceEntry) {
			log.Infof("--> added device:%#v", d)
		}, func(ctx context.Context, d gomlib.DeviceEntry) {
			log.Infof("--> removed device:%#v", d)
		},
	))

	ch := make(chan error)
	go func() {
		log.Infoln("=== adb monitor begin === ")
		err := services.Monitor(ctx, registry)
		log.Errorln("=== adb monitor quit ===", err)
		ch <- err
	}()

	select {
	case <-ctx.Done():
	case err := <-ch:
		t.Fatalf("quit:%v", err)
	}
}
