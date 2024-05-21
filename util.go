package adb

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	whitespaceRegex = regexp.MustCompile(`^\s*$`)
)

func containsWhitespace(str string) bool {
	return strings.ContainsAny(str, " \t\v")
}

func isBlank(str string) bool {
	return whitespaceRegex.MatchString(str)
}

func wrapClientError(err error, client *Device, operation string, args ...interface{}) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("%s on %s, err: %w", fmt.Sprintf(operation, args...), client.descriptor.serial, err)
}
