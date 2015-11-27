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
		"-port=10",
		"-poolsize=20",
		"-loglevel=warn",
		"-cachettl=30s",
		"-debug=true",
	}

	assert.Equal(t, expectedArgs, config.AsArgs())
}
