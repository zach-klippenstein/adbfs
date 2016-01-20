package adbfs

import (
	"io"
	"os"
	"time"

	"github.com/zach-klippenstein/goadb"
	"github.com/zach-klippenstein/goadb/util"
)

// DeviceClient wraps adb.DeviceClient for testing.
type DeviceClient interface {
	OpenRead(path string, log *LogEntry) (io.ReadCloser, error)
	OpenWrite(path string, perms os.FileMode, mtime time.Time, log *LogEntry) (io.WriteCloser, error)
	Stat(path string, log *LogEntry) (*adb.DirEntry, error)
	ListDirEntries(path string, log *LogEntry) ([]*adb.DirEntry, error)

	RunCommand(cmd string, args ...string) (string, error)
}

// goadbDeviceClient is an implementation of DeviceClient that wraps
// a adb.DeviceClient.
// It also detects when any operations return an error indicating that the device was disconnected,
// and calls deviceDisconnectedHandler to make it easier to handle disconnections in one spot.
type goadbDeviceClient struct {
	*adb.DeviceClient
	deviceDisconnectedHandler func()
}

// Error messages returned by the readlink command on Android devices.
// Should these be moved into goadb instead?
const (
	ReadlinkInvalidArgument  = "readlink: Invalid argument"
	ReadlinkPermissionDenied = "readlink: Permission denied"
)

func NewGoadbDeviceClientFactory(server adb.Server, deviceSerial string, deviceDisconnectedHandler func()) DeviceClientFactory {
	deviceDescriptor := adb.DeviceWithSerial(deviceSerial)

	return func() DeviceClient {
		return goadbDeviceClient{
			DeviceClient:              adb.NewDeviceClient(server, deviceDescriptor),
			deviceDisconnectedHandler: deviceDisconnectedHandler,
		}
	}
}

func (c goadbDeviceClient) OpenRead(path string, _ *LogEntry) (io.ReadCloser, error) {
	r, err := c.DeviceClient.OpenRead(path)
	if util.HasErrCode(err, util.DeviceNotFound) {
		return nil, c.handleDeviceNotFound(err)
	}
	return r, err
}

func (c goadbDeviceClient) OpenWrite(path string, mode os.FileMode, mtime time.Time, _ *LogEntry) (io.WriteCloser, error) {
	return c.DeviceClient.OpenWrite(path, mode, mtime)
}

func (c goadbDeviceClient) Stat(path string, _ *LogEntry) (*adb.DirEntry, error) {
	e, err := c.DeviceClient.Stat(path)
	if util.HasErrCode(err, util.DeviceNotFound) {
		return nil, c.handleDeviceNotFound(err)
	}
	return e, err
}

func (c goadbDeviceClient) ListDirEntries(path string, _ *LogEntry) ([]*adb.DirEntry, error) {
	entries, err := c.DeviceClient.ListDirEntries(path)
	if err != nil {
		if util.HasErrCode(err, util.DeviceNotFound) {
			c.handleDeviceNotFound(err)
		}
		return nil, err
	}
	return entries.ReadAll()
}

func (c goadbDeviceClient) handleDeviceNotFound(err error) error {
	if c.deviceDisconnectedHandler != nil {
		c.deviceDisconnectedHandler()
	}
	return err
}
