package adb

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/prife/goadb/internal/errors"
)

type DeviceInfo struct {
	// Always set.
	Serial string

	State string
	// Product, device, and model are not set in the short form.
	Product     string
	Model       string
	DeviceInfo  string
	TransportID int

	// Only set for devices connected via USB.
	Usb string
}

// IsUsb returns true if the device is connected via USB.
func (d *DeviceInfo) IsUsb() bool {
	return d.Usb != ""
}

func newDevice(serial, state string, attrs map[string]string) (*DeviceInfo, error) {
	if serial == "" {
		return nil, errors.AssertionErrorf("device serial cannot be blank")
	}

	var tid int
	tidstr, ok := attrs["transport_id"]
	if ok {
		value, err := strconv.Atoi(tidstr)
		if err == nil {
			tid = value
		}
	}

	return &DeviceInfo{
		Serial:      serial,
		State:       state,
		Product:     attrs["product"],
		Model:       attrs["model"],
		DeviceInfo:  attrs["device"],
		Usb:         attrs["usb"],
		TransportID: tid,
	}, nil
}

func parseDeviceList(list string, lineParseFunc func(string) (*DeviceInfo, error)) ([]*DeviceInfo, error) {
	devices := []*DeviceInfo{}
	scanner := bufio.NewScanner(strings.NewReader(list))

	for scanner.Scan() {
		device, err := lineParseFunc(scanner.Text())
		if err != nil {
			return nil, err
		}
		devices = append(devices, device)
	}

	return devices, nil
}

func parseDeviceShort(line string) (*DeviceInfo, error) {
	fields := strings.Fields(line)
	if len(fields) != 2 {
		return nil, errors.Errorf(errors.ParseError,
			"malformed device line, expected 2 fields but found %d", len(fields))
	}

	return newDevice(fields[0], fields[1], map[string]string{})
}

func readBuff(buf *bytes.Buffer, toSpace bool) ([]byte, error) {
	cbuf := buf.Bytes()

	for i, c := range cbuf {
		if toSpace {
			if c == '\t' || c == ' ' {
				return buf.Next(i), nil
			}
		} else {
			if !(c == '\t' || c == ' ') {
				return buf.Next(i), nil
			}
		}
	}
	return nil, fmt.Errorf("not found")
}

func parseDeviceLongE(line string) (*DeviceInfo, error) {
	invalidErr := errors.Errorf(errors.ParseError, "invalid line:%s", line)
	buf := bytes.NewBufferString(strings.TrimSpace(line))

	// Read serial
	serial, err := readBuff(buf, true)
	if err != nil {
		return nil, invalidErr
	}
	// skip spaces
	if _, err = readBuff(buf, false); err != nil {
		return nil, invalidErr
	}

	// Read state
	state, err := readBuff(buf, true)
	if err != nil {
		return nil, invalidErr
	}
	if _, err = readBuff(buf, false); err != nil {
		return nil, invalidErr
	}

	// Read attributes
	attrs := map[string]string{}
	// get the first 'key'
	rbuf, err := buf.ReadBytes(':')
	if err != nil {
		return nil, invalidErr
	}
	key := string(rbuf[:len(rbuf)-1])
	for {
		// get the value
		rbuf, err = buf.ReadBytes(':')
		if err != nil {
			value := string(rbuf)
			attrs[key] = value
			break
		}
		bi := bytes.LastIndexByte(rbuf, ' ')
		if bi < 0 {
			return nil, invalidErr
		}
		value := string(bytes.TrimSpace(rbuf[:bi]))
		attrs[key] = value

		// get the next key
		key = string(rbuf[bi+1 : len(rbuf)-1])
	}
	return newDevice(string(serial), string(state), attrs)
}

func parseDeviceLong(line string) (*DeviceInfo, error) {
	fields := strings.Fields(line)

	attrs := parseDeviceAttributes(fields[2:])
	return newDevice(fields[0], fields[1], attrs)
}

func parseDeviceAttributes(fields []string) map[string]string {
	attrs := map[string]string{}
	for _, field := range fields {
		key, val := parseKeyVal(field)
		attrs[key] = val
	}
	return attrs
}

// Parses a key:val pair and returns key, val.
func parseKeyVal(pair string) (string, string) {
	split := strings.Split(pair, ":")
	return split[0], split[1]
}
