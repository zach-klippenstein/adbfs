package adbfs

import (
	"io"

	"github.com/zach-klippenstein/goadb"
)

// DeviceClient wraps goadb.DeviceClient for testing.
type DeviceClient interface {
	OpenRead(path string, log *LogEntry) (io.ReadCloser, error)
	Stat(path string, log *LogEntry) (*goadb.DirEntry, error)
	ListDirEntries(path string, log *LogEntry) ([]*goadb.DirEntry, error)

	RunCommand(cmd string, args ...string) (string, error)
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
