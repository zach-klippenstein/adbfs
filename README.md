# adbfs [![GoDoc](https://godoc.org/github.com/zach-klippenstein/adbfs?status.svg)](https://godoc.org/github.com/zach-klippenstein/adbfs/fs)

NOTE: Travis build is currently broken, since fusermount doesn't exist on the build nodes. May need to move off Travis to get them working.

A FUSE filesystem that uses [goadb](https://github.com/zach-klippenstein/goadb) to expose Android devices' filesystems.

## Installation

adbfs depends on fuse. For OS X, install osxfuse.
Then run:

`go get github.com/zach-klippenstein/adbfs`

## Usage

Devices are specified by serial number. To list the serial numbers of all connected devices, run:

`adb devices -l`

The serial number is the left-most column. To mount a device with serial number `02b5c5a809117c73` on `/mnt`, run:

`adbfs -device 02b5c5a809117c73 -mountpoint /mnt`

Example:
```
$ adb devices -l
List of devices attached 
02b5c5a809117c73       device usb:14100000 product:hammerhead model:Nexus_5 device:hammerhead
$ mkdir ~/mnt
$ adbfs -device 02b5c5a809117c73 -mountpoint ~/mnt
INFO[2015-09-07T16:13:03.386813059-07:00] stat cache ttl: 300ms
INFO[2015-09-07T16:13:03.387113547-07:00] connection pool size: 2
INFO[2015-09-07T16:13:03.400838775-07:00] server ready.
INFO[2015-09-07T16:13:03.400884026-07:00] mounted 02b5c5a809117c73 on /Users/zach/mnt
⋮
```
