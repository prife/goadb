package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"runtime"

	log "github.com/sirupsen/logrus"
)

func tcp_to_unix(tcp, unix string) error {
	listener, err := net.Listen("unix", unix)
	if err != nil {
		return fmt.Errorf("usbmuxd: fail to listen on: %v, error:%v", unix, err)
	}

	os.Chmod(unix, 0777)
	log.Debugln("listen on: ")
	for {
		conn, err := listener.Accept()
		if err != nil {
			return fmt.Errorf("usbmuxd: fail to listen accept: %v", err)
		}

		client, err := net.Dial("tcp", tcp)
		go func() {
			io.Copy(client, conn)
			client.Close()
		}()
		go func() {
			io.Copy(conn, client)
			conn.Close()
		}()
	}
}

func unix_to_tcp(unix, tcp string) error {
	listener, err := net.Listen("tcp", tcp)
	if err != nil {
		return fmt.Errorf("usbmuxd: fail to listen on: %v, error:%v", tcp, err)
	}

	log.Debugln("listen on: ")
	for {
		conn, err := listener.Accept()
		if err != nil {
			return fmt.Errorf("usbmuxd: fail to listen accept: %v", err)
		}

		client, err := net.Dial("unix", unix)
		go func() {
			io.Copy(client, conn)
		}()
		go func() {
			io.Copy(conn, client)
		}()
	}
}

func tcp_to_tcp(target, listen string) error {
	listener, err := net.Listen("tcp4", listen)
	if err != nil {
		return fmt.Errorf("usbmuxd: fail to listen on: %v, error:%v", listen, err)
	}

	log.Debugln("listen on: ")
	for {
		conn, err := listener.Accept()
		if err != nil {
			return fmt.Errorf("usbmuxd: fail to listen accept: %v", err)
		}

		client, err := net.Dial("tcp", target)
		go func() {
			buf := make([]byte, 1024*1024)
			for {
				n, err := conn.Read(buf)
				if n > 0 {
					fmt.Printf("--->\n%s\n", hex.Dump(buf[:n]))
					client.Write(buf[:n])
				}

				if err != nil {
					client.Close()
					break
				}
			}
		}()
		go func() {
			buf := make([]byte, 1024*1024)
			for {
				n, err := client.Read(buf)
				if n > 0 {
					fmt.Printf("<---\n%s\n", hex.Dump(buf[:n]))
					conn.Write(buf[:n])
				}

				if err != nil {
					conn.Close()
					break
				}
			}
		}()
	}
}

var mode = flag.String("mode", "tcp2tcp", "adb server port")

func initLog() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetReportCaller(true)
	log.SetFormatter(&log.TextFormatter{
		CallerPrettyfier: func(f *runtime.Frame) (string, string) {
			filename := path.Base(f.File)
			return "", fmt.Sprintf("%s:%d", filename, f.Line)
		},
	})
	log.SetLevel(log.InfoLevel)
}

func main() {
	flag.Parse()

	initLog()

	switch *mode {
	case "unix":
		panic(tcp_to_unix("127.0.0.1:27015", "/var/run/usbmuxd"))
	case "tcp":
		panic(unix_to_tcp("/var/run/usbmuxd", "127.0.0.1:27015"))
	case "tcp2tcp":
		panic(tcp_to_tcp("127.0.0.1:6000", "127.0.0.1:5037"))
	}
}
