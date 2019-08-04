package adbfs

import (
	"bytes"
	"testing"
	"time"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	. "github.com/sebastianhaberey/adbfs/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/zach-klippenstein/goadb/util"
)

func TestAdbFile_InnerFile(t *testing.T) {
	file := getAdbFile(NewAdbFile(AdbFileOpenOptions{
		FileBuffer: testSingleRegularRoFileBuffer(t, ""),
	}))

	assert.NotNil(t, file.InnerFile())
	assert.IsType(t, &WrappingFile{}, file.InnerFile())
}

func TestAdbFile_Release(t *testing.T) {
	fileBuf := testSingleRegularRoFileBuffer(t, "")
	fileBuf.IncRefCount()
	file := getAdbFile(NewAdbFile(AdbFileOpenOptions{
		FileBuffer: fileBuf,
	}))
	file.Release()

	assert.Equal(t, 0, fileBuf.RefCount())
}

func TestAdbFile_GetAttr(t *testing.T) {
	file := getAdbFile(NewAdbFile(AdbFileOpenOptions{
		FileBuffer: testSingleRegularRoFileBuffer(t, ""),
	}))

	attr := new(fuse.Attr)
	status := file.GetAttr(attr)
	assertStatusOk(t, status)
	assert.Equal(t, osFileModeToFuseFileMode(0664), attr.Mode)
}

func TestAdbFile_Fsync(t *testing.T) {
	fileBuf := testSingleRegularRoFileBuffer(t, "hello")
	file := getAdbFile(NewAdbFile(AdbFileOpenOptions{
		FileBuffer: fileBuf,
	}))
	assert.Equal(t, "hello", fileBuf.Contents())

	// Success.
	fileBuf.Client.(*delegateDeviceClient).openRead = openReadString("world")
	status := file.Fsync(0)
	assertStatusOk(t, status)
	assert.Equal(t, "world", fileBuf.Contents())

	// Failure.
	fileBuf.Client.(*delegateDeviceClient).openRead = openReadError(util.Errorf(util.NetworkError, ""))
	status = file.Fsync(0)
	assert.Equal(t, fuse.EIO, status)
	assert.Equal(t, "world", fileBuf.Contents())
}

func TestAdbFile_ReadWrOnly(t *testing.T) {
	file := getAdbFile(NewAdbFile(AdbFileOpenOptions{
		Flags:      O_WRONLY,
		FileBuffer: testSingleRegularRoFileBuffer(t, "hello world"),
	}))

	result, status := file.Read(make([]byte, 5), 0)
	assert.Equal(t, fuse.EPERM, status)
	assert.Equal(t, 0, result.Size())

	contents, status := result.Bytes(nil)
	assert.Equal(t, fuse.EPERM, status)
	assert.Empty(t, contents)
}

func TestAdbFile_Read(t *testing.T) {
	file := getAdbFile(NewAdbFile(AdbFileOpenOptions{
		FileBuffer: testSingleRegularRoFileBuffer(t, "hello world"),
	}))

	result, status := file.Read(make([]byte, 1024), 0)
	assertStatusOk(t, status)

	contents, status := result.Bytes(nil)
	assertStatusOk(t, status)
	assert.Equal(t, "hello world", string(contents))
}

func TestAdbFile_TruncateReadOnly(t *testing.T) {
	file := getAdbFile(NewAdbFile(AdbFileOpenOptions{
		FileBuffer: testSingleRegularRoFileBuffer(t, "hello world"),
		Flags:      O_RDONLY,
	}))

	status := file.Truncate(0)
	assert.Equal(t, fuse.EPERM, status)
}

func TestAdbFile_TruncateSuccess(t *testing.T) {
	TestClock.Reset()
	fbuf, dev := testSingleRegularRdwrFileBuffer(t, "hello world")
	file := getAdbFile(NewAdbFile(AdbFileOpenOptions{
		FileBuffer: fbuf,
		Flags:      O_RDWR,
	}))

	status := file.Truncate(0)
	assertStatusOk(t, status)
	assert.EqualValues(t, 0, file.FileBuffer.Size())
	assert.Equal(t, "", dev.String())

	status = file.Truncate(10)
	assertStatusOk(t, status)
	assert.EqualValues(t, 10, file.FileBuffer.Size())
	assert.Equal(t, bytes.Repeat([]byte{0}, 10), dev.Bytes())
}

func TestAdbFile_WriteSuccess(t *testing.T) {
	TestClock.Reset()
	fbuf, dev := testSingleRegularRdwrFileBuffer(t, "")
	fbuf.DirtyTimeout = 5 * time.Second
	file := getAdbFile(NewAdbFile(AdbFileOpenOptions{
		FileBuffer: fbuf,
		Flags:      O_RDWR,
	}))

	n, status := file.Write([]byte("hello world"), 0)
	assertStatusOk(t, status)
	assert.EqualValues(t, 11, n)
	assert.EqualValues(t, 11, file.FileBuffer.Size())
	// Write shouldn't flush unless too dirty.
	assert.Empty(t, dev.String())

	n, status = file.Write([]byte("goodbye"), 6)
	assertStatusOk(t, status)
	assert.EqualValues(t, 7, n)
	assert.EqualValues(t, 13, file.FileBuffer.Size())
	assert.Empty(t, dev.String())

	// Allow time to pass to trigger a flush.
	TestClock.Advance(6 * time.Second)
	n, status = file.Write([]byte("world"), 0)
	assertStatusOk(t, status)
	assert.Equal(t, "world goodbye", dev.String())
}

func TestAdbFile_WriteReadOnly(t *testing.T) {
	fbuf, dev := testSingleRegularRdwrFileBuffer(t, "")
	file := getAdbFile(NewAdbFile(AdbFileOpenOptions{
		FileBuffer: fbuf,
		Flags:      O_RDONLY,
	}))

	n, status := file.Write([]byte("hello world"), 0)
	assert.Equal(t, fuse.EPERM, status)
	assert.EqualValues(t, 0, n)
	assert.EqualValues(t, 0, file.FileBuffer.Size())
	assert.Empty(t, dev.String())
}

func TestAdbFile_FlushReadOnly(t *testing.T) {
	fbuf, _ := testSingleRegularRdwrFileBuffer(t, "")
	file := getAdbFile(NewAdbFile(AdbFileOpenOptions{
		FileBuffer: fbuf,
		Flags:      O_RDONLY,
	}))

	status := file.Flush()
	assertStatusOk(t, status)
}

func getAdbFile(file nodefs.File) *AdbFile {
	for {
		if file, ok := file.(*AdbFile); ok {
			return file
		}
		if file == nil {
			panic("no AdbFile")
		}
		file = file.InnerFile()
	}
}
