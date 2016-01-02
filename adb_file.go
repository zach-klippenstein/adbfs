package adbfs

import (
	"fmt"
	"io"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
)

type AdbFileOpenOptions struct {
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

	return nodefs.NewReadOnlyFile(adbFile)
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
		return readResultError(fuse.EPERM), logEntry.Status(fuse.EPERM)
	}

	n, err := f.FileBuffer.ReadAt(buf, off)
	if err == io.EOF {
		err = nil
	}

	logEntry.Result("read %d bytes", n)
	return fuse.ReadResultData(buf[:n]), toFuseStatus(err, logEntry)
}

// Fsync re-reads the file from the device into memory.
func (f *AdbFile) Fsync(flags int) fuse.Status {
	logEntry := f.startFileOperation("Fsync", formatArgsListForLog(flags))
	defer logEntry.FinishOperation()

	err := f.FileBuffer.Sync(logEntry)
	return toFuseStatus(err, logEntry)
}

func (f *AdbFile) GetAttr(out *fuse.Attr) fuse.Status {
	logEntry := f.startFileOperation("GetAttr", "")
	defer logEntry.FinishOperation()

	// This operation doesn't require a read flag.

	return logEntry.Status(getAttr(f.FileBuffer.Path, f.FileBuffer.Client, logEntry, out))
}
