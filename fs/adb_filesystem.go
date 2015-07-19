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

// AdbFileSystem is an implementation of fuse.pathfs.FileSystem that wraps a
// goadb.DeviceClient.
type AdbFileSystem struct {
	config Config

	// Default method implementations.
	pathfs.FileSystem

	// Client pool for short-lived connections (e.g. listing devices, running commands).
	// Clients for long-lived connections like file transfers should be created as needed.
	quickUseClientPool chan DeviceClient

	// Number of times to retry operations after backing off.
	maxRetries int
}

type Config struct {
	// Absolute path to mountpoint, used to resolve symlinks.
	Mountpoint string

	// Used to initially populate the pool, and create clients for open files.
	ClientFactory func() DeviceClient

	Log *logrus.Logger
}

var _ pathfs.FileSystem = &AdbFileSystem{}

func NewAdbFileSystem(config Config) (fs pathfs.FileSystem, err error) {
	clientPool := make(chan DeviceClient, 1)
	clientPool <- config.ClientFactory()

	if config.Log == nil {
		config.Log = logrus.StandardLogger()
	}

	fs = &AdbFileSystem{
		config:             config,
		FileSystem:         pathfs.NewDefaultFileSystem(),
		quickUseClientPool: clientPool,
	}

	return fs, nil
}

func (fs *AdbFileSystem) String() string {
	return fmt.Sprintf("AdbFileSystem@%s", fs.config.Mountpoint)
}

func (fs *AdbFileSystem) logEntry(op string, path string) *logrus.Entry {
	return fs.config.Log.WithFields(logrus.Fields{
		"operation": op,
		"path":      path,
	})
}

func (fs *AdbFileSystem) logStart(op string, path string) {
	fs.logEntry(op, path).Debug()
}

func (fs *AdbFileSystem) logError(op string, msg string, path string, err error) {
	fs.logEntry(op, path).Errorf("%s: %+v", msg, err)
}

func (fs *AdbFileSystem) GetAttr(name string, _ *fuse.Context) (attr *fuse.Attr, status fuse.Status) {
	name = PrependSlash(name)
	var err error

	fs.logStart("GetAttr", name)

	device := fs.getQuickUseClient()
	defer fs.recycleQuickUseClient(device)

	entry, err := device.Stat(name)
	if util.HasErrCode(err, util.FileNoExistError) {
		status = fuse.ENOENT
	} else if err != nil {
		fs.logError("GetAttr", "", name, err)
		status = fuse.EIO
	} else {
		attr = NewAttr(entry)
		status = fuse.OK
	}

	fs.logEntry("GetAttr", "name").WithFields(logrus.Fields{
		"entry": entry,
		"attr":  attr,
	}).Debug()

	return
}

func (fs *AdbFileSystem) OpenDir(name string, _ *fuse.Context) ([]fuse.DirEntry, fuse.Status) {
	name = PrependSlash(name)

	fs.logStart("OpenDir", name)

	device := fs.getQuickUseClient()
	defer fs.recycleQuickUseClient(device)

	entries, err := device.ListDirEntries(name)
	if err != nil {
		fs.logError("OpenDir", "getting directory list", name, err)
		return nil, fuse.EIO
	}

	result, err := CollectDirEntries(entries)
	if err != nil {
		fs.logError("OpenDir", "reading dir entries", name, err)
		return nil, fuse.EIO
	}

	return result, fuse.OK
}

func (fs *AdbFileSystem) Readlink(name string, context *fuse.Context) (target string, code fuse.Status) {
	name = PrependSlash(name)

	fs.logStart("Readlink", name)

	device := fs.getQuickUseClient()
	defer fs.recycleQuickUseClient(device)

	// The sync protocol doesn't provide a way to read links.
	// For some reason OSX doesn't want to follow recursive symlinks, so just resolve
	// all symlinks all the way as a workaround.
	result, err := device.RunCommand("readlink", "-f", name)
	if err != nil {
		fs.logError("Readlink", "reading link", name, err)
		return "", fuse.EIO
	}
	result = strings.TrimSuffix(result, "\r\n")

	// Translate absolute links as relative to this mountpoint.
	if strings.HasPrefix(result, "/") {
		result = filepath.Join(fs.config.Mountpoint, result)
	}

	if result == "readlink: Invalid argument" {
		fs.logEntry("Readlink", name).Errorf("not a link: readlink returned '%s'", result)
	}

	return result, fuse.OK
}

func (fs *AdbFileSystem) Open(name string, flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	name = PrependSlash(name)

	fs.logStart("Open", name)

	// The client used to access this file will be used for an indeterminate time, so we don't want to use
	// a client from the pool.

	client := fs.getNewClient()

	// TODO: Temporary dev implementation: read entire file into memory.
	stream, err := client.OpenRead(name)
	if err != nil {
		fs.logError("Open", "opening file stream on device", name, err)
		return nil, fuse.EIO
	}
	defer stream.Close()

	fs.logEntry("Open", name).Debug("reading entire file into memory")

	data, err := ioutil.ReadAll(stream)
	if err != nil {
		fs.logError("Open", "reading data from file", name, err)
		return nil, fuse.EIO
	}

	fs.logEntry("Open", name).Debugf("read %d bytes", len(data))

	file = nodefs.NewDataFile(data)
	file = NewLoggingFile(file, fs.config.Log)

	return file, fuse.OK
}

func (fs *AdbFileSystem) getNewClient() (client DeviceClient) {
	fs.config.Log.Debug("creating new client…")
	client = fs.config.ClientFactory()
	fs.config.Log.Debugln("created client:", client)
	return
}

func (fs *AdbFileSystem) getQuickUseClient() (client DeviceClient) {
	fs.config.Log.Debug("retrieving client…")
	client = <-fs.quickUseClientPool
	fs.config.Log.Debugln("client retrieved:", client)
	return
}

func (fs *AdbFileSystem) recycleQuickUseClient(client DeviceClient) {
	fs.config.Log.Debugln("recycling client:", client)
	fs.quickUseClientPool <- client
}
