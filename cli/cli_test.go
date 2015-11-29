package cli

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAdbfsConfigAsArgs(t *testing.T) {
	config := AdbfsConfig{
		AdbPort:            10,
		ConnectionPoolSize: 20,
		LogLevel:           "warn",
		CacheTtl:           30 * time.Second,
		ServeDebug:         true,
	}

	expectedArgs := []string{
		"--port=10",
		"--pool=20",
		"--log=warn",
		"--cachettl=30s",
		"--debug",
	}

	assert.Equal(t, expectedArgs, config.AsArgs())
}

func TestFormatBoolFlag(t *testing.T) {
	assert.Equal(t, "--debug", FormatBoolFlag("debug", true))
	assert.Equal(t, "--no-debug", FormatBoolFlag("debug", false))
}
