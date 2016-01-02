package adbfs

import (
	"sync"

	"github.com/zach-klippenstein/adbfs/internal/cli"
)

type OpenFilesOptions struct {
	DeviceSerial  string
	ClientFactory DeviceClientFactory
}

// OpenFiles tracks and manages the set of all open files in a filesystem.
type OpenFiles struct {
	OpenFilesOptions

	lock          sync.Mutex
	buffersByPath map[string]*FileBuffer
}

func NewOpenFiles(opts OpenFilesOptions) *OpenFiles {
	return &OpenFiles{
		OpenFilesOptions: opts,
		buffersByPath:    make(map[string]*FileBuffer),
	}
}

func (f *OpenFiles) GetOrLoad(path string, openFlags FileOpenFlags, logEntry *LogEntry) (file *FileBuffer, err error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	if file = f.buffersByPath[path]; file == nil {
		file, err = NewFileBuffer(openFlags, FileBufferOptions{
			Path:                path,
			Client:              f.OpenFilesOptions.ClientFactory(),
			ZeroRefCountHandler: f.release,
		}, logEntry)
		if err != nil {
			return nil, err
		}
		f.buffersByPath[path] = file
	}

	// The refcount will be decremented when the AdbFile is released.
	refCount := file.IncRefCount()
	cli.Log.Debugf("OpenFiles: refcount is now %d for %s", refCount, path)

	return file, nil
}

func (f *OpenFiles) release(file *FileBuffer) {
	// Acquire the lock first, so that a concurrent call to GetOrLoad won't be able to increment
	// the refcount before we remove it from the map.
	f.lock.Lock()
	defer f.lock.Unlock()

	// However, the GetOrLoad may already have beat us to the punch.
	if file.RefCount() != 0 {
		return
	}

	cli.Log.Debugf("OpenFiles: releasing FileBuffer for %s", file.Path)
	delete(f.buffersByPath, file.Path)
}
