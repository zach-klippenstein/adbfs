package util

import "sync/atomic"

type AtomicBool int32

func (b *AtomicBool) Value() bool {
	return atomic.LoadInt32((*int32)(b)) != 0
}

// CompareAndSwap sets the value to newVal iff the current value is oldVal.
// If the comparison was successful, returns true.
func (b *AtomicBool) CompareAndSwap(oldVal, newVal bool) (swapped bool) {
	var oldIntVal int32 = 0
	if oldVal {
		oldIntVal = 1
	}
	var newIntVal int32 = 0
	if newVal {
		newIntVal = 1
	}
	return atomic.CompareAndSwapInt32((*int32)(b), oldIntVal, newIntVal)
}
