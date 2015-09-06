package fs

import (
	"testing"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/stretchr/testify/assert"
)

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
