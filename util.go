package adbfs

import (
	"bytes"
	"fmt"
	"os"
	"sync/atomic"

	"github.com/Sirupsen/logrus"
	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/zach-klippenstein/goadb"
)

type AtomicBool int32

func (b *AtomicBool) Value() bool {
	return atomic.LoadInt32((*int32)(b)) != 0
}

func (b *AtomicBool) CompareAndSwap(oldVal, newVal bool) (swapped bool) {
	var oldIntVal int32 = 0
	if oldVal {
		oldIntVal = 1
	}
	var newIntVal int32 = 0
	if newVal {
		newIntVal = 1
	}
	return atomic.CompareAndSwapInt32((*int32)(b), oldIntVal, newIntVal)
}

// asFuseDirEntries reads directory entries from a goadb DirEntries and returns them as a
// list of fuse DirEntry objects.
func asFuseDirEntries(entries []*goadb.DirEntry) (result []fuse.DirEntry) {
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
func asFuseAttr(entry *goadb.DirEntry) *fuse.Attr {
	return &fuse.Attr{
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
func newLoggingFile(file nodefs.File, log *logrus.Logger) nodefs.File {
	formatList := func(list []interface{}) string {
		if len(list) == 0 {
			return ""
		}

		var buffer bytes.Buffer
		buffer.WriteRune('[')
		for i, item := range list {
			fmt.Fprintf(&buffer, "%#v", item)

			if i < len(list)-1 {
				buffer.WriteString(", ")
			}
		}
		buffer.WriteRune(']')
		return buffer.String()
	}

	return &WrappingFile{
		File: file,
		BeforeCall: func(f *WrappingFile, method string, args ...interface{}) interface{} {
			summarizeForLog(args)
			return StartFileOperation(method, formatList(args))
		},
		AfterCall: func(f *WrappingFile, call interface{}, status *fuse.Status, results ...interface{}) {
			summarizeForLog(results)

			logEntry := call.(*LogEntry)
			if status != nil {
				logEntry.Status(*status)
			}
			logEntry.Result(formatList(results))
			logEntry.FinishOperation(log)
		},
	}
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
