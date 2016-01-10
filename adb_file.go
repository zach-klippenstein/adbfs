package adbfs

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/zach-klippenstein/goadb/util"
)

const (
	// See AdbFileOpenOptions.Perms.
	DontSetPerms = os.FileMode(0)

	// This seems pretty long, but every write that occurs after this period will
	// flush the entire buffer to the device – this could take a while, and if we do it
	// to often we're effectively thrashing. If a file has been written to continuously for
	// this time, it's guaranteed to take at least as long, probably a lot longer, to flush
	// to the device (process->kernel->fuse has lower latency than process->adb->device).
	DefaultDirtyTimeout = 5 * time.Minute
)

type AdbFileOpenOptions struct {
	// If the create flag is set, the file will immediately be created if it does not exist.
	Flags      FileOpenFlags
	FileBuffer *FileBuffer
}

/*
AdbFile is a nodefs.File that is backed by a file on an adb device.
There is one AdbFile for each file descriptor. All AdbFiles that point to the same
path are backed by the same FileBuffer.

Note: On OSX at least, the OS will automatically map multiple open files to a single AdbFile.
*/
type AdbFile struct {
	nodefs.File
	AdbFileOpenOptions
}

var _ nodefs.File = &AdbFile{}

// NewAdbFile returns a File that reads and writes to name on the device.
// perms should be set from the existing file if it exists, or to the desired new permissions if new.
func NewAdbFile(opts AdbFileOpenOptions) nodefs.File {
	logEntry := StartFileOperation("New", opts.FileBuffer.Path, fmt.Sprint(opts))
	defer logEntry.FinishOperation()

	adbFile := &AdbFile{
		// Log all the operations we don't implement.
		File:               newLoggingFile(nodefs.NewDefaultFile(), opts.FileBuffer.Path),
		AdbFileOpenOptions: opts,
	}

	return adbFile
}

func (f *AdbFile) startFileOperation(name string, args string) *LogEntry {
	return StartFileOperation(name, f.FileBuffer.Path, args)
}

func (f *AdbFile) InnerFile() nodefs.File {
	return f.File
}

func (f *AdbFile) Release() {
	logEntry := f.startFileOperation("Release", "")
	defer logEntry.FinishOperation()

	// Cleanup the underlying buffer after the last open file is closed.
	f.FileBuffer.DecRefCount()
}

func (f *AdbFile) Read(buf []byte, off int64) (fuse.ReadResult, fuse.Status) {
	logEntry := f.startFileOperation("Read", formatArgsListForLog(buf, off))
	defer logEntry.FinishOperation()

	if !f.Flags.CanRead() {
		// This is not a user-permission denial, it's a filesystem config denial, so don't use EACCES.
		return readError(ErrNotPermitted, logEntry)
	}

	n, err := f.FileBuffer.ReadAt(buf, off)
	if err == io.EOF {
		err = nil
	}
	if err != nil {
		return readError(err, logEntry)
	}

	logEntry.Result("read %d bytes", n)
	return fuse.ReadResultData(buf[:n]), toFuseStatusLog(OK, logEntry)
}

// Fsync flushes the file to device if the dirty flag is set, else re-reads the file from the device
// into memory.
func (f *AdbFile) Fsync(flags int) fuse.Status {
	logEntry := f.startFileOperation("Fsync", formatArgsListForLog(flags))
	defer logEntry.FinishOperation()

	err := f.FileBuffer.Sync(logEntry)
	return toFuseStatusLog(err, logEntry)
}

func (f *AdbFile) GetAttr(out *fuse.Attr) fuse.Status {
	logEntry := f.startFileOperation("GetAttr", "")
	defer logEntry.FinishOperation()

	// This operation doesn't require a read flag.

	err := getAttr(f.FileBuffer.Path, f.FileBuffer.Client, logEntry, out)
	return toFuseStatusLog(err, logEntry)
}

func (f *AdbFile) Write(data []byte, off int64) (uint32, fuse.Status) {
	logEntry := f.startFileOperation("Write", formatArgsListForLog(data, off))
	defer logEntry.FinishOperation()

	if !f.Flags.CanWrite() {
		return 0, toFuseStatusLog(ErrNotPermitted, logEntry)
	}

	n, err := f.FileBuffer.WriteAt(data, off)
	logEntry.Result("wrote %d bytes", n)

	if err == nil {
		err = f.FileBuffer.SyncIfTooDirty(logEntry)
		if err != nil {
			err = util.WrapErrf(err, "write successful, but error syncing after write")
		}
	}

	return uint32(n), toFuseStatusLog(err, logEntry)
}

func (f *AdbFile) Flush() fuse.Status {
	logEntry := f.startFileOperation("Flush", "")
	defer logEntry.FinishOperation()

	if !f.Flags.CanWrite() {
		// Flush is *always* called when the fd is closed, so it doesn't make sense
		// to return a permission error here. Instead, it's just a no-op.
		// This is also what the implementation of nodefs.NewDefaultFile does.
		return toFuseStatusLog(OK, logEntry)
	}

	err := f.FileBuffer.Flush(logEntry)
	return toFuseStatusLog(err, logEntry)
}

func (f *AdbFile) Truncate(size uint64) fuse.Status {
	logEntry := f.startFileOperation("Truncate", formatArgsListForLog(size, 10))
	defer logEntry.FinishOperation()

	if !f.Flags.CanWrite() {
		return toFuseStatusLog(ErrNotPermitted, logEntry)
	}

	f.FileBuffer.SetSize(int64(size))
	err := f.FileBuffer.Sync(logEntry)
	return toFuseStatusLog(err, logEntry)
}
