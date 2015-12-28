package util

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
)

const (
	// Once initialized, capacity will never drop below this number.
	initialGrowableByteSliceCapacity = 1024
)

/*
Buffer is used to hold file data in memory.
Slightly different and simpler behavior than bytes.Buffer.
Wraps a byte slice, and can grow it preserving existing data,
truncate it larger or smaller.

Indices are specified in int64s, not ints. Currently the slice is implemented
as a single underlying byte slice, so math.IntMax is the maximum length.
*/
type GrowableByteSlice struct {
	data []byte
}

var (
	// Read interfaces.
	_ io.ReaderAt = &GrowableByteSlice{}
	_ io.WriterTo = &GrowableByteSlice{}

	// Write interfaces.
	_ io.WriterAt   = &GrowableByteSlice{}
	_ io.ReaderFrom = &GrowableByteSlice{}
)

func (s *GrowableByteSlice) String() string {
	return string(s.data)
}

func (s *GrowableByteSlice) GoString() string {
	return fmt.Sprintf("GrowableSlice(len=%d,cap=%d)", len(s.data), cap(s.data))
}

// Resize changes the len of the slice, re-allocating if necessary, to be newLen.
// If the new len is larger, the "new" bytes at the end of the buffer will always be zeroed out.
func (s *GrowableByteSlice) Resize(newLen64 int64) {
	newLen := int(newLen64)
	switch {
	case newLen < 0:
		panic("newLen must be >= 0")
	case newLen < len(s.data):
		s.shrink(newLen)
	case newLen == len(s.data):
		return
	case newLen <= cap(s.data):
		s.data = s.data[:newLen]
	default: // newLen > cap(s.data)
		s.reallocate(newLen)
	}
}

// shrink returns a byte slice that has len < newLen and the same initial bytes copied from s.data.
func (s *GrowableByteSlice) shrink(newLen int) {
	if newLen >= len(s.data) {
		panic("cannot shrink larger")
	}

	if len(s.data) < cap(s.data)/3 {
		// Reallocate to avoid leaking memory.
		s.reallocate(newLen)
	} else {
		// Only shrinking a little, we can re-use the array.
		// …but zero out the old data so it doesn't leak next time we grow past newLen without reallocating.
		for i := range s.data[newLen:] {
			s.data[i] = 0
		}
		s.data = s.data[:newLen]
	}
}

// reallocate replaces the underlying array with a new array that has len newLen and capacity 2*newLen
// and copies data over.
func (s *GrowableByteSlice) reallocate(newLen int) {
	newCapacity := 2 * newLen
	if newCapacity < initialGrowableByteSliceCapacity {
		newCapacity = initialGrowableByteSliceCapacity
	}

	newData := make([]byte, newLen, newCapacity)
	copy(newData, s.data)
	s.data = newData
}

// ReadAt implements the io.ReaderAt interface.
func (s *GrowableByteSlice) ReadAt(buf []byte, off int64) (n int, err error) {
	if off >= int64(len(s.data)) {
		return 0, io.EOF
	}

	// Don't use Slice because we don't want to grow the slice.
	n = copy(buf, s.data[off:])
	if n+int(off) == len(s.data) {
		// Didn't copy the entire buffer, so buf must be bigger than the rest of our buffer.
		err = io.EOF
	}
	return
}

// WriteAt implements the io.WriterAt interface.
func (s *GrowableByteSlice) WriteAt(data []byte, off int64) (int, error) {
	end := off + int64(len(data))
	copy(s.slice(off, end), data)
	return len(data), nil
}

// slice returns the bytes in b[start:end], and will
// grow the slice if end > b.Len().
func (s *GrowableByteSlice) slice(start, end int64) []byte {
	if start > end {
		panic(fmt.Sprintf("start(%d) > end(%d)", start, end))
	}

	if end > int64(len(s.data)) {
		s.Resize(int64(end))
	}

	return s.data[start:end]
}

func (s *GrowableByteSlice) WriteTo(w io.Writer) (int64, error) {
	return io.Copy(w, bytes.NewReader(s.data))
}

// ReadFrom resizes the slice to 0 then reads all of r.
func (s *GrowableByteSlice) ReadFrom(r io.Reader) (int64, error) {
	data, err := ioutil.ReadAll(r)
	s.data = data
	return int64(len(data)), err
}

func (s *GrowableByteSlice) Len() int64 {
	return int64(len(s.data))
}
