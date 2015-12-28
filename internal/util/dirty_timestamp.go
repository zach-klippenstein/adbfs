package util

import "time"

var zeroTime time.Time

// DirtyTimestamp is a boolean flag that knows the last time it was set from false to true.
// Zero value is unset.
type DirtyTimestamp struct {
	clock Clock
	t     time.Time
}

func NewDirtyTimestamp(clock Clock) *DirtyTimestamp {
	if clock == nil {
		clock = SystemClock
	}
	return &DirtyTimestamp{
		clock: clock,
	}
}

func (ts *DirtyTimestamp) IsSet() bool {
	return !ts.t.IsZero()
}

func (ts *DirtyTimestamp) Set() {
	if ts.t.IsZero() {
		ts.t = ts.clock.Now()
	}
}

func (ts *DirtyTimestamp) Clear() {
	ts.t = zeroTime
}

func (ts *DirtyTimestamp) HasBeenDirtyFor(d time.Duration) bool {
	return ts.IsSet() && ts.t.Add(d).Before(ts.clock.Now())
}
