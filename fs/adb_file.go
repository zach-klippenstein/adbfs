package fs

import "github.com/hanwen/go-fuse/fuse/nodefs"

// AdbFile is a nodefs.File that is backed by a file on an adb device.
type AdbFile struct {
	nodefs.File
}

func NewAdbFile() nodefs.File {
	return nodefs.NewReadOnlyFile(&AdbFile{
		File: nodefs.NewDefaultFile(),
	})
}
