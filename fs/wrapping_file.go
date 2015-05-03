package fs

import (
	"fmt"
	"log"
	"time"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
)

type WrappingFile struct {
	nodefs.File
	AfterCall func(fs *WrappingFile, method string, args, results []interface{})
}

func NewLoggingFile(file nodefs.File) nodefs.File {
	return &WrappingFile{
		File: file,
		AfterCall: func(f *WrappingFile, method string, args, results []interface{}) {
			summarizeByteSlices(args)
			summarizeByteSlices(results)

			log.Printf("[%s] %s: %+v → %+v", f.File, method, args, results)
		},
	}
}

// Called upon registering the filehandle in the inode.
func (f *WrappingFile) SetInode(inode *nodefs.Inode) {
	f.File.SetInode(inode)
	f.AfterCall(f, "SetInode", []interface{}{inode}, []interface{}{})
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

func (f *WrappingFile) Read(dest []byte, off int64) (result fuse.ReadResult, code fuse.Status) {
	result, code = f.File.Read(dest, off)
	f.AfterCall(f, "Read", []interface{}{dest, off}, []interface{}{result, code})
	return
}

func (f *WrappingFile) Write(data []byte, off int64) (written uint32, code fuse.Status) {
	written, code = f.File.Write(data, off)
	f.AfterCall(f, "Write", []interface{}{data, off}, []interface{}{written, code})
	return
}

// Flush is called for close() call on a file descriptor. In
// case of duplicated descriptor, it may be called more than
// once for a file.
func (f *WrappingFile) Flush() (code fuse.Status) {
	code = f.File.Flush()
	f.AfterCall(f, "Flush", []interface{}{}, []interface{}{code})
	return
}

// This is called to before the file handle is forgotten. This
// method has no return value, so nothing can synchronizes on
// the call. Any cleanup that requires specific synchronization or
// could fail with I/O errors should happen in Flush instead.
func (f *WrappingFile) Release() {
	f.File.Release()
	f.AfterCall(f, "Release", []interface{}{}, []interface{}{})
}

func (f *WrappingFile) Fsync(flags int) (code fuse.Status) {
	code = f.File.Fsync(flags)
	f.AfterCall(f, "Fsync", []interface{}{flags}, []interface{}{code})
	return
}

// The methods below may be called on closed files, due to
// concurrency.  In that case, you should return EBADF.
func (f *WrappingFile) Truncate(size uint64) (code fuse.Status) {
	code = f.File.Truncate(size)
	f.AfterCall(f, "Truncate", []interface{}{size}, []interface{}{code})
	return
}

func (f *WrappingFile) GetAttr(out *fuse.Attr) (code fuse.Status) {
	code = f.File.GetAttr(out)
	f.AfterCall(f, "GetAttr", []interface{}{out}, []interface{}{code})
	return
}

func (f *WrappingFile) Chown(uid uint32, gid uint32) (code fuse.Status) {
	code = f.File.Chown(uid, gid)
	f.AfterCall(f, "Chown", []interface{}{uid}, []interface{}{gid})
	return
}

func (f *WrappingFile) Chmod(perms uint32) (code fuse.Status) {
	code = f.File.Chmod(perms)
	f.AfterCall(f, "Chmod", []interface{}{perms}, []interface{}{code})
	return
}

func (f *WrappingFile) Utimens(atime *time.Time, mtime *time.Time) (code fuse.Status) {
	code = f.File.Utimens(atime, mtime)
	f.AfterCall(f, "Utimens", []interface{}{atime, mtime}, []interface{}{code})
	return
}

func (f *WrappingFile) Allocate(off uint64, size uint64, mode uint32) (code fuse.Status) {
	code = f.File.Allocate(off, size, mode)
	f.AfterCall(f, "Allocate", []interface{}{off, size, mode}, []interface{}{code})
	return
}
