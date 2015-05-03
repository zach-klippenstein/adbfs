package fs

import (
	"fmt"
	"os"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/zach-klippenstein/goadb"
)

func PrependSlash(name string) string {
	return string(append([]byte("/"), []byte(name)...))
}

func CollectDirEntries(entries *goadb.DirEntries) (result []fuse.DirEntry, err error) {
	defer entries.Close()

	for entries.Next() {
		entry := entries.Entry()
		result = append(result, fuse.DirEntry{
			Name: entry.Name,
			Mode: OsFileModeToFuseFileMode(entry.Mode),
		})
	}
	err = entries.Err()

	return
}

func NewAttr(entry *goadb.DirEntry) *fuse.Attr {
	return &fuse.Attr{
		Mode:  OsFileModeToFuseFileMode(entry.Mode),
		Size:  uint64(entry.Size),
		Mtime: uint64(entry.ModifiedAt.Unix()),
	}
}

func OsFileModeToFuseFileMode(inMode os.FileMode) (outMode uint32) {
	if inMode.IsRegular() {
		outMode |= fuse.S_IFREG
	}
	if inMode.IsDir() {
		outMode |= fuse.S_IFDIR
	}
	if inMode&os.ModeSymlink == os.ModeSymlink {
		outMode |= fuse.S_IFLNK
	}
	if inMode&os.ModeNamedPipe == os.ModeNamedPipe {
		outMode |= fuse.S_IFIFO
	}
	outMode |= uint32(inMode.Perm())
	return
}

// summarizeByteSlices replaces all elements of the passed slice that are of type []byte with
// their length and type, so for logging they neither show sensitive data nor flood the log.
func summarizeByteSlices(vals []interface{}) {
	for i, val := range vals {
		if slice, ok := val.([]byte); ok {
			vals[i] = fmt.Sprintf("[]byte(%d)", len(slice))
		}
	}
}
