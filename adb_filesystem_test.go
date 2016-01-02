package adbfs

import (
	"fmt"
	"os"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/hanwen/go-fuse/fuse"
	"github.com/stretchr/testify/assert"
	"github.com/zach-klippenstein/goadb"
	"github.com/zach-klippenstein/goadb/util"
)

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
		Log:           logrus.StandardLogger(),
	})
	assert.NoError(t, err)

	attr, status := fs.GetAttr("", newContext())
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

func TestGetAttr_CustomDeviceRoot(t *testing.T) {
	for _, root := range []struct {
		DeviceRoot, DevicePath, RequestedPath string
	}{
		{"", "/", ""},
		{"", "/", "/"},
		{"/", "/", ""},
		{"/", "/", "/"},
		{"/sdcard", "/sdcard", ""},
		{"/sdcard", "/sdcard", "/"},
		{"/sdcard/", "/sdcard", ""},
		{"/sdcard/", "/sdcard", "/"},
	} {
		dev := &delegateDeviceClient{
			stat: func(path string) (*goadb.DirEntry, error) {
				if path == root.DevicePath {
					return &goadb.DirEntry{
						Name: root.DevicePath,
						Size: 0,
						Mode: os.ModeDir | 0755,
					}, nil
				}
				return nil, util.Errorf(util.FileNoExistError, "")
			},
		}
		fs, err := NewAdbFileSystem(Config{
			Mountpoint:    "",
			ClientFactory: func() DeviceClient { return dev },
			Log:           logrus.StandardLogger(),
			DeviceRoot:    root.DeviceRoot,
		})
		assert.NoError(t, err)

		_, status := fs.GetAttr(root.RequestedPath, newContext())
		assert.Equal(t, fuse.OK, status, "%v", root)
	}
}

func TestGetAttr_CustomDeviceRootSymlink(t *testing.T) {
	dev := &delegateDeviceClient{
		stat: func(path string) (*goadb.DirEntry, error) {
			switch path {
			case "/0":
				return &goadb.DirEntry{
					Name: path,
					Mode: os.ModeSymlink,
				}, nil
			case "/1":
				return &goadb.DirEntry{
					Name: path,
					Mode: os.ModeDir,
				}, nil
			default:
				return nil, util.Errorf(util.FileNoExistError, "")
			}
		},
		runCommand: func(cmd string, args []string) (string, error) {
			if cmd != "readlink" {
				t.Fatal("invalid command:", cmd, args)
			}
			switch args[0] {
			case "/0":
				return "/1", nil
			default:
				return "", ErrNotALink
			}
		},
	}
	fs, err := NewAdbFileSystem(Config{
		Mountpoint:    "",
		ClientFactory: func() DeviceClient { return dev },
		Log:           logrus.StandardLogger(),
		DeviceRoot:    "/0",
	})
	assert.NoError(t, err)

	entry, status := fs.GetAttr("", newContext())
	assertStatusOk(t, status)
	assert.False(t, entry.IsSymlink())
}

func TestReadLinkRecursively_Success(t *testing.T) {
	dev := &delegateDeviceClient{
		stat: func(path string) (*goadb.DirEntry, error) {
			switch path {
			case "/0", "/1":
				return &goadb.DirEntry{
					Name: path,
					Mode: os.ModeSymlink,
				}, nil
			case "/2":
				return &goadb.DirEntry{
					Name: path,
					Mode: os.ModeDir,
				}, nil
			default:
				return nil, util.Errorf(util.FileNoExistError, "")
			}
		},
		runCommand: func(cmd string, args []string) (string, error) {
			if cmd != "readlink" {
				t.Fatal("invalid command:", cmd, args)
			}
			switch args[0] {
			case "/0":
				return "/1", nil
			case "/1":
				return "/2", nil
			default:
				return "", ErrNotALink
			}
		},
	}

	target, _, err := readLinkRecursively(dev, "/0", &LogEntry{})
	assert.NoError(t, err)
	assert.Equal(t, "/2", target)
}

func TestReadLinkRecursively_MaxDepth(t *testing.T) {
	dev := &delegateDeviceClient{
		stat: func(path string) (*goadb.DirEntry, error) {
			return &goadb.DirEntry{
				Name: path,
				Mode: os.ModeSymlink,
			}, nil
		},
		runCommand: func(cmd string, args []string) (string, error) {
			if cmd != "readlink" {
				t.Fatal("invalid command:", cmd, args)
			}
			return "/0", nil
		},
	}

	_, _, err := readLinkRecursively(dev, "/0", &LogEntry{})
	assert.Equal(t, ErrLinkTooDeep, err)
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
		Log:           logrus.StandardLogger(),
	})
	assert.NoError(t, err)

	attr, status := fs.GetAttr("version.txt", newContext())
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
		Log:           logrus.StandardLogger(),
	})
	assert.NoError(t, err)

	target, status := fs.Readlink("version_link.txt", newContext())
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
		Log:           logrus.StandardLogger(),
	})
	assert.NoError(t, err)

	target, status := fs.Readlink("version_link.txt", newContext())
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
		Log:           logrus.StandardLogger(),
	})
	assert.NoError(t, err)

	_, status := fs.Readlink("version_link.txt", newContext())
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
		Log:           logrus.StandardLogger(),
	})
	assert.NoError(t, err)

	_, status := fs.Readlink("version_link.txt", newContext())
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
		Log:           logrus.StandardLogger(),
	})
	assert.NoError(t, err)

	status := fs.Mkdir("newdir", 0, newContext())
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
		Log:           logrus.StandardLogger(),
	})
	assert.NoError(t, err)

	status := fs.Mkdir("newdir", 0, newContext())
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
		Log:           logrus.StandardLogger(),
	})
	assert.NoError(t, err)

	status := fs.Rename("old", "new", newContext())
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
		Log:           logrus.StandardLogger(),
	})
	assert.NoError(t, err)

	status := fs.Rename("old", "new", newContext())
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
		Log:           logrus.StandardLogger(),
	})
	assert.NoError(t, err)

	status := fs.Rmdir("dir", newContext())
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
		Log:           logrus.StandardLogger(),
	})
	assert.NoError(t, err)

	status := fs.Rmdir("dir", newContext())
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
		Log:           logrus.StandardLogger(),
	})
	assert.NoError(t, err)

	status := fs.Unlink("file.txt", newContext())
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
		Log:           logrus.StandardLogger(),
	})
	assert.NoError(t, err)

	status := fs.Unlink("file.txt", newContext())
	assert.Equal(t, fuse.EACCES, status)
}

func TestParseStatfs(t *testing.T) {
	_, err := parseStatfs(``)
	assert.EqualError(t, err, "no output")

	stat, err := parseStatfs(`Namelen: a`)
	assert.EqualError(t, err, "invalid value for Namelen: a")

	stat, err = parseStatfs(`  File: "/sdcard/Pictures"
    ID: 0        Namelen: 255     Type: UNKNOWN
Block size: 4096
Blocks: Total: 1269664    Free: 1209578    Available: 1205482
Inodes: Total: 327680     Free: 326438`)
	assert.NoError(t, err)
	assert.Equal(t, fuse.StatfsOut{
		NameLen: 255,
		Bsize:   4096,
		Blocks:  1269664,
		Bfree:   1209578,
		Bavail:  1205482,
		Files:   327680,
		Ffree:   326438,
	}, *stat)
}

func newContext() *fuse.Context {
	return &fuse.Context{
		Owner: fuse.Owner{
			Uid: uint32(1),
			Gid: uint32(2),
		},
		Pid: uint32(3),
	}
}

func assertStatusOk(t *testing.T, status fuse.Status) {
	assert.True(t, status.Ok(), "Expected status to be Ok, was %s", status)
}
