package adbfs

import "github.com/hanwen/go-fuse/fuse"

// readError returns a ReadResult that will report 0 size and status, and then logs and returns status.
func readError(err error, logEntry *LogEntry) (fuse.ReadResult, fuse.Status) {
	status := toErrno(err)
	return errorReadResult(status), toFuseStatusLog(err, logEntry)
}

type errorReadResult fuse.Status

var _ fuse.ReadResult = errorReadResult(0)

func (e errorReadResult) Bytes(buf []byte) ([]byte, fuse.Status) {
	return nil, fuse.Status(e)
}

func (e errorReadResult) Size() int {
	return 0
}

func (e errorReadResult) Done() {
}
