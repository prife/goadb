package adb

import (
	"fmt"
	"io"
	"net"
)

// RunShellCommand runs the specified commands on a shell on the device.
// From the Android docs:
//
//	Run 'command arg1 arg2 ...' in a shell on the device, and return
//	its output and error streams. Note that arguments must be separated
//	by spaces. If an argument contains a space, it must be quoted with
//	double-quotes. Arguments cannot contain double quotes or things
//	will go very wrong.
//	Note that this is the non-interactive version of "adb shell"
//
// Source: https://android.googlesource.com/platform/system/core/+/master/adb/SERVICES.TXT
// This method quotes the arguments for you, and will return an error if any of them
// contain double quotes.
//
// shell:echo 1
// 00000000  31 0a                                             |1.|
// shell:echo 12
// 00000000  31 32 0a                                          |12.|
//
// shell,v2:echo 1
// 00000000  01 02 00 00 00 31 0a 03  01 00 00 00 00           |.....1.......|
// shell,v2:echo 12
// 00000000  01 03 00 00 00 31 32 0a  03 01 00 00 00 00        |.....12.......|
//
// shell,v2:pm list pacakges -3
// 00000000  01 df 06 00 00 70 61 63  6b 61 67 65 3a 63 6f 6d  |.....package:com|
// ...
// 000006e0  65 61 64 0a 03 01 00 00  00 00                    |ead.......|
//
// shell,v2:pm clear kage:com.heytap.smarthome
// 00000000  02 9f 04 00 00 0a 45 78  63 65 70 74 69 6f 6e 20  |......Exception |
// ........
// 000004a0  39 39 29 0a 03 01 00 00  00 ff                    |99).......|
//
// v2协议，在应用输出的开头包裹了5个字符，其中的第2~5个字符似乎是小端表示的4字节长度
// 在应用输出的结尾包裹了6个字符，似乎总是 03 01 00 00 00 [00 or ff]
// 参考：https://stackoverflow.com/questions/13578416/read-binary-stdout-data-like-screencap-data-from-adb-shell
func (c *Device) RunShellCommand(v2 bool, cmd string, args ...string) (fn net.Conn, err error) {
	cmd, err = prepareCommandLine(cmd, args...)
	if err != nil {
		return nil, wrapClientError(err, c, "RunCommand")
	}

	conn, err := c.dialDevice()
	if err != nil {
		return nil, wrapClientError(err, c, "RunCommand")
	}

	var req string
	if v2 {
		req = fmt.Sprintf("shell,v2:%s", cmd)
	} else {
		req = fmt.Sprintf("shell:%s", cmd)
	}
	// req := fmt.Sprintf("shell,v2:%s", cmd)
	// req := fmt.Sprintf("shell,v2,TERM=xterm-256color:%s", cmd)

	// Shell responses are special, they don't include a length header.
	// We read until the stream is closed.
	// So, we can't use conn.RoundTripSingleResponse.
	if err = conn.SendMessage([]byte(req)); err != nil {
		conn.Close()
		return nil, wrapClientError(err, c, "RunCommand")
	}
	if _, err = conn.ReadStatus(req); err != nil {
		conn.Close()
		return nil, wrapClientError(err, c, "RunCommand")
	}

	return conn, wrapClientError(err, c, "RunCommand")
}

func (c *Device) RunCommand(cmd string, args ...string) ([]byte, error) {
	return c.RunCommandToEnd(false, cmd, args...)
}

func (c *Device) RunCommandToEnd(v2 bool, cmd string, args ...string) (resp []byte, err error) {
	reader, err := c.RunShellCommand(v2, cmd, args...)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	resp, err = io.ReadAll(reader)
	if err != nil {
		return
	}
	// fmt.Println(hex.Dump(resp))
	// fmt.Println("----------------")
	// fmt.Printf("%s", resp)
	if v2 {
		// trim prefix and suffix chars, see comments above
		if len(resp) >= (5 + 6) {
			resp = resp[5 : len(resp)-6]
		}
	}
	return
}
