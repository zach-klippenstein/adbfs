package fs

import (
	"io"

	"github.com/zach-klippenstein/goadb"
)

// DeviceClient wraps goadb.DeviceClient for testing.
type DeviceClient interface {
	OpenRead(path string) (io.ReadCloser, error)
	Stat(path string) (*goadb.DirEntry, error)
	ListDirEntries(path string) (DirEntries, error)
	RunCommand(cmd string, args ...string) (string, error)
}

// DirEntries wraps goadb.DirEntries for testing.
type DirEntries interface {
	Next() bool
	Entry() *goadb.DirEntry
	Err() error
	Close() error
}

// goadbDeviceClient is an implementation of DeviceClient that wraps
// a goadb.DeviceClient.
type goadbDeviceClient struct {
	*goadb.DeviceClient
}

func NewGoadbDeviceClientFactory(clientConfig goadb.ClientConfig, deviceSerial string) DeviceClientFactory {
	deviceDescriptor := goadb.DeviceWithSerial(deviceSerial)

	return func() DeviceClient {
		return goadbDeviceClient{goadb.NewDeviceClient(clientConfig, deviceDescriptor)}
	}
}

func (c goadbDeviceClient) ListDirEntries(path string) (DirEntries, error) {
	return c.DeviceClient.ListDirEntries(path)
}
