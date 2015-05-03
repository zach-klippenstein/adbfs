package fs

import (
	"fmt"
	"log"
	"time"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
)

type WrappingFileSystem struct {
	pathfs.FileSystem
	AfterCall func(fs *WrappingFileSystem, method string, args, results []interface{})
}

func NewLoggingFileSystem(fs pathfs.FileSystem) pathfs.FileSystem {
	return &WrappingFileSystem{
		FileSystem: fs,
		AfterCall: func(fs *WrappingFileSystem, method string, args, results []interface{}) {
			if len(args) > 0 {
				// The last arg is the context pointer, so pull out the actual object instead of just
				// printing the pointer value.
				args[len(args)-1] = fmt.Sprintf("%+v", args[len(args)-1])
			}

			log.Printf("[%s] %s: %+v → %+v", fs.FileSystem, method, args, results)
		},
	}
}

// Used for pretty printing.
func (fs *WrappingFileSystem) String() string {
	return fmt.Sprintf("WrappingFileSystem(%s)", fs.FileSystem.String())
}

// If called, provide debug output through the log package.
func (fs *WrappingFileSystem) SetDebug(debug bool) {
	fs.FileSystem.SetDebug(debug)
	fs.AfterCall(fs, "SetDebug", []interface{}{debug}, []interface{}{})
}

// Attributes.  This function is the main entry point, through
// which FUSE discovers which files and directories exist.
//
// If the filesystem wants to implement hard-links, it should
// return consistent non-zero FileInfo.Ino data.  Using
// hardlinks incurs a performance hit.
func (fs *WrappingFileSystem) GetAttr(name string, context *fuse.Context) (attr *fuse.Attr, status fuse.Status) {
	attr, status = fs.FileSystem.GetAttr(name, context)
	fs.AfterCall(fs, "GetAttr", []interface{}{name, context}, []interface{}{attr, status})
	return
}

// These should update the file's ctime too.
func (fs *WrappingFileSystem) Chmod(name string, mode uint32, context *fuse.Context) (code fuse.Status) {
	code = fs.FileSystem.Chmod(name, mode, context)
	fs.AfterCall(fs, "Chmod", []interface{}{name, mode, context}, []interface{}{code})
	return
}

func (fs *WrappingFileSystem) Chown(name string, uid uint32, gid uint32, context *fuse.Context) (code fuse.Status) {
	code = fs.FileSystem.Chown(name, uid, gid, context)
	fs.AfterCall(fs, "Chown", []interface{}{name, uid, gid, context}, []interface{}{code})
	return
}

func (fs *WrappingFileSystem) Utimens(name string, Atime *time.Time, Mtime *time.Time, context *fuse.Context) (code fuse.Status) {
	code = fs.FileSystem.Utimens(name, Atime, Mtime, context)
	fs.AfterCall(fs, "Utimens", []interface{}{name, Atime, Mtime, context}, []interface{}{code})
	return
}

func (fs *WrappingFileSystem) Truncate(name string, size uint64, context *fuse.Context) (code fuse.Status) {
	code = fs.FileSystem.Truncate(name, size, context)
	fs.AfterCall(fs, "Truncate", []interface{}{name, size, context}, []interface{}{code})
	return
}

func (fs *WrappingFileSystem) Access(name string, mode uint32, context *fuse.Context) (code fuse.Status) {
	code = fs.FileSystem.Access(name, mode, context)
	fs.AfterCall(fs, "Access", []interface{}{name, mode, context}, []interface{}{code})
	return
}

// Tree structure
func (fs *WrappingFileSystem) Link(oldName string, newName string, context *fuse.Context) (code fuse.Status) {
	code = fs.FileSystem.Link(oldName, newName, context)
	fs.AfterCall(fs, "Link", []interface{}{oldName, newName, context}, []interface{}{code})
	return
}

func (fs *WrappingFileSystem) Mkdir(name string, mode uint32, context *fuse.Context) (code fuse.Status) {
	code = fs.FileSystem.Mkdir(name, mode, context)
	fs.AfterCall(fs, "Mkdir", []interface{}{name, mode, context}, []interface{}{code})
	return
}

func (fs *WrappingFileSystem) Mknod(name string, mode uint32, dev uint32, context *fuse.Context) (code fuse.Status) {
	code = fs.FileSystem.Mknod(name, mode, dev, context)
	fs.AfterCall(fs, "Mknod", []interface{}{name, mode, dev, context}, []interface{}{code})
	return
}

func (fs *WrappingFileSystem) Rename(oldName string, newName string, context *fuse.Context) (code fuse.Status) {
	code = fs.FileSystem.Rename(oldName, newName, context)
	fs.AfterCall(fs, "Rename", []interface{}{oldName, newName, context}, []interface{}{code})
	return
}

func (fs *WrappingFileSystem) Rmdir(name string, context *fuse.Context) (code fuse.Status) {
	code = fs.FileSystem.Rmdir(name, context)
	fs.AfterCall(fs, "Rmdir", []interface{}{name, context}, []interface{}{code})
	return
}

func (fs *WrappingFileSystem) Unlink(name string, context *fuse.Context) (code fuse.Status) {
	code = fs.FileSystem.Unlink(name, context)
	fs.AfterCall(fs, "Unlink", []interface{}{name, context}, []interface{}{code})
	return
}

// Extended attributes.
func (fs *WrappingFileSystem) GetXAttr(name string, attribute string, context *fuse.Context) (data []byte, code fuse.Status) {
	data, code = fs.FileSystem.GetXAttr(name, attribute, context)
	fs.AfterCall(fs, "GetXAttr", []interface{}{name, attribute, context}, []interface{}{data, code})
	return
}

func (fs *WrappingFileSystem) ListXAttr(name string, context *fuse.Context) (attributes []string, code fuse.Status) {
	attributes, code = fs.FileSystem.ListXAttr(name, context)
	fs.AfterCall(fs, "ListXAttr", []interface{}{name, context}, []interface{}{attributes, code})
	return
}

func (fs *WrappingFileSystem) RemoveXAttr(name string, attr string, context *fuse.Context) (code fuse.Status) {
	code = fs.FileSystem.RemoveXAttr(name, attr, context)
	fs.AfterCall(fs, "RemoveXAttr", []interface{}{name, attr, context}, []interface{}{code})
	return
}

func (fs *WrappingFileSystem) SetXAttr(name string, attr string, data []byte, flags int, context *fuse.Context) (code fuse.Status) {
	code = fs.FileSystem.SetXAttr(name, attr, data, flags, context)
	fs.AfterCall(fs, "SetXAttr", []interface{}{name, attr, data, flags, context}, []interface{}{code})
	return
}

// Called after mount.
func (fs *WrappingFileSystem) OnMount(nodeFs *pathfs.PathNodeFs) {
	fs.FileSystem.OnMount(nodeFs)
	fs.AfterCall(fs, "OnMount", []interface{}{nodeFs}, []interface{}{})
}

func (fs *WrappingFileSystem) OnUnmount() {
	fs.FileSystem.OnUnmount()
	fs.AfterCall(fs, "OnUnmount", []interface{}{}, []interface{}{})
}

// File handling.  If opening for writing, the file's mtime
// should be updated too.
func (fs *WrappingFileSystem) Open(name string, flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	file, code = fs.FileSystem.Open(name, flags, context)
	fs.AfterCall(fs, "Open", []interface{}{name, flags, context}, []interface{}{file, code})
	return
}

func (fs *WrappingFileSystem) Create(name string, flags uint32, mode uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	file, code = fs.FileSystem.Create(name, flags, mode, context)
	fs.AfterCall(fs, "Create", []interface{}{name, flags, mode, context}, []interface{}{file, code})
	return
}

// Directory handling
func (fs *WrappingFileSystem) OpenDir(name string, context *fuse.Context) (stream []fuse.DirEntry, code fuse.Status) {
	stream, code = fs.FileSystem.OpenDir(name, context)
	fs.AfterCall(fs, "OpenDir", []interface{}{name, context}, []interface{}{stream, code})
	return
}

// Symlinks.
func (fs *WrappingFileSystem) Symlink(value string, linkName string, context *fuse.Context) (code fuse.Status) {
	code = fs.FileSystem.Symlink(value, linkName, context)
	fs.AfterCall(fs, "Symlink", []interface{}{value, linkName, context}, []interface{}{code})
	return
}

func (fs *WrappingFileSystem) Readlink(name string, context *fuse.Context) (target string, code fuse.Status) {
	target, code = fs.FileSystem.Readlink(name, context)
	fs.AfterCall(fs, "Readlink", []interface{}{name, context}, []interface{}{target, code})
	return
}

func (fs *WrappingFileSystem) StatFs(name string) (statfs *fuse.StatfsOut) {
	statfs = fs.FileSystem.StatFs(name)
	fs.AfterCall(fs, "StatFs", []interface{}{name}, []interface{}{statfs})
	return
}
