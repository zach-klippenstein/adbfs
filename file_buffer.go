package adbfs

import (
	"io"
	"io/ioutil"
	"os"
	"sync"
	"sync/atomic"

	"github.com/zach-klippenstein/goadb/util"
)

const DefaultFilePermissions = os.FileMode(0664)

type FileBufferOptions struct {
	Path   string
	Client DeviceClient

	// Function called when ref count hits 0.
	// Note that, because concurrency, the ref count may be incremented again by the time
	// this function is executed.
	ZeroRefCountHandler func(*FileBuffer)
}

/*
FileBuffer loads, provides read/write access to, and saves a file on the device.
A single FileBuffer backs all the open files for a given path to provide as consistent a view
of that file as possible.
1 or more AdbFiles may point to a single FileBuffer.

Note: On OSX at least, the OS will automatically map multiple open files to a single AdbFile.
Still, this type is still useful because it separates the file model and logic from the go-fuse-specific
integration code.
*/
type FileBuffer struct {
	FileBufferOptions

	refCount int32

	// Stores the entire file in memory.
	buffer []byte
	lock   sync.Mutex
}

var _ io.ReaderAt = &FileBuffer{}

// NewFileBuffer returns a File that reads and writes to name on the device.
// initialFlags are the flags being used to open the file the first time, and are only used to
// determine if the buffer needs to be read into memory when initializing.
func NewFileBuffer(initialFlags FileOpenFlags, opts FileBufferOptions, logEntry *LogEntry) (file *FileBuffer, err error) {
	file = &FileBuffer{
		FileBufferOptions: opts,
	}
	if err := file.initialize(initialFlags, logEntry); err != nil {
		return nil, err
	}
	return file, nil
}

func (f *FileBuffer) initialize(flags FileOpenFlags, logEntry *LogEntry) (err error) {
	if !flags.CanRead() || flags.Contains(O_TRUNC) || flags.Contains(O_APPEND) {
		return ErrNotPermitted
	}

	if _, err = f.Client.Stat(f.Path, logEntry); err != nil {
		return err
	}

	// Perform the initial load.
	f.Sync(logEntry)

	return
}

func (f *FileBuffer) Contents() string {
	return string(f.buffer)
}

// ReadAt implements the io.ReaderAt interface.
func (f *FileBuffer) ReadAt(buf []byte, off int64) (n int, err error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	if off > int64(len(f.buffer)) {
		return 0, io.EOF
	}

	// Don't use Slice because we don't want to grow the slice.
	n = copy(buf, f.buffer[off:])
	if n+int(off) == len(f.buffer) {
		// This is still a successful read, but there's no more data.
		err = io.EOF
	}
	return n, err
}

// Sync saves the buffer to the device if dirty, else reloads the buffer from the device.
func (f *FileBuffer) Sync(logEntry *LogEntry) error {
	f.lock.Lock()
	defer f.lock.Unlock()
	return f.loadFromDevice(logEntry)
}

func (f *FileBuffer) IncRefCount() int {
	return int(atomic.AddInt32(&f.refCount, 1))
}

func (f *FileBuffer) DecRefCount() int {
	newCount := int(atomic.AddInt32(&f.refCount, -1))
	if newCount < 0 {
		panic("refcount decremented past 0")
	}
	if newCount == 0 && f.ZeroRefCountHandler != nil {
		f.ZeroRefCountHandler(f)
	}
	return newCount
}

func (f *FileBuffer) RefCount() int {
	return int(atomic.LoadInt32(&f.refCount))
}

// read reads the file from the device into the buffer.
func (f *FileBuffer) loadFromDevice(logEntry *LogEntry) error {
	stream, err := f.Client.OpenRead(f.Path, logEntry)
	if err != nil {
		return util.WrapErrf(err, "error opening file stream on device")
	}
	defer stream.Close()

	data, err := ioutil.ReadAll(stream)
	if err != nil {
		return util.WrapErrf(err, "error reading data from file (after reading %d bytes)", len(data))
	}
	f.buffer = data
	return nil
}
