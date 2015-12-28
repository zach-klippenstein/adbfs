package adbfs

import (
	"io"

	"github.com/zach-klippenstein/goadb"
)

type delegateDeviceClient struct {
	openRead       func(path string) (io.ReadCloser, error)
	stat           func(path string) (*goadb.DirEntry, error)
	listDirEntries func(path string) ([]*goadb.DirEntry, error)
	runCommand     func(cmd string, args []string) (string, error)
}

func (c *delegateDeviceClient) OpenRead(path string, _ *LogEntry) (io.ReadCloser, error) {
	return c.openRead(path)
}

func (c *delegateDeviceClient) Stat(path string, _ *LogEntry) (*goadb.DirEntry, error) {
	return c.stat(path)
}

func (c *delegateDeviceClient) ListDirEntries(path string, _ *LogEntry) ([]*goadb.DirEntry, error) {
	return c.listDirEntries(path)
}

func (c *delegateDeviceClient) RunCommand(cmd string, args ...string) (string, error) {
	return c.runCommand(cmd, args)
}
