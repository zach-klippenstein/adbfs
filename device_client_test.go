package adbfs

import (
	"io"
	"io/ioutil"
	"strings"

	"github.com/zach-klippenstein/goadb"
	"github.com/zach-klippenstein/goadb/util"
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

func statFiles(entries ...*goadb.DirEntry) func(string) (*goadb.DirEntry, error) {
	return func(path string) (*goadb.DirEntry, error) {
		for _, entry := range entries {
			if entry.Name == path {
				return entry, nil
			}
		}
		return nil, util.Errorf(util.FileNoExistError, "%s", path)
	}
}

func openReadString(str string) func(path string) (io.ReadCloser, error) {
	return func(path string) (io.ReadCloser, error) {
		return ioutil.NopCloser(strings.NewReader(str)), nil
	}
}

func openReadError(err error) func(path string) (io.ReadCloser, error) {
	return func(path string) (io.ReadCloser, error) {
		return nil, err
	}
}
