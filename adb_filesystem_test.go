package adbfs

import (
	"fmt"
	"os"
	"testing"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/stretchr/testify/assert"
	"github.com/zach-klippenstein/goadb"
	"github.com/zach-klippenstein/goadb/util"
)

func TestInitializeWithRecursiveRoot(t *testing.T) {
	// Sets up a fake filesystem that looks like:
	// /sdcard -> /mnt/sdcard -> /mnt/dev0
	dev := &delegateDeviceClient{
		stat: func(path string) (*adb.DirEntry, error) {
			switch path {
			case "/sdcard":
				return &adb.DirEntry{Mode: os.ModeSymlink}, nil
			case "/mnt/sdcard":
				return &adb.DirEntry{Mode: os.ModeSymlink}, nil
			case "/mnt/dev0":
				return &adb.DirEntry{Mode: os.ModeDir, Size: 42}, nil
			default:
				return nil, util.Errorf(util.FileNoExistError, "invalid path: %q", path)
			}
		},
		runCommand: func(cmd string, args []string) (string, error) {
			switch args[0] {
			case "/sdcard":
				return "/mnt/sdcard", nil
			case "/mnt/sdcard":
				return "/mnt/dev0", nil
			default:
				return "", util.Errorf(util.FileNoExistError, "invalid path: %q %q", cmd, args)
			}
		},
	}
	fs, err := NewAdbFileSystem(Config{
		DeviceRoot:    "/sdcard",
		ClientFactory: func() DeviceClient { return dev },
	})
	assert.NoError(t, err)

	attr, status := fs.GetAttr("/", newContext())
	assertStatusOk(t, status)
	assert.Equal(t, 42, int(attr.Size))
}

func TestInitializeWithRetries(t *testing.T) {
	// TODO write this

	// Sets up a fake filesystem that looks like:
	// /sdcard -> /mnt/sdcard -> /mnt/dev0
	dev := &delegateDeviceClient{
		stat: func(path string) (*adb.DirEntry, error) {
			switch path {
			case "/sdcard":
				return &adb.DirEntry{Mode: os.ModeSymlink}, nil
			default:
				return nil, util.Errorf(util.FileNoExistError, "invalid path: %q", path)
			}
		},
		runCommand: func(cmd string, args []string) (string, error) {
			// TODO ??
			switch args[0] {
			case "/sdcard":
				return "", util.Errorf(util.FileNoExistError, "sorry, try again")
			default:
				panic("invalid path: " + args[0])
			}
		},
	}
	fs, err := NewAdbFileSystem(Config{
		DeviceRoot:    "/sdcard",
		ClientFactory: func() DeviceClient { return dev },
	})
	assert.NoError(t, err)



	// Make sure this blocks until the initialize completes.

	attr, status := fs.GetAttr("/", newContext())
	assertStatusOk(t, status)
	assert.Equal(t, 42, int(attr.Size))
}

func TestGetAttr_Root(t *testing.T) {
	dev := &delegateDeviceClient{
		stat: statFiles(&adb.DirEntry{
			Name: "/",
			Mode: os.ModeDir | 0755,
		}),
	}
	fs, err := NewAdbFileSystem(Config{
		Mountpoint:    "",
		ClientFactory: func() DeviceClient { return dev },
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
			stat: statFiles(&adb.DirEntry{
				Name: root.DevicePath,
				Mode: os.ModeDir | 0755,
			}),
		}
		fs, err := NewAdbFileSystem(Config{
			Mountpoint: "",
			ClientFactory: func() DeviceClient {
				return dev
			},
			DeviceRoot: root.DeviceRoot,
		})
		assert.NoError(t, err)

		_, status := fs.GetAttr(root.RequestedPath, newContext())
		assert.Equal(t, fuse.OK, status, "%v", root)
	}
}

func TestGetAttr_CustomDeviceRootSymlink(t *testing.T) {
	dev := &delegateDeviceClient{
		stat: statFiles(&adb.DirEntry{
			Name: "/0",
			Mode: os.ModeSymlink,
		}, &adb.DirEntry{
			Name: "/1",
			Mode: os.ModeDir,
		}),
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
		Mountpoint: "",
		ClientFactory: func() DeviceClient {
			return dev
		},
		DeviceRoot: "/0",
	})
	assert.NoError(t, err)

	entry, status := fs.GetAttr("", newContext())
	assertStatusOk(t, status)
	assert.False(t, entry.IsSymlink())
}

func TestReadLinkRecursively_Success(t *testing.T) {
	dev := &delegateDeviceClient{
		stat: statFiles(&adb.DirEntry{
			Name: "/0",
			Mode: os.ModeSymlink,
		}, &adb.DirEntry{
			Name: "/1",
			Mode: os.ModeSymlink,
		}, &adb.DirEntry{
			Name: "/2",
			Mode: os.ModeDir,
		}),
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
		stat: statFiles(&adb.DirEntry{
			Name: "/0",
			Mode: os.ModeSymlink,
		}),
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
		stat: statFiles(&adb.DirEntry{
			Name: "/version.txt",
			Size: 42,
			Mode: 0444,
		}),
	}
	fs, err := NewAdbFileSystem(Config{
		Mountpoint: "",
		ClientFactory: func() DeviceClient {
			return dev
		},
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
		Mountpoint: "/foo/bar",
		ClientFactory: func() DeviceClient {
			return dev
		},
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
		Mountpoint: "/foo/bar",
		ClientFactory: func() DeviceClient {
			return dev
		},
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
		Mountpoint: "/foo/bar",
		ClientFactory: func() DeviceClient {
			return dev
		},
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
		Mountpoint: "/foo/bar",
		ClientFactory: func() DeviceClient {
			return dev
		},
	})
	assert.NoError(t, err)

	_, status := fs.Readlink("version_link.txt", newContext())
	assert.Equal(t, fuse.EACCES, status)
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
		Mountpoint: "",
		ClientFactory: func() DeviceClient {
			return dev
		},
	})
	assert.NoError(t, err)

	status := fs.Mkdir("newdir", 0, newContext())
	assertStatusOk(t, status)
}

func TestMkdir_ReadOnlyFs(t *testing.T) {
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
		Mountpoint: "",
		ClientFactory: func() DeviceClient {
			return dev
		},
		ReadOnly: true,
	})
	assert.NoError(t, err)

	status := fs.Mkdir("newdir", 0, newContext())
	assert.Equal(t, fuse.EPERM, status)
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
		Mountpoint: "",
		ClientFactory: func() DeviceClient {
			return dev
		},
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
		Mountpoint: "",
		ClientFactory: func() DeviceClient {
			return dev
		},
	})
	assert.NoError(t, err)

	status := fs.Rename("old", "new", newContext())
	assertStatusOk(t, status)
}

func TestRename_ReadOnlyFs(t *testing.T) {
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
		Mountpoint: "",
		ClientFactory: func() DeviceClient {
			return dev
		},
		ReadOnly: true,
	})
	assert.NoError(t, err)

	status := fs.Rename("old", "new", newContext())
	assert.Equal(t, fuse.EPERM, status)
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
		Mountpoint: "",
		ClientFactory: func() DeviceClient {
			return dev
		},
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
		Mountpoint: "",
		ClientFactory: func() DeviceClient {
			return dev
		},
	})
	assert.NoError(t, err)

	status := fs.Rmdir("dir", newContext())
	assertStatusOk(t, status)
}

func TestRmdir_ReadOnlyFs(t *testing.T) {
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
		Mountpoint: "",
		ClientFactory: func() DeviceClient {
			return dev
		},
		ReadOnly: true,
	})
	assert.NoError(t, err)

	status := fs.Rmdir("dir", newContext())
	assert.Equal(t, fuse.EPERM, status)
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
		Mountpoint: "",
		ClientFactory: func() DeviceClient {
			return dev
		},
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
		Mountpoint: "",
		ClientFactory: func() DeviceClient {
			return dev
		},
	})
	assert.NoError(t, err)

	status := fs.Unlink("file.txt", newContext())
	assertStatusOk(t, status)
}

func TestUnlink_ReadOnlyFs(t *testing.T) {
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
		Mountpoint: "",
		ClientFactory: func() DeviceClient {
			return dev
		},
		ReadOnly: true,
	})
	assert.NoError(t, err)

	status := fs.Unlink("file.txt", newContext())
	assert.Equal(t, fuse.EPERM, status)
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
		Mountpoint: "",
		ClientFactory: func() DeviceClient {
			return dev
		},
	})
	assert.NoError(t, err)

	status := fs.Unlink("file.txt", newContext())
	assert.Equal(t, fuse.EACCES, status)
}

func TestCreateFile_ExistSuccess(t *testing.T) {
	dev := &delegateDeviceClient{
		stat: statFiles(&adb.DirEntry{
			Name: "/file",
			Size: 512,
			Mode: 0600,
		}),
		openRead: openReadString("foobar"),
	}
	fs, err := NewAdbFileSystem(Config{
		Mountpoint: "",
		ClientFactory: func() DeviceClient {
			return dev
		},
	})
	assert.NoError(t, err)
	afs := fs.(*AdbFileSystem)

	file, err := afs.createFile("/file", O_RDONLY, DontSetPerms, &LogEntry{})
	assert.NoError(t, err)

	adbFile := getAdbFile(file)
	assert.NotNil(t, adbFile)
	assert.Equal(t, "/file", adbFile.FileBuffer.Path)
	assert.True(t, adbFile.Flags.CanRead())
	assert.Equal(t, "-rw-------", adbFile.FileBuffer.Perms.String())
	assert.True(t, adbFile.Flags.CanRead())
	assert.False(t, adbFile.Flags.CanWrite())
}

func TestCreateFile_NoExistCreateSuccess(t *testing.T) {
	dev := &delegateDeviceClient{
		stat:      statFiles(),
		openWrite: openWriteNoop(),
	}
	fs, err := NewAdbFileSystem(Config{
		Mountpoint: "",
		ClientFactory: func() DeviceClient {
			return dev
		},
	})
	assert.NoError(t, err)
	afs := fs.(*AdbFileSystem)

	file, err := afs.createFile("/file", O_RDWR|O_CREATE, DontSetPerms, &LogEntry{})
	assert.NoError(t, err)

	adbFile := getAdbFile(file)
	assert.NotNil(t, adbFile)
	assert.Equal(t, "/file", adbFile.FileBuffer.Path)
	assert.Equal(t, "-rw-rw-r--", adbFile.FileBuffer.Perms.String())
	assert.True(t, adbFile.Flags.CanRead())
	assert.True(t, adbFile.Flags.CanWrite())
}

func TestCreateFile_ReadOnlyFs(t *testing.T) {
	dev := &delegateDeviceClient{
		stat:      statFiles(),
		openWrite: openWriteNoop(),
	}
	fs, err := NewAdbFileSystem(Config{
		Mountpoint: "",
		ClientFactory: func() DeviceClient {
			return dev
		},
		ReadOnly: true,
	})
	assert.NoError(t, err)
	afs := fs.(*AdbFileSystem)

	for _, flags := range []FileOpenFlags{
		O_RDWR,
		O_WRONLY,
		O_CREATE,
		O_TRUNC,
		O_APPEND,
	} {
		_, err := afs.createFile("file", flags, 0644, &LogEntry{})
		assert.Equal(t, ErrNotPermitted, err)
	}
}

func TestOpen(t *testing.T) {
	// Open is just a thin wrapper around createFile, so this is just a smoke test.

	dev := &delegateDeviceClient{
		stat: statFiles(&adb.DirEntry{
			Name: "/file",
			Size: 512,
			Mode: 0600,
		}),
		openRead: openReadString("foobar"),
	}
	fs, err := NewAdbFileSystem(Config{
		Mountpoint: "",
		ClientFactory: func() DeviceClient {
			return dev
		},
	})
	assert.NoError(t, err)

	file, status := fs.Open("/file", uint32(O_RDONLY), newContext())
	assertStatusOk(t, status)

	adbFile := getAdbFile(file)
	assert.NotNil(t, adbFile)
	assert.Equal(t, "/file", adbFile.FileBuffer.Path)
	assert.True(t, adbFile.Flags.CanRead())
	assert.Equal(t, "-rw-------", adbFile.FileBuffer.Perms.String())
	assert.True(t, adbFile.Flags.CanRead())
	assert.False(t, adbFile.Flags.CanWrite())
}

func TestCreate_NoWriteFlag(t *testing.T) {
	// Create is just a thin wrapper around createFile, so this is just a smoke test.

	dev := &delegateDeviceClient{
		stat:      statFiles(),
		openWrite: openWriteNoop(),
	}
	fs, err := NewAdbFileSystem(Config{
		Mountpoint: "",
		ClientFactory: func() DeviceClient {
			return dev
		},
	})
	assert.NoError(t, err)

	file, status := fs.Create("file", uint32(os.O_RDONLY), 0644, newContext())
	assertStatusOk(t, status)

	adbFile := getAdbFile(file)
	assert.NotNil(t, adbFile)
	assert.Equal(t, "/file", adbFile.FileBuffer.Path)
	assert.Equal(t, "-rw-r--r--", adbFile.FileBuffer.Perms.String())
	assert.False(t, adbFile.Flags.CanRead())
	assert.True(t, adbFile.Flags.CanWrite())
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
