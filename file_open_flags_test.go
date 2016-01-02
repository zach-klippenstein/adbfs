package adbfs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFuseOpenFlagsCanRead(t *testing.T) {
	assert.True(t, O_RDONLY.CanRead())
	assert.True(t, O_RDWR.CanRead())
	assert.False(t, O_WRONLY.CanRead())
}
