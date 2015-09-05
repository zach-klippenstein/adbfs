// TODO: Implement better file read.
package fs

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
	"github.com/zach-klippenstein/goadb/util"
)

/*
AdbFileSystem is an implementation of fuse.pathfs.FileSystem that exposes the filesystem
on an adb device.

Since all operations go through a single adb server, short-lived connections are throttled by using a
fixed-size pool of device clients. The pool is initially filled by calling Config.ClientFactory.
The pool is not used for long-lived connections such as file transfers, which may be kept open
for arbitrary periods of time by processes using the filesystem.
*/
type AdbFileSystem struct {
	// Default method implementations.
	pathfs.FileSystem

	config Config

	// Client pool for short-lived connections (e.g. listing devices, running commands).
	// Clients for long-lived connections like file transfers should be created as needed.
	quickUseClientPool chan DeviceClient

	// Number of times to retry operations after backing off.
	maxRetries int
}

// Config stores arguments used by AdbFileSystem.
type Config struct {
	// Absolute path to mountpoint, used to resolve symlinks.
	Mountpoint string

	// Used to initially populate the device client pool, and create clients for open files.
	ClientFactory DeviceClientFactory

	Log *logrus.Logger
}

type DeviceClientFactory func() DeviceClient

var _ pathfs.FileSystem = &AdbFileSystem{}

func NewAdbFileSystem(config Config) (fs pathfs.FileSystem, err error) {
	clientPool := make(chan DeviceClient, 1)
	clientPool <- config.ClientFactory()

	if config.Log == nil {
		config.Log = logrus.StandardLogger()
	}

	fs = &AdbFileSystem{
		FileSystem:         pathfs.NewDefaultFileSystem(),
		config:             config,
		quickUseClientPool: clientPool,
	}

	return fs, nil
}

func (fs *AdbFileSystem) String() string {
	return fmt.Sprintf("AdbFileSystem@%s", fs.config.Mountpoint)
}

func (fs *AdbFileSystem) GetAttr(name string, _ *fuse.Context) (attr *fuse.Attr, status fuse.Status) {
	name = convertClientPathToDevicePath(name)
	var err error

	logEntry := StartOperation("GetAttr", name)
	defer logEntry.FinishOperation(fs.config.Log)

	device := fs.getQuickUseClient()
	defer fs.recycleQuickUseClient(device)

	entry, err := device.Stat(name)
	if util.HasErrCode(err, util.FileNoExistError) {
		status = fuse.ENOENT
		logEntry.Result("ENOENT")
	} else if err != nil {
		status = fuse.EIO
		logEntry.Error(err)
	} else {
		attr = asFuseAttr(entry)
		status = fuse.OK
		logEntry.Result("entry=%v, attr=%v", entry, attr)
	}

	return
}

func (fs *AdbFileSystem) OpenDir(name string, _ *fuse.Context) ([]fuse.DirEntry, fuse.Status) {
	name = convertClientPathToDevicePath(name)

	logEntry := StartOperation("OpenDir", name)
	defer logEntry.FinishOperation(fs.config.Log)

	device := fs.getQuickUseClient()
	defer fs.recycleQuickUseClient(device)

	entries, err := device.ListDirEntries(name)
	if err != nil {
		logEntry.ErrorMsg(err, "getting directory list")
		return nil, fuse.EIO
	}

	result, err := asFuseDirEntries(entries)
	if err != nil {
		logEntry.ErrorMsg(err, "reading dir entries")
		return nil, fuse.EIO
	}

	return result, fuse.OK
}

func (fs *AdbFileSystem) Readlink(name string, context *fuse.Context) (target string, code fuse.Status) {
	name = convertClientPathToDevicePath(name)

	logEntry := StartOperation("Readlink", name)
	defer logEntry.FinishOperation(fs.config.Log)

	device := fs.getQuickUseClient()
	defer fs.recycleQuickUseClient(device)

	// The sync protocol doesn't provide a way to read links.
	// For some reason OSX doesn't want to follow recursive symlinks, so just resolve
	// all symlinks all the way as a workaround.
	result, err := device.RunCommand("readlink", "-f", name)
	if err != nil {
		logEntry.ErrorMsg(err, "reading link")
		return "", fuse.EIO
	}
	result = strings.TrimSuffix(result, "\r\n")

	// Translate absolute links as relative to this mountpoint.
	if strings.HasPrefix(result, "/") {
		result = filepath.Join(fs.config.Mountpoint, result)
	}

	if result == "readlink: Invalid argument" {
		logEntry.Error(fmt.Errorf("not a link: readlink returned '%s'", result))
	} else {
		logEntry.Result("%s", result)
	}

	return result, fuse.OK
}

func (fs *AdbFileSystem) Open(name string, flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	name = convertClientPathToDevicePath(name)

	logEntry := StartOperation("Open", name)
	defer logEntry.FinishOperation(fs.config.Log)

	// The client used to access this file will be used for an indeterminate time, so we don't want to use
	// a client from the pool.

	client := fs.getNewClient()

	// TODO: Temporary dev implementation: read entire file into memory.
	stream, err := client.OpenRead(name)
	if err != nil {
		logEntry.ErrorMsg(err, "opening file stream on device")
		return nil, fuse.EIO
	}
	defer stream.Close()

	data, err := ioutil.ReadAll(stream)
	if err != nil {
		logEntry.ErrorMsg(err, "reading data from file")
		return nil, fuse.EIO
	}

	logEntry.Result("read %d bytes", len(data))

	file = nodefs.NewDataFile(data)
	file = newLoggingFile(file, fs.config.Log)

	return file, fuse.OK
}

func (fs *AdbFileSystem) getNewClient() (client DeviceClient) {
	client = fs.config.ClientFactory()
	fs.config.Log.Debug("created device client:", client)
	return
}

func (fs *AdbFileSystem) getQuickUseClient() DeviceClient {
	return <-fs.quickUseClientPool
}

func (fs *AdbFileSystem) recycleQuickUseClient(client DeviceClient) {
	fs.quickUseClientPool <- client
}

func convertClientPathToDevicePath(name string) string {
	return "/" + name
}
