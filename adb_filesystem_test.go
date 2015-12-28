package adbfs

import (
	"fmt"
	"os"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/hanwen/go-fuse/fuse"
	"github.com/stretchr/testify/assert"
	"github.com/zach-klippenstein/goadb"
)

type MockDeviceWatcher struct{}

func (MockDeviceWatcher) C() <-chan goadb.DeviceStateChangedEvent {
	return make(chan goadb.DeviceStateChangedEvent)
}

func (MockDeviceWatcher) Err() error {
	return nil
}

func (MockDeviceWatcher) Shutdown() {
}

func TestGetAttr_Root(t *testing.T) {
	dev := &delegateDeviceClient{
		stat: func(path string) (*goadb.DirEntry, error) {
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
		DeviceWatcher: MockDeviceWatcher{},
		Log:           logrus.StandardLogger(),
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
		stat: func(path string) (*goadb.DirEntry, error) {
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
		DeviceWatcher: MockDeviceWatcher{},
		Log:           logrus.StandardLogger(),
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

func TestReadLink_AbsoluteTarget(t *testing.T) {
	dev := &delegateDeviceClient{
		runCommand: func(cmd string, args []string) (string, error) {
			if cmd == "readlink" && args[0] == "/version_link.txt" {
				return "/version.txt\r\n", nil
			}
			t.Fatal("invalid command:", cmd, args)
			return "", nil
		},
	}
	fs, err := NewAdbFileSystem(Config{
		Mountpoint:    "/foo/bar",
		ClientFactory: func() DeviceClient { return dev },
		DeviceWatcher: MockDeviceWatcher{},
		Log:           logrus.StandardLogger(),
	})
	assert.NoError(t, err)

	target, status := fs.Readlink("version_link.txt", newContext(1, 2, 3))
	//	assert.NoError(t, err)
	assertStatusOk(t, status)
	assert.Equal(t, "/foo/bar/version.txt", target)
}

func TestReadLink_RelativeTarget(t *testing.T) {
	dev := &delegateDeviceClient{
		runCommand: func(cmd string, args []string) (string, error) {
			if cmd == "readlink" && args[0] == "/version_link.txt" {
				return "version.txt\r\n", nil
			}
			t.Fatal("invalid command:", cmd, args)
			return "", nil
		},
	}
	fs, err := NewAdbFileSystem(Config{
		Mountpoint:    "/foo/bar",
		ClientFactory: func() DeviceClient { return dev },
		DeviceWatcher: MockDeviceWatcher{},
		Log:           logrus.StandardLogger(),
	})
	assert.NoError(t, err)

	target, status := fs.Readlink("version_link.txt", newContext(1, 2, 3))
	assertStatusOk(t, status)
	assert.Equal(t, "version.txt", target)
}

func TestReadLink_NotALink(t *testing.T) {
	dev := &delegateDeviceClient{
		runCommand: func(cmd string, args []string) (string, error) {
			return ReadlinkInvalidArgument, nil
		},
	}
	fs, err := NewAdbFileSystem(Config{
		Mountpoint:    "/foo/bar",
		ClientFactory: func() DeviceClient { return dev },
		DeviceWatcher: MockDeviceWatcher{},
		Log:           logrus.StandardLogger(),
	})
	assert.NoError(t, err)

	_, status := fs.Readlink("version_link.txt", newContext(1, 2, 3))
	assert.Equal(t, fuse.EINVAL, status)
}

func TestReadLink_PermissionDenied(t *testing.T) {
	dev := &delegateDeviceClient{
		runCommand: func(cmd string, args []string) (string, error) {
			if cmd == "readlink" && args[0] == "/version_link.txt" {
				return ReadlinkPermissionDenied, nil
			}
			t.Fatal("invalid command:", cmd, args)
			return "", nil
		},
	}
	fs, err := NewAdbFileSystem(Config{
		Mountpoint:    "/foo/bar",
		ClientFactory: func() DeviceClient { return dev },
		DeviceWatcher: MockDeviceWatcher{},
		Log:           logrus.StandardLogger(),
	})
	assert.NoError(t, err)

	_, status := fs.Readlink("version_link.txt", newContext(1, 2, 3))
	assert.Equal(t, fuse.EPERM, status)
}

func TestMkdir_Success(t *testing.T) {
	dev := &delegateDeviceClient{
		runCommand: func(cmd string, args []string) (string, error) {
			if cmd == "mkdir" && args[0] == "/newdir" {
				return "", nil
			}
			t.Fatal("invalid command:", cmd, args)
			return "", nil
		},
	}
	fs, err := NewAdbFileSystem(Config{
		Mountpoint:    "",
		ClientFactory: func() DeviceClient { return dev },
		DeviceWatcher: MockDeviceWatcher{},
		Log:           logrus.StandardLogger(),
	})
	assert.NoError(t, err)

	status := fs.Mkdir("newdir", 0, newContext(1, 2, 3))
	assertStatusOk(t, status)
}

func TestMkdir_Error(t *testing.T) {
	dev := &delegateDeviceClient{
		runCommand: func(cmd string, args []string) (string, error) {
			if cmd == "mkdir" {
				return fmt.Sprintf("mkdir failed for %s, Read-only file system", args[0]), nil
			}
			t.Fatal("invalid command:", cmd, args)
			return "", nil
		},
	}
	fs, err := NewAdbFileSystem(Config{
		Mountpoint:    "",
		ClientFactory: func() DeviceClient { return dev },
		DeviceWatcher: MockDeviceWatcher{},
		Log:           logrus.StandardLogger(),
	})
	assert.NoError(t, err)

	status := fs.Mkdir("newdir", 0, newContext(1, 2, 3))
	assert.Equal(t, fuse.EACCES, status)
}

func TestRename_Success(t *testing.T) {
	dev := &delegateDeviceClient{
		runCommand: func(cmd string, args []string) (string, error) {
			if cmd == "mv" && args[0] == "/old" && args[1] == "/new" {
				return "", nil
			}
			t.Fatal("invalid command:", cmd, args)
			return "", nil
		},
	}
	fs, err := NewAdbFileSystem(Config{
		Mountpoint:    "",
		ClientFactory: func() DeviceClient { return dev },
		DeviceWatcher: MockDeviceWatcher{},
		Log:           logrus.StandardLogger(),
	})
	assert.NoError(t, err)

	status := fs.Rename("old", "new", newContext(1, 2, 3))
	assertStatusOk(t, status)
}

func TestRename_Error(t *testing.T) {
	dev := &delegateDeviceClient{
		runCommand: func(cmd string, args []string) (string, error) {
			if cmd == "mv" {
				return fmt.Sprintf("mv failed for %s, Read-only file system", args[0]), nil
			}
			t.Fatal("invalid command:", cmd, args)
			return "", nil
		},
	}
	fs, err := NewAdbFileSystem(Config{
		Mountpoint:    "",
		ClientFactory: func() DeviceClient { return dev },
		DeviceWatcher: MockDeviceWatcher{},
		Log:           logrus.StandardLogger(),
	})
	assert.NoError(t, err)

	status := fs.Rename("old", "new", newContext(1, 2, 3))
	assert.Equal(t, fuse.EACCES, status)
}

func TestRmdir_Success(t *testing.T) {
	dev := &delegateDeviceClient{
		runCommand: func(cmd string, args []string) (string, error) {
			if cmd == "rmdir" && args[0] == "/dir" {
				return "", nil
			}
			t.Fatal("invalid command:", cmd, args)
			return "", nil
		},
	}
	fs, err := NewAdbFileSystem(Config{
		Mountpoint:    "",
		ClientFactory: func() DeviceClient { return dev },
		DeviceWatcher: MockDeviceWatcher{},
		Log:           logrus.StandardLogger(),
	})
	assert.NoError(t, err)

	status := fs.Rmdir("dir", newContext(1, 2, 3))
	assertStatusOk(t, status)
}

func TestRmdir_Error(t *testing.T) {
	dev := &delegateDeviceClient{
		runCommand: func(cmd string, args []string) (string, error) {
			if cmd == "rmdir" {
				return fmt.Sprintf("rmdir failed for %s, Read-only file system", args[0]), nil
			}
			t.Fatal("invalid command:", cmd, args)
			return "", nil
		},
	}
	fs, err := NewAdbFileSystem(Config{
		Mountpoint:    "",
		ClientFactory: func() DeviceClient { return dev },
		DeviceWatcher: MockDeviceWatcher{},
		Log:           logrus.StandardLogger(),
	})
	assert.NoError(t, err)

	status := fs.Rmdir("dir", newContext(1, 2, 3))
	assert.Equal(t, fuse.EINVAL, status)
}

func TestUnlink_Success(t *testing.T) {
	dev := &delegateDeviceClient{
		runCommand: func(cmd string, args []string) (string, error) {
			if cmd == "rm" && args[0] == "/file.txt" {
				return "", nil
			}
			t.Fatal("invalid command:", cmd, args)
			return "", nil
		},
	}
	fs, err := NewAdbFileSystem(Config{
		Mountpoint:    "",
		ClientFactory: func() DeviceClient { return dev },
		DeviceWatcher: MockDeviceWatcher{},
		Log:           logrus.StandardLogger(),
	})
	assert.NoError(t, err)

	status := fs.Unlink("file.txt", newContext(1, 2, 3))
	assertStatusOk(t, status)
}

func TestUnlink_Error(t *testing.T) {
	dev := &delegateDeviceClient{
		runCommand: func(cmd string, args []string) (string, error) {
			if cmd == "rm" {
				return fmt.Sprintf("rm failed for %s, Read-only file system", args[0]), nil
			}
			t.Fatal("invalid command:", cmd, args)
			return "", nil
		},
	}
	fs, err := NewAdbFileSystem(Config{
		Mountpoint:    "",
		ClientFactory: func() DeviceClient { return dev },
		DeviceWatcher: MockDeviceWatcher{},
		Log:           logrus.StandardLogger(),
	})
	assert.NoError(t, err)

	status := fs.Unlink("file.txt", newContext(1, 2, 3))
	assert.Equal(t, fuse.EACCES, status)
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
