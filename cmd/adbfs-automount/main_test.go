package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zach-klippenstein/goadb"
)

func TestBuildDirNameForDevice(t *testing.T) {
	name := buildDirNameForDevice(&goadb.DeviceInfo{
		Model:      "foo1",
		Serial:     "bar2",
		Product:    "ignored",
		Usb:        "ignored",
		DeviceInfo: "ignored",
	})
	assert.Equal(t, "foo1-bar2", name)

	name = buildDirNameForDevice(&goadb.DeviceInfo{
		Model:  "-f-o-o_!@#$",
		Serial: "bar%^&*()",
	})
	assert.Equal(t, "-f-o-o_-bar_", name)
}
