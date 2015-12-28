package util

import "time"

var (
	// Wraps time.Now().
	SystemClock = systemClock{}

	// A mock Clock object that can be used for tests.
	// Every call to Now() will advance time by 1 nanosecond.
	// Every test that relies on this should call TestClock.Reset() before using it.
	TestClock MockClock
)

type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time {
	return time.Now()
}

type MockClock time.Time

func (c *MockClock) Reset() {
	*c = MockClock(time.Unix(1, 0))
}

func (c *MockClock) Now() (now time.Time ){
	now = time.Time(*c)
	// 2 reads of Now should never return the same value.
	c.Advance(1 * time.Nanosecond)
	return
}

func (c *MockClock) Advance(d time.Duration) {
	*c = MockClock(time.Time(*c).Add(d))
}
