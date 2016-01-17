package adbfs

import (
	"bytes"
	"fmt"
	"os"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/zach-klippenstein/goadb"
)

// asFuseDirEntries reads directory entries from a goadb DirEntries and returns them as a
// list of fuse DirEntry objects.
func asFuseDirEntries(entries []*adb.DirEntry) (result []fuse.DirEntry) {
	result = make([]fuse.DirEntry, len(entries))

	for i, entry := range entries {
		result[i] = fuse.DirEntry{
			Name: entry.Name,
			Mode: osFileModeToFuseFileMode(entry.Mode),
		}
	}

	return
}

// asFuseAttr creates a fuse Attr struct that contains the information from a goadb DirEntry.
func asFuseAttr(entry *adb.DirEntry, attr *fuse.Attr) {
	*attr = fuse.Attr{
		Mode:  osFileModeToFuseFileMode(entry.Mode),
		Size:  uint64(entry.Size),
		Mtime: uint64(entry.ModifiedAt.Unix()),
	}
}

// osFileModeToFuseFileMode converts a standard os.FileMode to the bit array used
// by the fuse package. Permissions, regular/dir modes, symlinks, and named pipes
// are the only bits that are converted.
func osFileModeToFuseFileMode(inMode os.FileMode) (outMode uint32) {
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

// newLoggingFile returns a file object that logs all operations performed on it.
func newLoggingFile(file nodefs.File, path string) nodefs.File {
	return &WrappingFile{
		File: file,
		BeforeCall: func(f *WrappingFile, method string, args ...interface{}) interface{} {
			return StartFileOperation(method, path, formatArgsListForLog(args...))
		},
		AfterCall: func(f *WrappingFile, call interface{}, status *fuse.Status, results ...interface{}) {
			logEntry := call.(*LogEntry)
			if status != nil {
				logEntry.Status(fuseStatusToErrno(*status))
			}
			logEntry.Result(formatArgsListForLog(results...))
			logEntry.FinishOperation()
		},
	}
}

func formatArgsListForLog(args ...interface{}) string {
	if len(args) == 0 {
		return ""
	}

	summarizeForLog(args)

	var buffer bytes.Buffer
	buffer.WriteRune('[')
	for i, item := range args {
		fmt.Fprintf(&buffer, "%#v", item)

		if i < len(args)-1 {
			buffer.WriteString(", ")
		}
	}
	buffer.WriteRune(']')
	return buffer.String()
}

// summarizeByteSlices replaces all elements of the passed slice that are of type []byte with
// their length and type, so for logging they neither show sensitive data nor flood the log.
func summarizeForLog(vals []interface{}) {
	for i, val := range vals {
		switch val := val.(type) {
		case []byte:
			vals[i] = fmt.Sprintf("[]byte(%d)", len(val))
		case fuse.ReadResult:
			vals[i] = fmt.Sprintf("fuse.ReadResult(%d)", val.Size())
		}
	}
}
