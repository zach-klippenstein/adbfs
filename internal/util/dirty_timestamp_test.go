package util

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDirtyTimestamp(t *testing.T) {
	TestClock.Reset()
	ts := NewDirtyTimestamp(&TestClock)
	assert.False(t, ts.IsSet())
	assert.False(t, ts.HasBeenDirtyFor(0))

	ts.Set()
	assert.True(t, ts.IsSet())
	assert.True(t, ts.HasBeenDirtyFor(0))
	assert.False(t, ts.HasBeenDirtyFor(5*time.Minute))

	ts.Clear()
	assert.False(t, ts.IsSet())

	// Check timing behavior with multiple sets and elapsed time.
	ts.Set()
	TestClock.Advance(501 * time.Millisecond)
	ts.Set()
	assert.True(t, ts.HasBeenDirtyFor(0))
	assert.True(t, ts.HasBeenDirtyFor(250*time.Millisecond))
	assert.True(t, ts.HasBeenDirtyFor(500*time.Millisecond))
	assert.False(t, ts.HasBeenDirtyFor(1*time.Second))
}
