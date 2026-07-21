package adb

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/prife/goadb/wire"
)

// ShellExitCodeUnavailable is returned when Shell cannot obtain a remote exit
// status because validation, transport, protocol, writer, or context handling
// failed.
const ShellExitCodeUnavailable = -1

// ErrShellV2Unsupported reports that the selected device does not advertise
// the ADB shell_v2 feature.
//
// Shell deliberately does not fall back to shell v1. The v1 protocol merges
// stdout and stderr and does not report a trustworthy remote exit status.
// Callers that can tolerate those limitations may explicitly use ShellV1Raw
// after checking this error with errors.Is.
var ErrShellV2Unsupported = errors.New("adb shell v2 is unsupported")

type shellResult struct {
	exitCode int
	err      error
}

// Shell runs one argv-style command using ADB's shell v2 protocol.
//
// Stdout and stderr are streamed without text conversion, so stdout may be a
// binary file. A nil writer discards that stream. A non-zero remote exit code
// is returned as exitCode with a nil error; err is reserved for validation,
// transport, protocol, writer, and context failures. When err is non-nil,
// exitCode is ShellExitCodeUnavailable.
//
// command and args are shell-quoted independently. Shell does not accept a
// preformatted command line. Pass "sh", "-c", script as command and args when
// shell syntax is intentionally required.
func (c *Device) Shell(
	ctx context.Context,
	stdout io.Writer,
	stderr io.Writer,
	command string,
	args ...string,
) (exitCode int, err error) {
	if ctx == nil {
		return ShellExitCodeUnavailable, wrapClientError(
			errors.New("context cannot be nil"), c, "Shell")
	}
	if err := ctx.Err(); err != nil {
		return ShellExitCodeUnavailable, wrapClientError(err, c, "Shell")
	}

	commandLine, err := prepareShellCommandLine(command, args...)
	if err != nil {
		return ShellExitCodeUnavailable, wrapClientError(err, c, "Shell")
	}

	features, err := c.DeviceFeatures()
	if err != nil {
		return ShellExitCodeUnavailable, wrapClientError(err, c, "Shell")
	}
	if !features[FeatureShell2] {
		return ShellExitCodeUnavailable, wrapClientError(
			ErrShellV2Unsupported, c, "Shell")
	}

	conn, err := c.dialDevice(c.CmdTimeoutShort)
	if err != nil {
		return ShellExitCodeUnavailable, wrapClientError(err, c, "Shell")
	}
	defer conn.Close()

	req := "shell,v2,raw:" + commandLine
	if err := conn.SendMessage([]byte(req)); err != nil {
		return ShellExitCodeUnavailable, wrapClientError(err, c, "Shell")
	}
	if _, err := readStatusWithTimeout(conn, req, c.CmdTimeoutShort); err != nil {
		return ShellExitCodeUnavailable, wrapClientError(err, c, "Shell")
	}

	transport := newShellTransport(conn, 0)
	if err := transport.Send(shellCloseStdin, nil); err != nil {
		return ShellExitCodeUnavailable, wrapClientError(err, c, "Shell")
	}

	resultCh := make(chan shellResult, 1)
	go func() {
		code, readErr := readShellV2(transport, stdout, stderr)
		resultCh <- shellResult{exitCode: code, err: readErr}
	}()

	select {
	case result := <-resultCh:
		if result.err != nil {
			return ShellExitCodeUnavailable, wrapClientError(result.err, c, "Shell")
		}
		return result.exitCode, nil
	case <-ctx.Done():
		// Closing the connection unblocks the reader. Wait for it to finish so
		// cancellation never leaves a background shell goroutine behind.
		closeErr := conn.Close()
		<-resultCh
		cancelErr := ctx.Err()
		if closeErr != nil {
			cancelErr = fmt.Errorf("%w; closing shell connection: %v", cancelErr, closeErr)
		}
		return ShellExitCodeUnavailable, wrapClientError(cancelErr, c, "Shell")
	}
}

// ShellV1Raw runs one safely quoted argv-style command using the legacy ADB
// shell protocol and streams its combined output without text conversion.
//
// This API exists for old devices that return ErrShellV2Unsupported from
// Shell. The legacy protocol combines stdout and stderr, has no remote exit
// status, and cannot distinguish normal EOF from a transport disconnect.
// Therefore a nil error only means that the raw stream ended. Callers pulling
// a file must independently verify its expected byte size and contents.
//
// Cancellation closes the connection and waits for the reader goroutine to
// stop. A nil output writer discards the stream.
func (c *Device) ShellV1Raw(
	ctx context.Context,
	output io.Writer,
	command string,
	args ...string,
) error {
	if ctx == nil {
		return wrapClientError(errors.New("context cannot be nil"), c, "ShellV1Raw")
	}
	if err := ctx.Err(); err != nil {
		return wrapClientError(err, c, "ShellV1Raw")
	}

	commandLine, err := prepareShellCommandLine(command, args...)
	if err != nil {
		return wrapClientError(err, c, "ShellV1Raw")
	}

	conn, err := c.dialDevice(c.CmdTimeoutShort)
	if err != nil {
		return wrapClientError(err, c, "ShellV1Raw")
	}
	defer conn.Close()

	req := "shell:" + commandLine
	if err := conn.SendMessage([]byte(req)); err != nil {
		return wrapClientError(err, c, "ShellV1Raw")
	}
	if _, err := readStatusWithTimeout(conn, req, c.CmdTimeoutShort); err != nil {
		return wrapClientError(err, c, "ShellV1Raw")
	}

	resultCh := make(chan error, 1)
	go func() {
		resultCh <- readShellV1Raw(conn, output)
	}()

	select {
	case resultErr := <-resultCh:
		return wrapClientError(resultErr, c, "ShellV1Raw")
	case <-ctx.Done():
		closeErr := conn.Close()
		<-resultCh
		cancelErr := ctx.Err()
		if closeErr != nil {
			cancelErr = fmt.Errorf("%w; closing shell connection: %v", cancelErr, closeErr)
		}
		return wrapClientError(cancelErr, c, "ShellV1Raw")
	}
}

func readShellV1Raw(reader io.Reader, output io.Writer) error {
	if output == nil {
		output = io.Discard
	}
	buffer := make([]byte, wire.SyncMaxChunkSize)
	for {
		count, readErr := reader.Read(buffer)
		if count > 0 {
			if err := writeShellStream(output, buffer[:count]); err != nil {
				return fmt.Errorf("write legacy shell output: %w", err)
			}
		}
		if errors.Is(readErr, io.EOF) {
			return nil
		}
		if readErr != nil {
			return fmt.Errorf("read legacy shell output: %w", readErr)
		}
		if count == 0 {
			return fmt.Errorf("read legacy shell output: %w", io.ErrNoProgress)
		}
	}
}

func readShellV2(transport shellTransport, stdout, stderr io.Writer) (int, error) {
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}

	for {
		messageType, payload, err := transport.Read()
		if errors.Is(err, io.EOF) {
			return ShellExitCodeUnavailable, &ExitMissingError{}
		}
		if err != nil {
			return ShellExitCodeUnavailable, err
		}

		switch messageType {
		case shellStdout:
			if err := writeShellStream(stdout, payload); err != nil {
				return ShellExitCodeUnavailable, fmt.Errorf("write shell stdout: %w", err)
			}
		case shellStderr:
			if err := writeShellStream(stderr, payload); err != nil {
				return ShellExitCodeUnavailable, fmt.Errorf("write shell stderr: %w", err)
			}
		case shellExit:
			if len(payload) != 1 {
				return ShellExitCodeUnavailable, fmt.Errorf(
					"invalid shell exit payload length %d", len(payload))
			}
			return int(payload[0]), nil
		default:
			return ShellExitCodeUnavailable, fmt.Errorf(
				"unexpected shell message type %d", messageType)
		}
	}
}

func writeShellStream(writer io.Writer, payload []byte) error {
	written, err := writer.Write(payload)
	if err != nil {
		return err
	}
	if written != len(payload) {
		return io.ErrShortWrite
	}
	return nil
}

func prepareShellCommandLine(command string, args ...string) (string, error) {
	if isBlank(command) {
		return "", fmt.Errorf("%w: command cannot be empty", wire.ErrAssertion)
	}

	argv := make([]string, 0, len(args)+1)
	argv = append(argv, command)
	argv = append(argv, args...)
	for index, arg := range argv {
		if strings.IndexByte(arg, 0) >= 0 {
			return "", fmt.Errorf(
				"%w: argv at index %d contains a NUL byte", wire.ErrParse, index)
		}
		argv[index] = quoteShellArg(arg)
	}
	return strings.Join(argv, " "), nil
}

func quoteShellArg(arg string) string {
	return "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
}

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

	conn, err := c.dialDevice(c.CmdTimeoutShort)
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
	// fmt.Println("run command: ", req)
	if err = conn.SendMessage([]byte(req)); err != nil {
		conn.Close()
		return nil, wrapClientError(err, c, "RunCommand")
	}

	if _, err = readStatusWithTimeout(conn, req, c.CmdTimeoutShort); err != nil {
		conn.Close()
		return nil, wrapClientError(err, c, "RunCommand")
	}

	return conn, wrapClientError(err, c, "RunCommand")
}

func (c *Device) RunCommandTimeout(timeout time.Duration, cmd string, args ...string) (resp []byte, err error) {
	conn, err := c.RunShellCommand(false, cmd, args...)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// set read timeout
	if timeout > 0 {
		if err = conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
			return nil, wrapClientError(err, c, "RunCommand")
		}
	}
	resp, err = io.ReadAll(conn)
	if err != nil {
		return
	}
	// fmt.Println(hex.Dump(resp))
	// fmt.Println("----------------")
	// fmt.Printf("%s", resp)
	return
}

// RunCommand default timeout is CommandTimeoutShortDefault which is 2 seconds, be careful
func (c *Device) RunCommand(cmd string, args ...string) ([]byte, error) {
	return c.RunCommandTimeout(c.CmdTimeoutShort, cmd, args...)
}

// RunCommandCtx wrap RunShellCommand with context
func (c *Device) RunCommandCtx(ctx context.Context, writer io.Writer, cmd string, args ...string) error {
	conn, err := c.RunShellCommand(false, cmd, args...)
	if err != nil {
		return err
	}
	defer conn.Close()
	buf := make([]byte, wire.SyncMaxChunkSize)
	ch := make(chan error, 2)
	go func() {
		for {
			n, err := conn.Read(buf)
			if writer != nil && n > 0 {
				writer.Write(buf[:n])
			}

			// shell v1 协议无法确认 connection 结束的真正原因，实际测试效果如下：
			// 1. 手机 adb 连接正常，程序正常结束，err返回 EOF
			// 2. 手机 adb 连接正常，kill 掉正在执行的程序，err 也会返回 EOF
			// 3. 如果进程执行中，断开 USB 线，err 还会返回 EOF
			// 综上: 需要支持 v2 协议，才有可能区分上述三种情况。
			if err != nil {
				// fmt.Println("err:", err)
				ch <- io.EOF
				return
			}
		}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-ch:
		if err == io.EOF {
			return nil
		}
		return err
	}
}

func (c *Device) RunCommandOutputCtx(ctx context.Context, cmd string, args ...string) ([]byte, error) {
	buf := &bytes.Buffer{}
	err := c.RunCommandCtx(ctx, buf, cmd, args...)
	return buf.Bytes(), err
}
