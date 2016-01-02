package adbfs

import "github.com/hanwen/go-fuse/fuse"

type readResultError fuse.Status

var _ fuse.ReadResult = readResultError(fuse.ENOSYS)

func (e readResultError) Bytes(buf []byte) ([]byte, fuse.Status) {
	return nil, fuse.Status(e)
}

func (e readResultError) Size() int {
	return 0
}

func (e readResultError) Done() {
}
