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
