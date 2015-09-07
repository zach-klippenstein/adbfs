package fs

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/zach-klippenstein/goadb"
	"github.com/zach-klippenstein/goadb/util"
)

type DeviceShellRunner func(cmd string, args ...string) (string, error)

// DeviceClient wraps goadb.DeviceClient for testing.
type DeviceClient interface {
	OpenRead(path string, log *LogEntry) (io.ReadCloser, error)
	Stat(path string, log *LogEntry) (*goadb.DirEntry, error)
	ListDirEntries(path string, log *LogEntry) ([]*goadb.DirEntry, error)

	// ReadLink returns the target of a symlink.
	// If the target is relative, resolves it using rootPath.
	ReadLink(path, rootPath string, log *LogEntry) (string, error, fuse.Status)
}

// goadbDeviceClient is an implementation of DeviceClient that wraps
// a goadb.DeviceClient.
type goadbDeviceClient struct {
	*goadb.DeviceClient
}

// Error messages returned by the readlink command on Android devices.
// Should these be moved into goadb instead?
const (
	ReadlinkInvalidArgument  = "readlink: Invalid argument"
	ReadlinkPermissionDenied = "readlink: Permission denied"
)

func NewGoadbDeviceClientFactory(clientConfig goadb.ClientConfig, deviceSerial string) DeviceClientFactory {
	deviceDescriptor := goadb.DeviceWithSerial(deviceSerial)

	return func() DeviceClient {
		return goadbDeviceClient{goadb.NewDeviceClient(clientConfig, deviceDescriptor)}
	}
}

func (c goadbDeviceClient) OpenRead(path string, _ *LogEntry) (io.ReadCloser, error) {
	return c.DeviceClient.OpenRead(path)
}

func (c goadbDeviceClient) Stat(path string, _ *LogEntry) (*goadb.DirEntry, error) {
	return c.DeviceClient.Stat(path)
}

func (c goadbDeviceClient) ListDirEntries(path string, _ *LogEntry) ([]*goadb.DirEntry, error) {
	entries, err := c.DeviceClient.ListDirEntries(path)
	if err != nil {
		return nil, err
	}
	return entries.ReadAll()
}

func (c goadbDeviceClient) ReadLink(path, rootPath string, _ *LogEntry) (string, error, fuse.Status) {
	return readLinkFromDevice(path, rootPath, c.DeviceClient.RunCommand)
}

// readLinkFromDevice uses runner to execute a readlink command and parses the result.
func readLinkFromDevice(path, rootPath string, runner DeviceShellRunner) (string, error, fuse.Status) {
	// The sync protocol doesn't provide a way to read links.
	// OSX doesn't follow recursive symlinks, so just resolve
	// all symlinks all the way as a workaround.
	result, err := runner("readlink", "-f", path)
	if util.HasErrCode(err, util.DeviceNotFound) {
		return "", err, fuse.EIO
	} else if err != nil {
		return "", err, fuse.EIO
	}
	result = strings.TrimSuffix(result, "\r\n")

	// Translate absolute links as relative to this mountpoint.
	// Don't use path.Abs since we don't want to have platform-specific behavior.
	if strings.HasPrefix(result, "/") {
		result = filepath.Join(rootPath, result)
	}

	if result == ReadlinkInvalidArgument {
		return "",
			fmt.Errorf("not a link: readlink returned '%s' reading link: %s", result, path),
			fuse.EINVAL
	} else if result == ReadlinkPermissionDenied {
		return "", nil, fuse.EPERM
	}

	return result, nil, fuse.OK
}
