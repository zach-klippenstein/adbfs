package adbfs

import (
	"io"
	"testing"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/stretchr/testify/assert"
	"github.com/zach-klippenstein/goadb"
)

type delegateDeviceClient struct {
	openRead       func(path string, log *LogEntry) (io.ReadCloser, error)
	stat           func(path string, log *LogEntry) (*goadb.DirEntry, error)
	listDirEntries func(path string, log *LogEntry) ([]*goadb.DirEntry, error)
	readLink       func(path, rootPath string, log *LogEntry) (string, error, fuse.Status)
}

func (c *delegateDeviceClient) OpenRead(path string, log *LogEntry) (io.ReadCloser, error) {
	return c.openRead(path, log)
}

func (c *delegateDeviceClient) Stat(path string, log *LogEntry) (*goadb.DirEntry, error) {
	return c.stat(path, log)
}

func (c *delegateDeviceClient) ListDirEntries(path string, log *LogEntry) ([]*goadb.DirEntry, error) {
	return c.listDirEntries(path, log)
}

func (c *delegateDeviceClient) ReadLink(path, rootPath string, log *LogEntry) (string, error, fuse.Status) {
	return c.readLink(path, rootPath, log)
}

func TestReadLinkFromDevice_AbsoluteTarget(t *testing.T) {
	target, err, status := readLinkFromDevice("version_link.txt", "/foo/bar",
		func(cmd string, args ...string) (string, error) {
			if cmd == "readlink" && args[0] == "-f" && args[1] == "version_link.txt" {
				return "/version.txt\r\n", nil
			}
			t.Fatal("invalid command:", cmd, args)
			return "", nil
		})
	assert.NoError(t, err)
	assertStatusOk(t, status)
	assert.Equal(t, "/foo/bar/version.txt", target)
}

func TestReadLinkFromDevice_RelativeTarget(t *testing.T) {
	target, err, status := readLinkFromDevice("version_link.txt", "/foo/bar",
		func(cmd string, args ...string) (string, error) {
			if cmd == "readlink" && args[0] == "-f" && args[1] == "version_link.txt" {
				return "version.txt\r\n", nil
			}
			t.Fatal("invalid command:", cmd, args)
			return "", nil
		})
	assert.NoError(t, err)
	assertStatusOk(t, status)
	assert.Equal(t, "version.txt", target)
}

func TestReadLinkFromDevice_NotALink(t *testing.T) {
	_, err, status := readLinkFromDevice("version.txt", "",
		func(cmd string, args ...string) (string, error) {
			if cmd == "readlink" && args[0] == "-f" && args[1] == "version.txt" {
				return ReadlinkInvalidArgument, nil
			}
			t.Fatal("invalid command:", cmd, args)
			return "", nil
		})
	assert.Error(t, err)
	assert.Equal(t, fuse.EINVAL, status)
}

func TestReadLinkFromDevice_PermissionDenied(t *testing.T) {
	_, err, status := readLinkFromDevice("version_link.txt", "",
		func(cmd string, args ...string) (string, error) {
			if cmd == "readlink" && args[0] == "-f" && args[1] == "version_link.txt" {
				return ReadlinkPermissionDenied, nil
			}
			t.Fatal("invalid command:", cmd, args)
			return "", nil
		})
	assert.NoError(t, err)
	assert.Equal(t, fuse.EPERM, status)
}
