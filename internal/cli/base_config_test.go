package cli

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAdbfsConfigAsArgs(t *testing.T) {
	config := BaseConfig{
		AdbPort:            10,
		ConnectionPoolSize: 20,
		LogLevel:           "warn",
		CacheTtl:           30 * time.Second,
		ServeDebug:         true,
		DeviceRoot:         "/abc",
		ReadOnly:           true,
	}

	expectedArgs := []string{
		"--port=10",
		"--pool=20",
		"--log=warn",
		"--cachettl=30s",
		"--debug",
		"--no-verbose",
		"--device-root=/abc",
		"--readonly",
	}

	assert.Equal(t, expectedArgs, config.AsArgs())
}

func TestFormatBoolFlag(t *testing.T) {
	assert.Equal(t, "--debug", formatFlag("debug", true))
	assert.Equal(t, "--no-debug", formatFlag("debug", false))
}
