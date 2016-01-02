// TODO: Implement better file read.
package adbfs

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
	"github.com/zach-klippenstein/goadb/util"
)

// 64 symlinks ought to be deep enough for anybody.
const MaxLinkResolveDepth = 64

var (
	ErrLinkTooDeep  = util.AssertionErrorf("link recursion too deep")
	ErrNoPermission = util.AssertionErrorf("permission denied")
	ErrNotALink     = util.AssertionErrorf("not a link")
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

	// Directory on device to consider root.
	DeviceRoot string

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

	config.DeviceRoot = strings.TrimSuffix(config.DeviceRoot, "/")
	config.Log.Infoln("device root:", config.DeviceRoot)

	clientPool := make(chan DeviceClient, config.ConnectionPoolSize)
	clientPool <- config.ClientFactory()

	fs := &AdbFileSystem{
		FileSystem:         pathfs.NewDefaultFileSystem(),
		config:             config,
		quickUseClientPool: clientPool,
	}
	if err := fs.initialize(); err != nil {
		return nil, err
	}

	return fs, nil
}

func (fs *AdbFileSystem) initialize() error {
	logEntry := StartOperation("Initialize", "", fs.config.Log)
	defer logEntry.FinishOperation()

	if fs.config.DeviceRoot != "" {
		// The mountpoint can't report itself as a symlink (it couldn't have any meaningful target).
		device := fs.getQuickUseClient()
		defer fs.recycleQuickUseClient(device)

		target, err := readLinkRecursively(device, fs.config.DeviceRoot, logEntry)
		if err != nil {
			logEntry.ErrorMsg(err, "reading link")
			return err
		}

		logEntry.Result("resolved device root %s ➜ %s", fs.config.DeviceRoot, target)
		fs.config.DeviceRoot = target
	}

	return nil
}

func readLinkRecursively(device DeviceClient, path string, logEntry *LogEntry) (string, error) {
	var result bytes.Buffer
	currentDepth := 0

	fmt.Fprintf(&result, "attempting to resolve %s if it's a symlink\n", path)

	entry, err := device.Stat(path, logEntry)
	if err != nil {
		return "", err
	}

	for entry.Mode&os.ModeSymlink == os.ModeSymlink {
		if currentDepth > MaxLinkResolveDepth {
			return "", ErrLinkTooDeep
		}
		currentDepth++

		fmt.Fprintln(&result, path)
		path, err = readLink(device, path)
		if err != nil {
			return "", util.WrapErrf(err, "reading link: %s", result.String())
		}

		fmt.Fprintln(&result, " ➜", path)
		entry, err = device.Stat(path, logEntry)
		if err != nil {
			return "", util.WrapErrf(err, "stating %s: %s", path, result.String())
		}
	}

	return path, nil
}

func (fs *AdbFileSystem) String() string {
	return fmt.Sprintf("AdbFileSystem@%s", fs.config.Mountpoint)
}

func (fs *AdbFileSystem) GetAttr(name string, _ *fuse.Context) (attr *fuse.Attr, status fuse.Status) {
	name = fs.convertClientPathToDevicePath(name)
	var err error

	logEntry := StartOperation("GetAttr", name, fs.config.Log)
	// This is a very noisy operation on OSX.
	defer logEntry.SuppressFinishOperation()

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
	name = fs.convertClientPathToDevicePath(name)

	logEntry := StartOperation("OpenDir", name, fs.config.Log)
	defer logEntry.FinishOperation()

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

func (fs *AdbFileSystem) Readlink(name string, context *fuse.Context) (target string, status fuse.Status) {
	name = fs.convertClientPathToDevicePath(name)

	logEntry := StartOperation("Readlink", name, fs.config.Log)
	defer logEntry.FinishOperation()

	device := fs.getQuickUseClient()
	defer fs.recycleQuickUseClient(device)

	target, err := readLink(device, name)
	if err == ErrNotALink {
		status = fuse.EINVAL
		err = nil
	} else if err == ErrNoPermission {
		status = fuse.EPERM
		err = nil
	} else if err != nil {
		status = fuse.EIO
	}
	if err != nil {
		logEntry.Error(err)
	}

	// Translate absolute links as relative to this mountpoint.
	// Don't use path.Abs since we don't want to have platform-specific behavior.
	if strings.HasPrefix(target, "/") {
		target = filepath.Join(fs.config.Mountpoint, target)
	}

	logEntry.Result("%s", target)
	return target, logEntry.Status(status)
}

func readLink(client DeviceClient, path string) (string, error) {
	// The sync protocol doesn't provide a way to read links.
	// Some versions of Android have a readlink command that supports resolving recursively, but
	// others (notably Marshmallow) don't, so don't try to do anything fancy (see issue #14).
	// OSX Finder won't follow recursive symlinks in tree view, but it should resolve them if you
	// open them.
	result, err := client.RunCommand("readlink", path)
	if err != nil {
		return "", err
	}
	result = strings.TrimSuffix(result, "\r\n")

	if result == ReadlinkInvalidArgument {
		return "", ErrNotALink
	} else if result == ReadlinkPermissionDenied {
		return "", ErrNoPermission
	}

	return result, nil
}

func (fs *AdbFileSystem) Open(name string, flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	name = fs.convertClientPathToDevicePath(name)

	logEntry := StartOperation("Open", name, fs.config.Log)
	defer logEntry.FinishOperation()

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
	file = newLoggingFile(file, name, fs.config.Log)

	return file, logEntry.Status(fuse.OK)
}

// Mkdir creates name on the device with the default permissions.
// perms is ignored.
func (fs *AdbFileSystem) Mkdir(name string, perms uint32, context *fuse.Context) fuse.Status {
	name = fs.convertClientPathToDevicePath(name)

	logEntry := StartOperation("Mkdir", name, fs.config.Log)
	defer logEntry.FinishOperation()

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
	oldName = fs.convertClientPathToDevicePath(oldName)
	newName = fs.convertClientPathToDevicePath(newName)

	logEntry := StartOperation("Rename", fmt.Sprintf("%s→%s", oldName, newName), fs.config.Log)
	defer logEntry.FinishOperation()

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
	name = fs.convertClientPathToDevicePath(name)

	logEntry := StartOperation("Rename", name, fs.config.Log)
	defer logEntry.FinishOperation()

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
	name = fs.convertClientPathToDevicePath(name)

	logEntry := StartOperation("Unlink", name, fs.config.Log)
	defer logEntry.FinishOperation()

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

func (fs *AdbFileSystem) convertClientPathToDevicePath(name string) string {
	return path.Join("/", fs.config.DeviceRoot, name)
}
