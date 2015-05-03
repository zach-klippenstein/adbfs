package fs

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
)

var (
	VerboseLogging = false
	ErrorLogging   = true
)

// AdbFileSystem is an implementation of fuse.pathfs.FileSystem that wraps a
// goadb.DeviceClient.
type AdbFileSystem struct {
	// Default method implementations.
	pathfs.FileSystem

	// Absolute path to mountpoint, used to resolve symlinks.
	mountpoint string

	// Used to initially populate the pool, and create clients for open files.
	clientFactory func() DeviceClient

	// Client pool.
	quickUseClientPool chan DeviceClient

	// Number of times to retry operations after backing off.
	maxRetries int
}

var _ pathfs.FileSystem = &AdbFileSystem{}

func NewAdbFileSystem(mountpoint string, clientPoolSize int, clientFactory func() DeviceClient) (fs pathfs.FileSystem, err error) {
	if clientPoolSize < 1 {
		return nil, fmt.Errorf("clientPoolSize must be > 0, was %d", clientPoolSize)
	}

	clientPool := make(chan DeviceClient, clientPoolSize)
	for i := 0; i < clientPoolSize; i++ {
		clientPool <- clientFactory()
	}

	fs = &AdbFileSystem{
		FileSystem:         pathfs.NewDefaultFileSystem(),
		mountpoint:         mountpoint,
		clientFactory:      clientFactory,
		quickUseClientPool: clientPool,
	}

	if VerboseLogging {
		fs = NewLoggingFileSystem(fs)
	}

	return fs, nil
}

func (fs *AdbFileSystem) String() string {
	return fmt.Sprintf("AdbFileSystem@%s", fs.mountpoint)
}

func (fs *AdbFileSystem) GetAttr(name string, _ *fuse.Context) (attr *fuse.Attr, status fuse.Status) {
	name = PrependSlash(name)
	var err error

	device := fs.getQuickUseClient()
	defer fs.recycleQuickUseClient(device)

	entry, err := device.Stat(name)
	if err == os.ErrNotExist {
		status = fuse.ENOENT
	} else if err != nil {
		if ErrorLogging {
			log.Printf("GetAttr: error statting '%s': %+v", name, err)
		}
		status = fuse.EIO
	} else {
		attr = NewAttr(entry)
		status = fuse.OK
	}

	if VerboseLogging {
		log.Printf("\tEntry: %+v\n", entry)
		log.Printf("\t Attr: %+v\n", attr)
	}

	return
}

func (fs *AdbFileSystem) OpenDir(name string, _ *fuse.Context) ([]fuse.DirEntry, fuse.Status) {
	name = PrependSlash(name)

	device := fs.getQuickUseClient()
	defer fs.recycleQuickUseClient(device)

	entries, err := device.ListDirEntries(name)
	if err != nil {
		if ErrorLogging {
			log.Printf("OpenDir: error getting directory list for '%s': %+v", name, err)
		}
		return nil, fuse.EIO
	}

	result, err := CollectDirEntries(entries)
	if err != nil {
		if ErrorLogging {
			log.Printf("OpenDir: error reading dir entries for '%s': %+v", name, err)
		}
		return nil, fuse.EIO
	}

	return result, fuse.OK
}

func (fs *AdbFileSystem) Readlink(name string, context *fuse.Context) (target string, code fuse.Status) {
	name = PrependSlash(name)

	device := fs.getQuickUseClient()
	defer fs.recycleQuickUseClient(device)

	// The sync protocol doesn't provide a way to read links.
	// For some reason OSX doesn't want to follow recursive symlinks, so just resolve
	// all symlinks all the way as a workaround.
	result, err := device.RunCommand("readlink", "-f", name)
	if err != nil {
		if ErrorLogging {
			log.Printf("Readlink: error reading link '%s': %+v", name, err)
		}
		return "", fuse.EIO
	}
	result = strings.TrimSuffix(result, "\r\n")

	// Translate absolute links as relative to this mountpoint.
	if strings.HasPrefix(result, "/") {
		result = filepath.Join(fs.mountpoint, result)
	}

	if result == "readlink: Invalid argument" {
		if ErrorLogging {
			log.Printf("Readlink: '%s' is not a link: readlink returned '%s'", name, result)
		}
	}

	return result, fuse.OK
}

func (fs *AdbFileSystem) Open(name string, flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	name = PrependSlash(name)

	// The client used to access this file will be used for an indeterminate time, so we don't want to use
	// a client from the pool.

	client := fs.getNewClient()

	// TODO: Temporary dev implementation: read entire file into memory.
	stream, err := client.OpenRead(name)
	if err != nil {
		if ErrorLogging {
			log.Printf("Open: error opening file stream on device for '%s': %+v", name, err)
		}
		return nil, fuse.EIO
	}
	defer stream.Close()

	if VerboseLogging {
		log.Printf("Open: reading entire file into memory: %s", name)
	}

	data, err := ioutil.ReadAll(stream)
	if err != nil {
		if ErrorLogging {
			log.Printf("Open: error reading data from file '%s': %+v", name, err)
		}
		return nil, fuse.EIO
	}

	if VerboseLogging {
		log.Printf("Open: read %d bytes from %s", len(data), name)
	}

	// TODO: In the future, will want to replace with NewAdbFile().
	file = nodefs.NewDataFile(data)

	if VerboseLogging {
		file = NewLoggingFile(file)
	}

	return file, fuse.OK
}

func (fs *AdbFileSystem) getNewClient() (client DeviceClient) {
	if VerboseLogging {
		log.Println("creating new client…")
	}

	client = fs.clientFactory()

	if VerboseLogging {
		log.Println("created client:", client)
	}
	return
}

func (fs *AdbFileSystem) getQuickUseClient() (client DeviceClient) {
	if VerboseLogging {
		log.Println("retrieving client…")
	}

	client = <-fs.quickUseClientPool

	if VerboseLogging {
		log.Println("client retrieved:", client)
	}
	return
}

func (fs *AdbFileSystem) recycleQuickUseClient(client DeviceClient) {
	if VerboseLogging {
		log.Println("recycling client:", client)
	}

	fs.quickUseClientPool <- client
}
