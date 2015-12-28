package adbfs

import (
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"bytes"

	"github.com/zach-klippenstein/goadb"
	"github.com/zach-klippenstein/goadb/util"
)

type delegateDeviceClient struct {
	openRead       func(path string) (io.ReadCloser, error)
	openWrite      func(path string, mode os.FileMode, mtime time.Time) (io.WriteCloser, error)
	stat           func(path string) (*goadb.DirEntry, error)
	listDirEntries func(path string) ([]*goadb.DirEntry, error)
	runCommand     func(cmd string, args []string) (string, error)
}

func (c *delegateDeviceClient) OpenRead(path string, _ *LogEntry) (io.ReadCloser, error) {
	return c.openRead(path)
}

func (c *delegateDeviceClient) OpenWrite(path string, mode os.FileMode, mtime time.Time, _ *LogEntry) (io.WriteCloser, error) {
	return c.openWrite(path, mode, mtime)
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

func openWriteNoop() func(path string, mode os.FileMode, mtime time.Time) (io.WriteCloser, error) {
	return openWriteTo(nil)
}

func openWriteTo(w *bytes.Buffer) func(path string, mode os.FileMode, mtime time.Time) (io.WriteCloser, error) {
	return func(path string, mode os.FileMode, mtime time.Time) (io.WriteCloser, error) {
		// Simulate how a goadb.OpenRead call works.
		if w != nil {
			w.Reset()
		}

		return noopWriteCloser{
			w:     w,
			Mode:  mode,
			Mtime: mtime,
		}, nil
	}
}

func openWriteError(err error) func(path string, mode os.FileMode, mtime time.Time) (io.WriteCloser, error) {
	return func(path string, mode os.FileMode, mtime time.Time) (io.WriteCloser, error) {
		return nil, err
	}
}

type noopWriteCloser struct {
	w     io.Writer
	Mode  os.FileMode
	Mtime time.Time
}

func (w noopWriteCloser) Write(data []byte) (int, error) {
	if w.w != nil {
		return w.w.Write(data)
	}
	return 0, nil
}

func (noopWriteCloser) Close() error {
	return nil
}
