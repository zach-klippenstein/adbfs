// TODO: Implement better file read.
package adbfs

import (
	"errors"
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
}

// Config stores arguments used by AdbFileSystem.
type Config struct {
	// Serial number of the device for which ClientFactory returns clients.
	DeviceSerial string

	// Absolute path to mountpoint, used to resolve symlinks.
	Mountpoint string

	// Used to initially populate the device client pool, and create clients for open files.
	ClientFactory DeviceClientFactory

	Log *logrus.Logger

	// Maximum number of concurrent connections for short-lived connections (does not restrict
	// the number of concurrently open files).
	// Values <1 are treated as 1.
	ConnectionPoolSize int
}

type DeviceClientFactory func() DeviceClient

var _ pathfs.FileSystem = &AdbFileSystem{}

func NewAdbFileSystem(config Config) (pathfs.FileSystem, error) {
	if config.Log == nil {
		panic("no logger specified for filesystem")
	}

	if config.ConnectionPoolSize < 1 {
		config.ConnectionPoolSize = 1
	}
	config.Log.Infoln("connection pool size:", config.ConnectionPoolSize)

	clientPool := make(chan DeviceClient, config.ConnectionPoolSize)
	clientPool <- config.ClientFactory()

	fs := &AdbFileSystem{
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

	entry, err := device.Stat(name, logEntry)
	if util.HasErrCode(err, util.FileNoExistError) {
		return nil, logEntry.Status(fuse.ENOENT)
	} else if err != nil {
		logEntry.Error(err)
		return nil, logEntry.Status(fuse.EIO)
	}

	attr = asFuseAttr(entry)
	logEntry.Result("entry=%v, attr=%v", entry, attr)
	return attr, logEntry.Status(fuse.OK)
}

func (fs *AdbFileSystem) OpenDir(name string, _ *fuse.Context) ([]fuse.DirEntry, fuse.Status) {
	name = convertClientPathToDevicePath(name)

	logEntry := StartOperation("OpenDir", name)
	defer logEntry.FinishOperation(fs.config.Log)

	device := fs.getQuickUseClient()
	defer fs.recycleQuickUseClient(device)

	entries, err := device.ListDirEntries(name, logEntry)
	if err != nil {
		logEntry.ErrorMsg(err, "getting directory list")
		return nil, logEntry.Status(fuse.EIO)
	}

	result := asFuseDirEntries(entries)
	return result, logEntry.Status(fuse.OK)
}

func (fs *AdbFileSystem) Readlink(name string, context *fuse.Context) (target string, code fuse.Status) {
	name = convertClientPathToDevicePath(name)

	logEntry := StartOperation("Readlink", name)
	defer logEntry.FinishOperation(fs.config.Log)

	device := fs.getQuickUseClient()
	defer fs.recycleQuickUseClient(device)

	result, err, status := readLink(device, name, fs.config.Mountpoint)
	if err != nil {
		logEntry.Error(err)
	}

	logEntry.Result("%s", result)
	return result, logEntry.Status(status)
}

func readLink(client DeviceClient, path, rootPath string) (string, error, fuse.Status) {
	// The sync protocol doesn't provide a way to read links.
	// Some versions of Android have a readlink command that supports resolving recursively, but
	// others (notably Marshmallow) don't, so don't try to do anything fancy (see issue #14).
	// OSX Finder won't follow recursive symlinks in tree view, but it should resolve them if you
	// open them.
	result, err := client.RunCommand("readlink", path)
	if err != nil {
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

func (fs *AdbFileSystem) Open(name string, flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	name = convertClientPathToDevicePath(name)

	logEntry := StartOperation("Open", name)
	defer logEntry.FinishOperation(fs.config.Log)

	// The client used to access this file will be used for an indeterminate time, so we don't want to use
	// a client from the pool.

	client := fs.getNewClient()

	// TODO: Temporary dev implementation: read entire file into memory.
	stream, err := client.OpenRead(name, logEntry)
	if err != nil {
		logEntry.ErrorMsg(err, "opening file stream on device")
		return nil, logEntry.Status(fuse.EIO)
	}
	defer stream.Close()

	data, err := ioutil.ReadAll(stream)
	if err != nil {
		logEntry.ErrorMsg(err, "reading data from file")
		return nil, logEntry.Status(fuse.EIO)
	}

	logEntry.Result("read %d bytes", len(data))

	file = nodefs.NewDataFile(data)
	file = newLoggingFile(file, fs.config.Log)

	return file, logEntry.Status(fuse.OK)
}

// Mkdir creates name on the device with the default permissions.
// mode is ignored.
func (fs *AdbFileSystem) Mkdir(name string, mode uint32, context *fuse.Context) fuse.Status {
	name = convertClientPathToDevicePath(name)

	logEntry := StartOperation("Mkdir", name)
	defer logEntry.FinishOperation(fs.config.Log)

	device := fs.getQuickUseClient()
	defer fs.recycleQuickUseClient(device)

	err, status := mkdir(device, name)
	if err != nil {
		logEntry.Error(err)
	}

	return logEntry.Status(status)
}

func mkdir(client DeviceClient, path string) (error, fuse.Status) {
	result, err := client.RunCommand("mkdir", path)
	if err != nil {
		return err, fuse.EIO
	}

	if result != "" {
		result = strings.TrimSuffix(result, "\r\n")
		return errors.New(result), fuse.EACCES
	}

	return nil, fuse.OK
}

func (fs *AdbFileSystem) Rename(oldName, newName string, context *fuse.Context) fuse.Status {
	oldName = convertClientPathToDevicePath(oldName)
	newName = convertClientPathToDevicePath(newName)

	logEntry := StartOperation("Rename", fmt.Sprintf("%s→%s", oldName, newName))
	defer logEntry.FinishOperation(fs.config.Log)

	device := fs.getQuickUseClient()
	defer fs.recycleQuickUseClient(device)

	err, status := rename(device, oldName, newName)
	if err != nil {
		logEntry.Error(err)
	}

	return logEntry.Status(status)
}

func rename(client DeviceClient, oldName, newName string) (error, fuse.Status) {
	result, err := client.RunCommand("mv", oldName, newName)
	if err != nil {
		return err, fuse.EIO
	}

	if result != "" {
		result = strings.TrimSuffix(result, "\r\n")
		return errors.New(result), fuse.EACCES
	}

	return nil, fuse.OK
}

func (fs *AdbFileSystem) Rmdir(name string, context *fuse.Context) fuse.Status {
	name = convertClientPathToDevicePath(name)

	logEntry := StartOperation("Rename", name)
	defer logEntry.FinishOperation(fs.config.Log)

	device := fs.getQuickUseClient()
	defer fs.recycleQuickUseClient(device)

	err, status := rmdir(device, name)
	if err != nil {
		logEntry.Error(err)
	}

	return logEntry.Status(status)
}

func rmdir(client DeviceClient, name string) (error, fuse.Status) {
	result, err := client.RunCommand("rmdir", name)
	if err != nil {
		return err, fuse.EIO
	}

	if result != "" {
		result = strings.TrimSuffix(result, "\r\n")
		return errors.New(result), fuse.EINVAL
	}

	return nil, fuse.OK
}

func (fs *AdbFileSystem) Unlink(name string, context *fuse.Context) fuse.Status {
	name = convertClientPathToDevicePath(name)

	logEntry := StartOperation("Unlink", name)
	defer logEntry.FinishOperation(fs.config.Log)

	device := fs.getQuickUseClient()
	defer fs.recycleQuickUseClient(device)

	err, status := unlink(device, name)
	if err != nil {
		logEntry.Error(err)
	}

	return logEntry.Status(status)
}

func unlink(client DeviceClient, name string) (error, fuse.Status) {
	result, err := client.RunCommand("rm", name)
	if err != nil {
		return err, fuse.EIO
	}

	if result != "" {
		result = strings.TrimSuffix(result, "\r\n")
		return errors.New(result), fuse.EACCES
	}

	return nil, fuse.OK
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
