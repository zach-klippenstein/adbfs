package util

import (
	"io"
	"testing"

	"bytes"

	"github.com/stretchr/testify/assert"
	"strings"
)

func TestGrowableByteSlice_ReadAt(t *testing.T) {
	source := GrowableByteSlice{[]byte("hello world")}

	var buf []byte
	var n int
	var err error

	// Read into an empty buffer.
	buf = make([]byte, 0)
	n, err = source.ReadAt(buf, 0)
	assert.Equal(t, 0, n)
	assert.NoError(t, err)
	assert.Equal(t, "", string(buf))

	// Read less than available.
	buf = make([]byte, 5)
	n, err = source.ReadAt(buf, 0)
	assert.Equal(t, len(buf), n)
	assert.NoError(t, err)
	assert.Equal(t, "hello", string(buf))

	// Read exactly available.
	buf = make([]byte, len("hello world"))
	n, err = source.ReadAt(buf, 0)
	assert.Equal(t, len(buf), n)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, "hello world", string(buf))

	// Read from middle of stream.
	buf = make([]byte, 5)
	n, err = source.ReadAt(buf, 6)
	assert.Equal(t, len(buf), n)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, "world", string(buf))

	// Try reading more than available.
	buf = make([]byte, 1024)
	n, err = source.ReadAt(buf, 0)
	assert.Equal(t, len("hello world"), n)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, "hello world", string(buf[:n]))

	// Try reading after last data.
	buf = make([]byte, 5)
	n, err = source.ReadAt(buf, 1024)
	assert.Equal(t, 0, n)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, "", string(buf[:n]))
}

func TestGrowableByteSlice_WriteAt(t *testing.T) {
	var dest *GrowableByteSlice
	var n int
	var err error

	// Write to beginning of an empty buffer.
	dest = new(GrowableByteSlice)
	n, err = dest.WriteAt([]byte("hello"), 0)
	assert.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.EqualValues(t, 5, dest.Len())
	assert.Equal(t, "hello", dest.String())

	// Write after beginning of an empty buffer.
	dest = new(GrowableByteSlice)
	n, err = dest.WriteAt([]byte("world"), 6)
	assert.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.EqualValues(t, 11, dest.Len())
	assert.Equal(t, "\000\000\000\000\000\000world", dest.String())

	// Write at beginning of an non-empty buffer.
	dest = &GrowableByteSlice{[]byte("      world")}
	n, err = dest.WriteAt([]byte("hello"), 0)
	assert.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.EqualValues(t, 11, dest.Len())
	assert.Equal(t, "hello world", dest.String())

	// Write at end of an non-empty buffer.
	dest = &GrowableByteSlice{[]byte("hello ")}
	n, err = dest.WriteAt([]byte("world"), 6)
	assert.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.EqualValues(t, 11, dest.Len())
	assert.Equal(t, "hello world", dest.String())

	// Multiple writes.
	dest = new(GrowableByteSlice)
	dest.WriteAt([]byte("world"), 6)
	dest.WriteAt([]byte("hello"), 0)
	dest.WriteAt([]byte(" "), 5)
	assert.Equal(t, "hello world", dest.String())
}

func TestGrowableByteSlice_ReadWrite(t *testing.T) {
	dest := new(GrowableByteSlice)

	n, err := dest.WriteAt([]byte("a"), 65536)
	assert.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.EqualValues(t, 65537, dest.Len())

	buf := make([]byte, 3)
	n, err = dest.ReadAt(buf, 65535)
	assert.Equal(t, 2, n)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, "\000a", string(buf[:n]))
}

func TestGrowableByteSlice_Resize(t *testing.T) {
	dest := new(GrowableByteSlice)
	assert.EqualValues(t, 0, dest.Len())

	// Basic length check.
	dest.Resize(100)
	assert.EqualValues(t, 100, dest.Len())

	// Check that data is preserved during resize.
	dest = new(GrowableByteSlice)
	dest.WriteAt([]byte{'a'}, 0)
	dest.Resize(100)
	dest.WriteAt([]byte{'b'}, 100)
	dest.Resize(200)
	assert.EqualValues(t, 'a', dest.data[0])
	assert.EqualValues(t, 'b', dest.data[100])
	assert.Equal(t, bytes.Repeat([]byte{0}, 99), dest.data[1:100])
	assert.Equal(t, bytes.Repeat([]byte{0}, 99), dest.data[101:200])

	// Check that old data isn't leaked.
	dest = new(GrowableByteSlice)
	dest.WriteAt([]byte("hello"), 100)
	dest.Resize(0)
	dest.Resize(200)
	assert.Equal(t, bytes.Repeat([]byte{0}, 200), dest.data)
}

func TestGrowableByteSlice_WriteTo(t *testing.T) {
	data := &GrowableByteSlice{[]byte("hello world")}
	var buf bytes.Buffer
	n, _ := data.WriteTo(&buf)
	assert.EqualValues(t, 11, n)
	assert.Equal(t, "hello world", buf.String())
}

func TestGrowableByteSlice_ReadFrom(t *testing.T) {
	data := new(GrowableByteSlice)
	buf := strings.NewReader("hello world")
	n, _ := data.ReadFrom(buf)
	assert.EqualValues(t, 11, n)
	assert.Equal(t, "hello world", data.String())
}
