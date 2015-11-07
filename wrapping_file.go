package adbfs

import (
	"fmt"
	"time"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
)

// WrappingFile is an implementation of nodefs.File that invokes a callback after
// every method call.
type WrappingFile struct {
	nodefs.File

	BeforeCall func(fs *WrappingFile, method string, args ...interface{}) (call interface{})

	// AfterCall is called after every operation on the file with the method receiver,
	// the name of the method, and slices of all the passed and returned values.
	AfterCall func(fs *WrappingFile, call interface{}, status *fuse.Status, results ...interface{})
}

// The String method is for debug printing.
func (f *WrappingFile) String() string {
	return fmt.Sprintf("WrappingFile(%s)", f.File.String())
}

// Wrappers around other File implementations, should return
// the inner file here.
func (f *WrappingFile) InnerFile() (file nodefs.File) {
	return f.File
}

// Called upon registering the filehandle in the inode.
func (f *WrappingFile) SetInode(inode *nodefs.Inode) {
	call := f.BeforeCall(f, "SetInode", inode)
	f.File.SetInode(inode)
	f.AfterCall(f, call, nil)
}

func (f *WrappingFile) Read(dest []byte, off int64) (result fuse.ReadResult, code fuse.Status) {
	call := f.BeforeCall(f, "Read", dest, off)
	result, code = f.File.Read(dest, off)
	f.AfterCall(f, call, &code, result)
	return
}

func (f *WrappingFile) Write(data []byte, off int64) (written uint32, code fuse.Status) {
	call := f.BeforeCall(f, "Write", data, off)
	written, code = f.File.Write(data, off)
	f.AfterCall(f, call, &code, written)
	return
}

// Flush is called for close() call on a file descriptor. In
// case of duplicated descriptor, it may be called more than
// once for a file.
func (f *WrappingFile) Flush() (code fuse.Status) {
	call := f.BeforeCall(f, "Flush")
	code = f.File.Flush()
	f.AfterCall(f, call, &code)
	return
}

// This is called to before the file handle is forgotten. This
// method has no return value, so nothing can synchronizes on
// the call. Any cleanup that requires specific synchronization or
// could fail with I/O errors should happen in Flush instead.
func (f *WrappingFile) Release() {
	call := f.BeforeCall(f, "Release")
	f.File.Release()
	f.AfterCall(f, call, nil)
}

func (f *WrappingFile) Fsync(flags int) (code fuse.Status) {
	call := f.BeforeCall(f, "Fsync", flags)
	code = f.File.Fsync(flags)
	f.AfterCall(f, call, &code)
	return
}

// The methods below may be called on closed files, due to
// concurrency.  In that case, you should return EBADF.
func (f *WrappingFile) Truncate(size uint64) (code fuse.Status) {
	call := f.BeforeCall(f, "Truncate", size)
	code = f.File.Truncate(size)
	f.AfterCall(f, call, &code)
	return
}

func (f *WrappingFile) GetAttr(out *fuse.Attr) (code fuse.Status) {
	call := f.BeforeCall(f, "GetAttr", out)
	code = f.File.GetAttr(out)
	f.AfterCall(f, call, &code)
	return
}

func (f *WrappingFile) Chown(uid uint32, gid uint32) (code fuse.Status) {
	call := f.BeforeCall(f, "Chown", uid, gid)
	code = f.File.Chown(uid, gid)
	f.AfterCall(f, call, &code)
	return
}

func (f *WrappingFile) Chmod(perms uint32) (code fuse.Status) {
	call := f.BeforeCall(f, "Chmod", perms)
	code = f.File.Chmod(perms)
	f.AfterCall(f, call, &code)
	return
}

func (f *WrappingFile) Utimens(atime *time.Time, mtime *time.Time) (code fuse.Status) {
	call := f.BeforeCall(f, "Utimens", atime, mtime)
	code = f.File.Utimens(atime, mtime)
	f.AfterCall(f, call, &code)
	return
}

func (f *WrappingFile) Allocate(off uint64, size uint64, mode uint32) (code fuse.Status) {
	call := f.BeforeCall(f, "Allocate", off, size, mode)
	code = f.File.Allocate(off, size, mode)
	f.AfterCall(f, call, &code)
	return
}
