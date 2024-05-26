package adb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_parseUptime(t *testing.T) {
	upttimestr := "73681.99 69586.45"
	uptime, err := parseUptime([]byte(upttimestr))
	assert.Nil(t, err)
	assert.Equal(t, uptime, float64(73681.99))

	upttimestr = "\x0D\x0A73681.99 69586.45\x0D\x0A"
	uptime, err = parseUptime([]byte(upttimestr))
	assert.Nil(t, err)
	assert.Equal(t, uptime, float64(73681.99))

	upttimestr = "\x0D\x0A73681.99    69586.45\x0D\x0A"
	uptime, err = parseUptime([]byte(upttimestr))
	assert.Nil(t, err)
	assert.Equal(t, uptime, float64(73681.99))
}
