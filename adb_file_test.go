package adbfs

import (
	"testing"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/stretchr/testify/assert"
	"github.com/zach-klippenstein/goadb/util"
)

func TestAdbFile_InnerFile(t *testing.T) {
	file := getAdbFile(NewAdbFile(AdbFileOpenOptions{
		FileBuffer: testSingleRegularFileBuffer(t, ""),
	}))

	assert.NotNil(t, file.InnerFile())
	assert.IsType(t, &WrappingFile{}, file.InnerFile())
}

func TestAdbFile_Release(t *testing.T) {
	fileBuf := testSingleRegularFileBuffer(t, "")
	fileBuf.IncRefCount()
	file := getAdbFile(NewAdbFile(AdbFileOpenOptions{
		FileBuffer: fileBuf,
	}))
	file.Release()

	assert.Equal(t, 0, fileBuf.RefCount())
}

func TestAdbFile_GetAttr(t *testing.T) {
	file := getAdbFile(NewAdbFile(AdbFileOpenOptions{
		FileBuffer: testSingleRegularFileBuffer(t, ""),
	}))

	attr := new(fuse.Attr)
	status := file.GetAttr(attr)
	assertStatusOk(t, status)
	assert.Equal(t, osFileModeToFuseFileMode(0664), attr.Mode)
}

func TestAdbFile_Fsync(t *testing.T) {
	fileBuf := testSingleRegularFileBuffer(t, "hello")
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
		FileBuffer: testSingleRegularFileBuffer(t, "hello world"),
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
		FileBuffer: testSingleRegularFileBuffer(t, "hello world"),
	}))

	result, status := file.Read(make([]byte, 1024), 0)
	assertStatusOk(t, status)

	contents, status := result.Bytes(nil)
	assertStatusOk(t, status)
	assert.Equal(t, "hello world", string(contents))
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
