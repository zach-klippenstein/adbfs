package fs

import "github.com/hanwen/go-fuse/fuse/nodefs"

type AdbFile struct {
	nodefs.File
}

func NewAdbFile() nodefs.File {
	return nodefs.NewReadOnlyFile(&AdbFile{
		File: nodefs.NewDefaultFile(),
	})
}
