package adbfs

import (
	"os"

	"github.com/hanwen/go-fuse/fuse"
)

const (
	O_RDONLY = FileOpenFlags(os.O_RDONLY)
	O_WRONLY = FileOpenFlags(os.O_WRONLY)
	O_RDWR   = FileOpenFlags(os.O_RDWR)
	O_APPEND = FileOpenFlags(os.O_APPEND)
	O_CREATE = FileOpenFlags(os.O_CREATE)
	O_EXCL   = FileOpenFlags(os.O_EXCL)
	O_SYNC   = FileOpenFlags(os.O_SYNC)
	O_TRUNC  = FileOpenFlags(os.O_TRUNC)
)

// Helper methods around the flags passed to the Open call.
type FileOpenFlags uint32

func (f FileOpenFlags) String() string {
	return fuse.FlagString(fuse.OpenFlagNames, int64(f), "")
}

func (f FileOpenFlags) GoString() string {
	return f.String()
}

func (f FileOpenFlags) CanRead() bool {
	// O_RDONLY is just 0, so we can't do Contains(O_RDONLY).
	return !f.Contains(O_WRONLY)
}

func (f FileOpenFlags) CanWrite() bool {
	return f.Contains(O_WRONLY | O_RDWR)
}

// Contains returns true if the current flags contain any of the bits in bits.
func (f FileOpenFlags) Contains(bits FileOpenFlags) bool {
	return (f & bits) != 0
}
