package fs

import (
	"io"

	"github.com/zach-klippenstein/goadb"
)

type DeviceClient interface {
	OpenRead(path string) (io.ReadCloser, error)
	Stat(path string) (*goadb.DirEntry, error)
	ListDirEntries(path string) (*goadb.DirEntries, error)
	RunCommand(cmd string, args ...string) (string, error)
}

type goadbDeviceClient struct {
	*goadb.DeviceClient
}
