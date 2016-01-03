package adbfs

import (
	"errors"
	"os"
	"syscall"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/zach-klippenstein/goadb/util"
)

const OK = syscall.Errno(0)

var (
	// A symlink cycle is detected.
	ErrLinkTooDeep = errors.New("link recursion too deep")
	ErrNotALink    = errors.New("not a link")
	// The user doesn't have permission to perform an operation.
	ErrNoPermission = os.ErrPermission
	// The operation is not permitted due to reasons other than user permission.
	ErrNotPermitted = errors.New("operation not permitted")
)

// toFuseStatusLog converts an Errno to a Status and logs it.
func toFuseStatusLog(err error, logEntry *LogEntry) fuse.Status {
	return fuse.Status(toErrnoLog(err, logEntry))
}

func fuseStatusToErrno(status fuse.Status) syscall.Errno {
	return syscall.Errno(status)
}

// toErrnoLog converts an error to an Errno and logs it.
func toErrnoLog(err error, logEntry *LogEntry) (status syscall.Errno) {
	status = toErrno(err)
	if status == syscall.EIO {
		logEntry.Error(err)
	}
	return logEntry.Status(status)
}

// toErrno converts a known error to an Errno, or EIO if the error is not known.
func toErrno(err error) syscall.Errno {
	switch {
	case err == nil:
		return OK
	case err == ErrLinkTooDeep:
		return syscall.ELOOP
	case err == ErrNotALink:
		return syscall.EINVAL
	case err == ErrNoPermission || err == os.ErrPermission:
		// See http://blog.unclesniper.org/archives/2-Linux-programmers,-learn-the-difference-between-EACCES-and-EPERM-already!.html
		return syscall.EACCES
	case err == ErrNotPermitted:
		return syscall.EPERM
	case util.HasErrCode(err, util.FileNoExistError):
		return syscall.ENOENT
	}
	if err, ok := err.(syscall.Errno); ok {
		return err
	}
	return syscall.EIO
}
