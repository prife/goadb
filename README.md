# goadb

[![Build Status](https://travis-ci.org/prife/goadb.svg?branch=master)](https://travis-ci.org/prife/goadb)
[![GoDoc](https://godoc.org/github.com/prife/goadb?status.svg)](https://godoc.org/github.com/prife/goadb)

A Golang library for interacting with the Android Debug Bridge (adb).

See [demo.go](cmd/demo/demo.go) for usage.

`Adb.ListDevices()` and `Device.DeviceInfo()` expose WeTest's optional
`adbd_port` metadata as `DeviceInfo.AdbdPort`. This is the host forwarding
port for multi-device controllers, not the device's TCP property. Missing
or invalid metadata stays zero; callers must not substitute 5555 for USB
devices whose forwarding port is not ready. Standard ADB remains supported.

## Watching devices

`Adb.NewDeviceWatcher()` restarts the adb server when it dies, which is wrong
when something else owns the server. `Adb.NewDeviceWatcherWithContext(ctx)`
subscribes to such a server instead: it reconnects with exponential backoff
after a stream failure and never starts or restarts adb.

A brief interruption is not a disconnect. The watcher keeps the last device
list and re-diffs against it after reconnecting, so a server restart produces
no events at all if it comes back quickly. Only after 30 seconds without a
device list are the known devices reported disconnected, once; they are
rediscovered normally when the server returns.

Events are delivered in order on `C()`, and a slow consumer applies
backpressure rather than losing events. Cancel `ctx` or call the idempotent
`Shutdown()` to close the connection and join the worker; both are clean exits
and leave `Err()` nil.

Device states adb cannot serve commands for — `recovery`, `bootloader`,
`sideload`, and any state a future server adds — are reported as
`StateOffline`. One such device never invalidates the rest of the list.
