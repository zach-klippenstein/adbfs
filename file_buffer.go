package adbfs

import (
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	. "github.com/zach-klippenstein/adbfs/internal/util"
	"github.com/zach-klippenstein/goadb"
	"github.com/zach-klippenstein/goadb/util"
)

const DefaultFilePermissions = os.FileMode(0664)

type FileBufferOptions struct {
	Path         string
	Client       DeviceClient
	Clock        Clock
	DirtyTimeout time.Duration

	// The permissions to set on the file when flushing.
	// If this is DontSetPerms, the file's existing permissions will be used.
	// Set from the existing file if it exists, or to the desired new permissions if new.
	Perms os.FileMode

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
	lock     sync.Mutex

	// Stores the entire file in memory.
	buffer GrowableByteSlice
	dirty  *DirtyTimestamp
}

var (
	_ io.ReaderAt = &FileBuffer{}
	_ io.WriterAt = &FileBuffer{}
)

// NewFileBuffer returns a File that reads and writes to name on the device.
// initialFlags are the flags being used to open the file the first time, and are only used to
// determine if the buffer needs to be read into memory when initializing.
func NewFileBuffer(initialFlags FileOpenFlags, opts FileBufferOptions, logEntry *LogEntry) (file *FileBuffer, err error) {
	file = &FileBuffer{
		FileBufferOptions: opts,
		dirty:             NewDirtyTimestamp(opts.Clock),
	}
	if err := file.initialize(initialFlags, logEntry); err != nil {
		return nil, err
	}
	return file, nil
}

func (f *FileBuffer) initialize(flags FileOpenFlags, logEntry *LogEntry) (err error) {
	createNewFile := false

	flagRequiresWrite := flags.Contains(O_CREATE | O_TRUNC | O_APPEND)
	if flagRequiresWrite && !flags.CanWrite() {
		return ErrNotPermitted
	}

	currentPerms, err := f.getFilePermissions(logEntry)
	if util.HasErrCode(err, util.FileNoExistError) {
		// The file doesn't exist.
		if !flags.Contains(O_CREATE) {
			// If the file doesn't exist and we can't create, we can't do anything.
			return err
		}

		createNewFile = true
		currentPerms = DefaultFilePermissions
		err = nil
	} else if err != nil {
		return err
	}

	if f.Perms == DontSetPerms {
		// Open won't set perms and we want to use the existing ones from the file if it exists.
		// If it doesn't, use the default ones from the AdbFileSystem.
		// Create should always set this to non-zero.
		f.Perms = currentPerms
	}

	// Now either the file exists or it doesn't and we can create it.

	if createNewFile || flags.Contains(O_TRUNC) {
		// We don't care what was in the file (if it exists), leave the buffer empty.
		//
		// Not sure about other OSes, but OSX Finder will do a GetAttr on the file immediately after
		// the Create syscall (before flushing), and if it fails, will give up.
		f.dirty.Set()
	}

	// Perform the initial load or save.
	f.Sync(logEntry)

	return
}

func (f *FileBuffer) getFilePermissions(logEntry *LogEntry) (os.FileMode, error) {
	entry, err := f.Client.Stat(f.Path, logEntry)
	if err != nil {
		return 0, util.WrapErrf(err, "error reading file permissions")
	}
	return entry.Mode.Perm(), nil
}

func (f *FileBuffer) Contents() string {
	return f.buffer.String()
}

// ReadAt implements the io.ReaderAt interface.
func (f *FileBuffer) ReadAt(buf []byte, off int64) (n int, err error) {
	f.lock.Lock()
	defer f.lock.Unlock()
	return f.buffer.ReadAt(buf, off)
}

func (f *FileBuffer) WriteAt(data []byte, off int64) (int, error) {
	f.lock.Lock()
	defer f.lock.Unlock()
	// FileBuffer.WriteAt will never fail, so we can set the dirty flag before writing.
	f.dirty.Set()
	return f.buffer.WriteAt(data, off)
}

// Sync saves the buffer to the device if dirty, else reloads the buffer from the device.
// Like Flush, but reloads the buffer if not dirty.
func (f *FileBuffer) Sync(logEntry *LogEntry) error {
	f.lock.Lock()
	defer f.lock.Unlock()

	if f.dirty.IsSet() {
		return f.saveToDevice(logEntry)
	} else {
		return f.loadFromDevice(logEntry)
	}
}

// Flush saves the buffer to the device if dirty, else does nothing.
// Like Sync, but without the read.
func (f *FileBuffer) Flush(logEntry *LogEntry) error {
	f.lock.Lock()
	defer f.lock.Unlock()

	if f.dirty.IsSet() {
		return f.saveToDevice(logEntry)
	}
	return nil
}

// SyncIfTooDirty performs a sync if the buffer has been dirty for longer than the timeout specified
// in FileBufferOptions.
func (f *FileBuffer) SyncIfTooDirty(logEntry *LogEntry) error {
	f.lock.Lock()
	defer f.lock.Unlock()

	if f.dirty.HasBeenDirtyFor(f.FileBufferOptions.DirtyTimeout) {
		return f.saveToDevice(logEntry)
	}
	return nil
}

func (f *FileBuffer) SetSize(size int64) {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.dirty.Set()
	f.buffer.Resize(size)
}

func (f *FileBuffer) Size() int64 {
	f.lock.Lock()
	defer f.lock.Unlock()
	return f.buffer.Len()
}

func (f *FileBuffer) IsDirty() bool {
	return f.dirty.IsSet()
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

	n, err := f.buffer.ReadFrom(stream)
	if err != nil {
		return util.WrapErrf(err, "error reading data from file (after reading %d bytes)", n)
	}
	return nil
}

func (f *FileBuffer) saveToDevice(logEntry *LogEntry) error {
	writer, err := f.Client.OpenWrite(f.Path, f.Perms, goadb.MtimeOfClose, logEntry)
	if err != nil {
		return util.WrapErrf(err, "error opening file stream on device")
	}
	// Make sure we close the writer if we return early, but we still want to check
	// for errors in the happy case.
	defer writer.Close()

	// TODO Optimize by using a buffer that is wire.SyncMaxChunkSize.
	n, err := f.buffer.WriteTo(writer)
	if err != nil {
		return util.WrapErrf(err, "writing data to file: len(buffer)=%d, n=%d", f.buffer.Len(), n)
	}

	if err := writer.Close(); err != nil {
		return util.WrapErrf(err, "closing file stream")
	}

	// If there were any errors, the file may not have been written on device at all, so we're still
	// dirty.
	f.dirty.Clear()

	return nil
}
