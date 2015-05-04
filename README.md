# adbfs

[![Build Status](https://travis-ci.org/zach-klippenstein/adbfs.svg)](https://travis-ci.org/zach-klippenstein/adbfs)
[![GoDoc](https://godoc.org/github.com/zach-klippenstein/adbfs?status.svg)](https://godoc.org/github.com/zach-klippenstein/adbfs/fs)

NOTE: Travis build is currently broken, since fusermount doesn't exist on the build nodes. May need to move off Travis to get them working.

A FUSE filesystem that uses [goadb](https://github.com/zach-klippenstein/goadb) to expose Android devices' filesystems.
