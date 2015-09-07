package fs

import (
	"os"
	"testing"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/stretchr/testify/assert"
	"github.com/zach-klippenstein/goadb"
)

func TestGetAttr_Root(t *testing.T) {
	dev := &delegateDeviceClient{
		stat: func(path string, log *LogEntry) (*goadb.DirEntry, error) {
			return &goadb.DirEntry{
				Name: "/",
				Size: 0,
				Mode: os.ModeDir | 0755,
			}, nil
		},
	}
	fs, err := NewAdbFileSystem(Config{
		Mountpoint:    "",
		ClientFactory: func() DeviceClient { return dev },
	})
	assert.NoError(t, err)

	attr, status := fs.GetAttr("", newContext(1, 2, 3))
	assertStatusOk(t, status)
	assert.NotNil(t, attr)

	assert.Equal(t, uint64(0), attr.Size)
	assert.False(t, attr.IsRegular())
	assert.True(t, attr.IsDir())
	assert.False(t, attr.IsBlock())
	assert.False(t, attr.IsChar())
	assert.False(t, attr.IsFifo())
	assert.False(t, attr.IsSocket())
	assert.False(t, attr.IsSymlink())
	assert.Equal(t, uint32(0755), attr.Mode&uint32(os.ModePerm))
}

func TestGetAttr_RegularFile(t *testing.T) {
	dev := &delegateDeviceClient{
		stat: func(path string, log *LogEntry) (*goadb.DirEntry, error) {
			return &goadb.DirEntry{
				Name: "/version.txt",
				Size: 42,
				Mode: 0444,
			}, nil
		},
	}
	fs, err := NewAdbFileSystem(Config{
		Mountpoint:    "",
		ClientFactory: func() DeviceClient { return dev },
	})
	assert.NoError(t, err)

	attr, status := fs.GetAttr("version.txt", newContext(1, 2, 3))
	assertStatusOk(t, status)
	assert.NotNil(t, attr)

	assert.Equal(t, uint64(42), attr.Size)
	assert.True(t, attr.IsRegular())
	assert.False(t, attr.IsDir())
	assert.False(t, attr.IsBlock())
	assert.False(t, attr.IsChar())
	assert.False(t, attr.IsFifo())
	assert.False(t, attr.IsSocket())
	assert.False(t, attr.IsSymlink())
	assert.Equal(t, uint32(0444), attr.Mode&uint32(os.ModePerm))
}

func newContext(uid, gid, pid int) *fuse.Context {
	return &fuse.Context{
		Owner: fuse.Owner{
			Uid: uint32(uid),
			Gid: uint32(gid),
		},
		Pid: uint32(pid),
	}
}

func assertStatusOk(t *testing.T, status fuse.Status) {
	assert.True(t, status.Ok(), "Expected status to be Ok, was %s", status)
}
