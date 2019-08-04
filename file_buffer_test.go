package adbfs

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	. "github.com/sebastianhaberey/adbfs/internal/util"
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
		{O_RDWR | O_TRUNC, ""},
		{O_RDWR | O_APPEND, "hello"},
		{O_WRONLY, "hello"},
		{O_WRONLY | O_CREATE, "hello"},
		{O_WRONLY | O_TRUNC, ""},
		{O_WRONLY | O_APPEND, "hello"},
	} {
		file, err := NewFileBuffer(config.flags, FileBufferOptions{
			Path: "/file",
			Client: &delegateDeviceClient{
				stat: func(path string) (*adb.DirEntry, error) {
					return &adb.DirEntry{
						Name: "/file",
						Mode: 0664,
					}, nil
				},
				openRead:  openReadString("hello"),
				openWrite: openWriteNoop(),
			},
		}, &LogEntry{})

		assert.NoError(t, err)
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

		assert.Equal(t, ErrNotPermitted, err)
	}
}

func TestFileBuffer_LoadFromDevice(t *testing.T) {
	dev := &delegateDeviceClient{
		stat: statFiles(&adb.DirEntry{
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
	err := file.loadFromDevice(&LogEntry{})
	assert.NoError(t, err)
	assert.Equal(t, "world", file.Contents())

	// Failure.
	dev.openRead = openReadError(util.Errorf(util.NetworkError, "fail"))
	err = file.loadFromDevice(&LogEntry{})
	assert.Equal(t, `NetworkError: error opening file stream on device
caused by NetworkError: fail`, util.ErrorWithCauseChain(err))
	assert.Equal(t, "world", file.Contents())
}

func TestFileBuffer_SaveToDevice(t *testing.T) {
	var buf *bytes.Buffer
	dev := &delegateDeviceClient{
		stat: statFiles(&adb.DirEntry{
			Name: "/file",
			Mode: 0664,
		}),
	}

	buf = bytes.NewBufferString("hello world")
	dev.openWrite = openWriteTo(buf)
	file := newTestFileBuffer(t, O_WRONLY|O_TRUNC, FileBufferOptions{
		Path:   "/file",
		Client: dev,
	})
	assert.Equal(t, "", buf.String())
	assert.Equal(t, "", file.Contents())

	// Success.
	file.WriteAt([]byte("hello world"), 0)
	err := file.saveToDevice(&LogEntry{})
	assert.NoError(t, err)
	assert.Equal(t, "hello world", file.Contents())
	assert.Equal(t, "hello world", buf.String())

	// Failure.
	dev.openWrite = openWriteError(util.Errorf(util.NetworkError, "fail"))
	err = file.saveToDevice(&LogEntry{})
	assert.Equal(t, `NetworkError: error opening file stream on device
caused by NetworkError: fail`, util.ErrorWithCauseChain(err))
	assert.Equal(t, "hello world", file.Contents())
	assert.Equal(t, "hello world", buf.String())
}

func TestFileBuffer_RefCount(t *testing.T) {
	zeroRefCountHandlerCalled := false

	file := newTestFileBuffer(t, O_RDONLY, FileBufferOptions{
		Path: "/",
		ZeroRefCountHandler: func(f *FileBuffer) {
			zeroRefCountHandlerCalled = true
		},
		Client: &delegateDeviceClient{
			stat: statFiles(&adb.DirEntry{
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

func TestNewFileBuffer_NoExistCreateSuccess(t *testing.T) {
	file, err := NewFileBuffer(O_RDWR|O_CREATE, FileBufferOptions{
		Path: "/file",
		Client: &delegateDeviceClient{
			stat:      statFiles(),
			openWrite: openWriteNoop(),
		},
	}, &LogEntry{})
	assert.NoError(t, err)
	assert.NotNil(t, file)
}

func TestNewFileBuffer_ModFlagWithoutWriteFailure(t *testing.T) {
	for _, flags := range []FileOpenFlags{
		O_CREATE | O_RDONLY,
		O_TRUNC | O_RDONLY,
		O_APPEND | O_RDONLY,
	} {
		file, err := NewFileBuffer(flags, FileBufferOptions{
			Path: "/file",
			Client: &delegateDeviceClient{
				stat: func(path string) (*adb.DirEntry, error) {
					return &adb.DirEntry{
						Name: "/file",
						Mode: 0664,
					}, nil
				},
				openRead: func(path string) (io.ReadCloser, error) {
					return ioutil.NopCloser(strings.NewReader("hello")), nil
				},
			},
		}, &LogEntry{})
		assert.Equal(t, ErrNotPermitted, err)
		assert.Nil(t, file)
	}
}

func TestNewFileBuffer_PermsFromCorrectSource(t *testing.T) {
	const NoExist = DontSetPerms
	for _, perms := range []struct {
		// The perms requested in the Open call.
		Requested os.FileMode
		// The perms returned by Stat.
		StatResult os.FileMode
		Expected   os.FileMode
	}{
		{DontSetPerms, 0664, DefaultFilePermissions},
		{0600, 0664, 0600},
		{0600, NoExist, 0600},
		{DontSetPerms, NoExist, DefaultFilePermissions},
	} {
		file, err := NewFileBuffer(O_RDWR|O_CREATE, FileBufferOptions{
			Path:  "/file",
			Perms: perms.Requested,
			Client: &delegateDeviceClient{
				stat: func(path string) (*adb.DirEntry, error) {
					if perms.StatResult == NoExist {
						return nil, util.Errorf(util.FileNoExistError, "fail")
					}
					return &adb.DirEntry{
						Name: "/file",
						Mode: perms.StatResult,
					}, nil
				},
				openRead:  openReadString("hello"),
				openWrite: openWriteNoop(),
			},
		}, &LogEntry{})
		assert.NoError(t, err)
		assert.NotNil(t, file)
		actualPerms := file.Perms
		assert.Equal(t, perms.Expected, actualPerms, "expected %v, got %s", perms, actualPerms)
	}
}

func TestFileBuffer_SetSize(t *testing.T) {
	fbuf, dev := testSingleRegularRdwrFileBuffer(t, "hello world")
	assert.EqualValues(t, 11, fbuf.Size())
	assert.False(t, fbuf.IsDirty())

	fbuf.SetSize(0)
	assert.EqualValues(t, 0, fbuf.Size())
	assert.True(t, fbuf.IsDirty())
	assert.Empty(t, dev.String(), "set size shouldn't flush")
}

func TestFileBuffer_Flush(t *testing.T) {
	fbuf, dev := testSingleRegularRdwrFileBuffer(t, "hello world")
	assert.False(t, fbuf.IsDirty())
	assert.Empty(t, dev.String())

	// Flush is a no-op when not dirty.
	fbuf.Flush(&LogEntry{})
	assert.False(t, fbuf.IsDirty())
	assert.Empty(t, dev.String())

	// Dirty the buffer.
	fbuf.WriteAt([]byte{}, 0)
	fbuf.Flush(&LogEntry{})
	assert.False(t, fbuf.IsDirty())
	assert.Equal(t, "hello world", dev.String())
}

func newTestFileBuffer(t *testing.T, flags FileOpenFlags, opts FileBufferOptions) *FileBuffer {
	f, err := NewFileBuffer(flags, opts, &LogEntry{})
	assert.NoError(t, err)
	return f
}

func testSingleRegularRoFileBuffer(t *testing.T, contents string) *FileBuffer {
	return newTestFileBuffer(t, O_RDONLY, FileBufferOptions{
		Path: "/",
		Client: &delegateDeviceClient{
			stat: statFiles(&adb.DirEntry{
				Name: "/",
				Mode: 0664,
			}),
			openRead: openReadString(contents),
		},
	})
}

// testSingleRegularRdwrFileBuffer returns a *FileBuffer and a buffer that all saveToDevice calls
// will write to.
func testSingleRegularRdwrFileBuffer(t *testing.T, contents string) (*FileBuffer, *bytes.Buffer) {
	var buf bytes.Buffer
	return newTestFileBuffer(t, O_RDWR, FileBufferOptions{
		Path:  "/",
		Clock: &TestClock,
		Client: &delegateDeviceClient{
			stat: statFiles(&adb.DirEntry{
				Name: "/",
				Mode: 0664,
			}),
			openRead:  openReadString(contents),
			openWrite: openWriteTo(&buf),
		},
	}), &buf
}
