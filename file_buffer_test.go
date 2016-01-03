package adbfs

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zach-klippenstein/goadb"
	"github.com/zach-klippenstein/goadb/util"
)

func TestNewFileBuffer_RdonlyExistSuccess(t *testing.T) {
	for _, config := range []struct {
		flags            FileOpenFlags
		expectedContents string
	}{
		{O_RDONLY, "hello"},
		{O_RDWR, "hello"},
		{O_RDWR | O_CREATE, "hello"},
	} {
		file := testSingleRegularFileBuffer(t, config.expectedContents)

		assert.NotNil(t, file)
		assert.Equal(t, config.expectedContents, file.Contents(), "%v", config)
	}
}

func TestNewFileBuffer_RdonlyNoExistFailure(t *testing.T) {
	file, err := NewFileBuffer(O_RDONLY, FileBufferOptions{
		Path: "/file",
		Client: &delegateDeviceClient{
			stat: statFiles(),
		},
	}, &LogEntry{})
	assert.True(t, util.HasErrCode(err, util.FileNoExistError))
	assert.Nil(t, file)
}

func TestNewFileBuffer_OpenWithoutReadFailure(t *testing.T) {
	for _, flags := range []FileOpenFlags{O_TRUNC, O_APPEND} {
		_, err := NewFileBuffer(flags, FileBufferOptions{
			Path:   "/file",
			Client: &delegateDeviceClient{},
		}, &LogEntry{})

		assert.Equal(t, os.ErrPermission, err)
	}
}

func TestFileBufferSync(t *testing.T) {
	dev := &delegateDeviceClient{
		stat: statFiles(&goadb.DirEntry{
			Name: "/file",
			Mode: 0664,
		}),
		openRead: openReadString("hello"),
	}
	file := newTestFileBuffer(t, O_RDONLY, FileBufferOptions{
		Path:   "/file",
		Client: dev,
	})
	assert.Equal(t, "hello", file.Contents())

	// Success.
	dev.openRead = openReadString("world")
	err := file.Sync(&LogEntry{})
	assert.NoError(t, err)
	assert.Equal(t, "world", file.Contents())

	// Failure.
	dev.openRead = openReadError(util.Errorf(util.NetworkError, "fail"))
	err = file.Sync(&LogEntry{})
	assert.Equal(t, `NetworkError: error opening file stream on device
caused by NetworkError: fail`, util.ErrorWithCauseChain(err))
	assert.Equal(t, "world", file.Contents())
}

func TestFileBufferReadAt(t *testing.T) {
	file := testSingleRegularFileBuffer(t, "hello world")

	var buf []byte
	var n int
	var err error

	// Read empty buffer.
	buf = make([]byte, 0)
	n, err = file.ReadAt(buf, 0)
	assert.Equal(t, 0, n)
	assert.NoError(t, err)
	assert.Equal(t, "", string(buf))

	// Read less than available.
	buf = make([]byte, 5)
	n, err = file.ReadAt(buf, 0)
	assert.Equal(t, len(buf), n)
	assert.NoError(t, err)
	assert.Equal(t, "hello", string(buf))

	// Read exactly available.
	buf = make([]byte, len("hello world"))
	n, err = file.ReadAt(buf, 0)
	assert.Equal(t, len(buf), n)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, "hello world", string(buf))

	// Read from middle of stream.
	buf = make([]byte, 5)
	n, err = file.ReadAt(buf, 6)
	assert.Equal(t, len(buf), n)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, "world", string(buf))

	// Try reading more than available.
	buf = make([]byte, 1024)
	n, err = file.ReadAt(buf, 0)
	assert.Equal(t, len("hello world"), n)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, "hello world", string(buf[:n]))

	// Try reading after last data.
	buf = make([]byte, 5)
	n, err = file.ReadAt(buf, 1024)
	assert.Equal(t, 0, n)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, "", string(buf[:n]))
}

func TestFileBuffer_RefCount(t *testing.T) {
	zeroRefCountHandlerCalled := false

	file := newTestFileBuffer(t, O_RDONLY, FileBufferOptions{
		Path: "/",
		ZeroRefCountHandler: func(f *FileBuffer) {
			zeroRefCountHandlerCalled = true
		},
		Client: &delegateDeviceClient{
			stat: statFiles(&goadb.DirEntry{
				Name: "/",
			}),
			openRead: openReadString(""),
		},
	})
	assert.Equal(t, 0, file.RefCount())
	assert.False(t, zeroRefCountHandlerCalled)

	assert.Equal(t, 1, file.IncRefCount())
	assert.Equal(t, 1, file.RefCount())

	assert.Equal(t, 0, file.DecRefCount())
	assert.Equal(t, 0, file.RefCount())
	assert.True(t, zeroRefCountHandlerCalled)

	func() {
		defer func() {
			p := recover()
			assert.NotNil(t, p)
			assert.Equal(t, "refcount decremented past 0", p)
		}()
		file.DecRefCount()
	}()
}

func newTestFileBuffer(t *testing.T, flags FileOpenFlags, opts FileBufferOptions) *FileBuffer {
	f, err := NewFileBuffer(flags, opts, &LogEntry{})
	assert.NoError(t, err)
	return f
}

func testSingleRegularFileBuffer(t *testing.T, contents string) *FileBuffer {
	return newTestFileBuffer(t, O_RDONLY, FileBufferOptions{
		Path: "/",
		Client: &delegateDeviceClient{
			stat: statFiles(&goadb.DirEntry{
				Name: "/",
				Mode: 0664,
			}),
			openRead: openReadString(contents),
		},
	})
}
