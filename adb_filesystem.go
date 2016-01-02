// TODO: Implement better file read.
package adbfs

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
	"github.com/zach-klippenstein/adbfs/internal/cli"
	"github.com/zach-klippenstein/goadb"
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

	// Maximum number of concurrent connections for short-lived connections (does not restrict
	// the number of concurrently open files).
	// Values <1 are treated as 1.
	ConnectionPoolSize int
}

type DeviceClientFactory func() DeviceClient

var _ pathfs.FileSystem = &AdbFileSystem{}

func NewAdbFileSystem(config Config) (pathfs.FileSystem, error) {
	if config.ConnectionPoolSize < 1 {
		config.ConnectionPoolSize = 1
	}
	cli.Log.Infoln("connection pool size:", config.ConnectionPoolSize)

	config.DeviceRoot = strings.TrimSuffix(config.DeviceRoot, "/")
	cli.Log.Infoln("device root:", config.DeviceRoot)

	clientPool := make(chan DeviceClient, config.ConnectionPoolSize)
	clientPool <- config.ClientFactory()

	fs := &AdbFileSystem{
		config:             config,
		quickUseClientPool: clientPool,
	}
	if err := fs.initialize(); err != nil {
		return nil, err
	}

	return fs, nil
}

func (fs *AdbFileSystem) initialize() error {
	logEntry := StartOperation("Initialize", "")
	defer logEntry.FinishOperation()

	if fs.config.DeviceRoot != "" {
		// The mountpoint can't report itself as a symlink (it couldn't have any meaningful target).
		device := fs.getQuickUseClient()
		defer fs.recycleQuickUseClient(device)

		target, _, err := readLinkRecursively(device, fs.config.DeviceRoot, logEntry)
		if err != nil {
			logEntry.ErrorMsg(err, "reading link")
			return err
		}

		logEntry.Result("resolved device root %s ➜ %s", fs.config.DeviceRoot, target)
		fs.config.DeviceRoot = target
	}

	return nil
}

func readLinkRecursively(device DeviceClient, path string, logEntry *LogEntry) (string, *goadb.DirEntry, error) {
	var result bytes.Buffer
	currentDepth := 0

	fmt.Fprintf(&result, "attempting to resolve %s if it's a symlink\n", path)

	entry, err := device.Stat(path, logEntry)
	if err != nil {
		return "", nil, err
	}

	for entry.Mode&os.ModeSymlink == os.ModeSymlink {
		if currentDepth > MaxLinkResolveDepth {
			return "", nil, ErrLinkTooDeep
		}
		currentDepth++

		fmt.Fprintln(&result, path)
		path, err = readLink(device, path)
		if err != nil {
			return "", nil, util.WrapErrf(err, "reading link: %s", result.String())
		}

		fmt.Fprintln(&result, " ➜", path)
		entry, err = device.Stat(path, logEntry)
		if err != nil {
			return "", nil, util.WrapErrf(err, "stating %s: %s", path, result.String())
		}
	}

	return path, entry, nil
}

func (fs *AdbFileSystem) String() string {
	return fmt.Sprintf("AdbFileSystem@%s", fs.config.Mountpoint)
}

func (fs *AdbFileSystem) StatFs(name string) *fuse.StatfsOut {
	name = fs.convertClientPathToDevicePath(name)
	logEntry := StartOperation("StatFs", name)
	defer logEntry.SuppressFinishOperation()

	device := fs.getQuickUseClient()
	defer fs.recycleQuickUseClient(device)

	name, _, err := readLinkRecursively(device, name, logEntry)
	if err != nil {
		logEntry.Error(err)
		return nil
	}

	output, err := device.RunCommand("stat", "-f", name)
	if err != nil {
		logEntry.ErrorMsg(err, "running statfs command")
		return nil
	}

	statfs, err := parseStatfs(output)
	if err != nil {
		logEntry.ErrorMsg(err, "invalid stat command output:%v\n%s", err, output)
		return nil
	}
	logEntry.Result("%+v", *statfs)
	return statfs
}

func parseStatfs(output string) (stat *fuse.StatfsOut, err error) {
	if output == "" {
		return nil, errors.New("no output")
	}

	scanner := bufio.NewScanner(strings.NewReader(output))
	scanner.Split(bufio.ScanWords)

	/*
		Sample output:

		File: "/sdcard/Pictures"
		ID: 0        Namelen: 255     Type: UNKNOWN
		Block size: 4096
		Blocks: Total: 1269664    Free: 1209578    Available: 1205482
		Inodes: Total: 327680     Free: 326438
	*/

	stat = new(fuse.StatfsOut)
	var scope, key, value string
	for scanner.Scan() {
		if !strings.HasSuffix(key, ":") {
			// Keys end with :. If the key doesn't end with : yet, it's a multi-word key.
			key += scanner.Text()
			continue
		} else if strings.HasSuffix(scanner.Text(), ":") {
			// Handle the prefix keys (Blocks and Inodes).
			scope = strings.TrimSuffix(key, ":")
			key = scanner.Text()
			continue
		} else {
			value = scanner.Text()
		}

		key = strings.TrimSuffix(key, ":")
		intVal, err := strconv.Atoi(value)
		// Don't return err immediately, we don't always need to parse an int.

		switch key {
		case "Namelen":
			if err == nil {
				stat.NameLen = uint32(intVal)
			}
		case "Blocksize":
			if err == nil {
				stat.Bsize = uint32(intVal)
			}
		case "Total":
			if err == nil {
				switch scope {
				case "Blocks":
					stat.Blocks = uint64(intVal)
				case "Inodes":
					stat.Files = uint64(intVal)
				}
			}
		case "Free":
			if err == nil {
				switch scope {
				case "Blocks":
					stat.Bfree = uint64(intVal)
				case "Inodes":
					stat.Ffree = uint64(intVal)
				}
			}
		case "Available":
			if err == nil {
				switch scope {
				case "Blocks":
					stat.Bavail = uint64(intVal)
				}
			}
		default:
			// Ignore other keys.
			err = nil
		}

		if err != nil {
			return nil, fmt.Errorf("invalid value for %s: %v", key, value)
		}

		key = ""
	}
	if scanner.Err() != nil {
		return nil, err
	}

	return stat, nil
}

func (fs *AdbFileSystem) GetAttr(name string, _ *fuse.Context) (attr *fuse.Attr, status fuse.Status) {
	name = fs.convertClientPathToDevicePath(name)

	logEntry := StartOperation("GetAttr", name)
	// This is a very noisy operation on OSX.
	defer logEntry.SuppressFinishOperation()

	device := fs.getQuickUseClient()
	defer fs.recycleQuickUseClient(device)

	attr = new(fuse.Attr)
	return attr, logEntry.Status(getAttr(name, device, logEntry, attr))
}

// getAttr performs the actual stat call on a client, converts errors to status, and converts
// the DirEntry to a fuse.Attr. It also sets the LogEntry result.
func getAttr(name string, client DeviceClient, logEntry *LogEntry, attr *fuse.Attr) fuse.Status {
	entry, err := client.Stat(name, logEntry)
	if util.HasErrCode(err, util.FileNoExistError) {
		return fuse.ENOENT
	} else if err != nil {
		logEntry.Error(err)
		return fuse.EIO
	}

	asFuseAttr(entry, attr)
	logEntry.Result("entry=%v, attr=%v", entry, attr)
	return fuse.OK
}

func (fs *AdbFileSystem) OpenDir(name string, _ *fuse.Context) ([]fuse.DirEntry, fuse.Status) {
	name = fs.convertClientPathToDevicePath(name)

	logEntry := StartOperation("OpenDir", name)
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

func toFuseStatus(err error, logEntry *LogEntry) (status fuse.Status) {
	switch {
	case err == ErrLinkTooDeep:
		status = fuse.Status(syscall.ELOOP)
	case err == ErrNotALink:
		status = fuse.EINVAL
	case err == ErrNoPermission:
		status = fuse.EPERM
	case util.HasErrCode(err, util.FileNoExistError):
		status = fuse.ENOENT
	default:
		logEntry.Error(err)
		status = fuse.EIO
	}
	return logEntry.Status(status)
}

func (fs *AdbFileSystem) Readlink(name string, context *fuse.Context) (target string, status fuse.Status) {
	name = fs.convertClientPathToDevicePath(name)

	logEntry := StartOperation("Readlink", name)
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

func (fs *AdbFileSystem) Access(name string, mode uint32, context *fuse.Context) fuse.Status {
	name = fs.convertClientPathToDevicePath(name)

	logEntry := StartOperation("Access", name)
	defer logEntry.SuppressFinishOperation()

	device := fs.getQuickUseClient()
	defer fs.recycleQuickUseClient(device)

	// Access is required to resolve symlinks.
	name, _, err := readLinkRecursively(device, name, logEntry)
	if err != nil {
		return toFuseStatus(err, logEntry)
	}

	if mode&fuse.W_OK == fuse.W_OK {
		logEntry.Result("writes not supported")
		return logEntry.Status(fuse.EACCES)
	}

	logEntry.Result("target %s exists, read/execute access permitted", name)
	return logEntry.Status(fuse.OK)
}

func (fs *AdbFileSystem) Create(name string, rawFlags uint32, perms uint32, context *fuse.Context) (nodefs.File, fuse.Status) {
	name = fs.convertClientPathToDevicePath(name)
	logEntry := StartOperation("Create", name)
	defer logEntry.FinishOperation()
	return nil, logEntry.Status(fuse.ENOSYS)
}

func (fs *AdbFileSystem) Open(name string, flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	name = fs.convertClientPathToDevicePath(name)

	logEntry := StartOperation("Open", name)
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
	file = newLoggingFile(file, name)

	return file, logEntry.Status(fuse.OK)
}

// Mkdir creates name on the device with the default permissions.
// perms is ignored.
func (fs *AdbFileSystem) Mkdir(name string, perms uint32, context *fuse.Context) fuse.Status {
	name = fs.convertClientPathToDevicePath(name)

	logEntry := StartOperation("Mkdir", name)
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

	logEntry := StartOperation("Rename", fmt.Sprintf("%s→%s", oldName, newName))
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

	logEntry := StartOperation("Rename", name)
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

	logEntry := StartOperation("Unlink", name)
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

func (fs *AdbFileSystem) Chmod(name string, mode uint32, context *fuse.Context) (code fuse.Status) {
	name = fs.convertClientPathToDevicePath(name)
	logEntry := StartOperation("Chmod", formatArgsListForLog(name, os.FileMode(mode)))
	defer logEntry.FinishOperation()
	return logEntry.Status(fuse.ENOSYS)
}

func (fs *AdbFileSystem) Chown(name string, uid uint32, gid uint32, context *fuse.Context) (code fuse.Status) {
	name = fs.convertClientPathToDevicePath(name)
	logEntry := StartOperation("Chown", fmt.Sprintf("%s uid=%d, gid=%d", name, uid, gid))
	defer logEntry.FinishOperation()
	return logEntry.Status(fuse.ENOSYS)
}

func (fs *AdbFileSystem) GetXAttr(name string, attribute string, context *fuse.Context) (data []byte, code fuse.Status) {
	name = fs.convertClientPathToDevicePath(name)
	logEntry := StartOperation("GetXAttr", formatArgsListForLog(name, attribute))
	defer logEntry.FinishOperation()
	return nil, logEntry.Status(fuse.ENOSYS)
}

func (fs *AdbFileSystem) ListXAttr(name string, context *fuse.Context) (attributes []string, code fuse.Status) {
	name = fs.convertClientPathToDevicePath(name)
	logEntry := StartOperation("ListXAttr", formatArgsListForLog(name))
	defer logEntry.FinishOperation()
	return nil, logEntry.Status(fuse.ENOSYS)
}

func (fs *AdbFileSystem) RemoveXAttr(name string, attr string, context *fuse.Context) fuse.Status {
	name = fs.convertClientPathToDevicePath(name)
	logEntry := StartOperation("RemoveXAttr", formatArgsListForLog(name, attr))
	defer logEntry.FinishOperation()
	return logEntry.Status(fuse.ENOSYS)
}

func (fs *AdbFileSystem) SetXAttr(name string, attr string, data []byte, flags int, context *fuse.Context) fuse.Status {
	name = fs.convertClientPathToDevicePath(name)
	logEntry := StartOperation("SetXAttr", formatArgsListForLog(name, attr, data, flags))
	defer logEntry.FinishOperation()
	return logEntry.Status(fuse.ENOSYS)
}

func (fs *AdbFileSystem) Link(oldName string, newName string, context *fuse.Context) fuse.Status {
	oldName = fs.convertClientPathToDevicePath(oldName)
	newName = fs.convertClientPathToDevicePath(newName)
	logEntry := StartOperation("Link", formatArgsListForLog(oldName, newName))
	defer logEntry.FinishOperation()
	return logEntry.Status(fuse.ENOSYS)
}

func (fs *AdbFileSystem) Symlink(oldName string, newName string, context *fuse.Context) fuse.Status {
	oldName = fs.convertClientPathToDevicePath(oldName)
	newName = fs.convertClientPathToDevicePath(newName)
	logEntry := StartOperation("Symlink", formatArgsListForLog(oldName, newName))
	defer logEntry.FinishOperation()
	return logEntry.Status(fuse.ENOSYS)
}

func (fs *AdbFileSystem) Mknod(name string, mode uint32, dev uint32, context *fuse.Context) fuse.Status {
	name = fs.convertClientPathToDevicePath(name)
	logEntry := StartOperation("Mknod", formatArgsListForLog(name, mode, dev))
	defer logEntry.FinishOperation()
	return logEntry.Status(fuse.ENOSYS)
}

func (fs *AdbFileSystem) Truncate(name string, size uint64, context *fuse.Context) (code fuse.Status) {
	name = fs.convertClientPathToDevicePath(name)
	logEntry := StartOperation("Truncate", formatArgsListForLog(name, size))
	defer logEntry.FinishOperation()
	return logEntry.Status(fuse.ENOSYS)
}

func (fs *AdbFileSystem) Utimens(name string, Atime *time.Time, Mtime *time.Time, context *fuse.Context) (code fuse.Status) {
	name = fs.convertClientPathToDevicePath(name)
	logEntry := StartOperation("Utimens", formatArgsListForLog(name, Atime, Mtime))
	defer logEntry.FinishOperation()
	return logEntry.Status(fuse.ENOSYS)
}

func (fs *AdbFileSystem) OnMount(nodeFs *pathfs.PathNodeFs) {
}

func (fs *AdbFileSystem) OnUnmount() {
}

func (fs *AdbFileSystem) SetDebug(debug bool) {
}

func (fs *AdbFileSystem) getNewClient() (client DeviceClient) {
	client = fs.config.ClientFactory()
	cli.Log.Debug("created device client:", client)
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
