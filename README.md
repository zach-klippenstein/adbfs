# adbfs [![Build Status](https://travis-ci.org/zach-klippenstein/adbfs.svg?branch=master)](https://travis-ci.org/zach-klippenstein/adbfs) [![GoDoc](https://godoc.org/github.com/zach-klippenstein/adbfs?status.svg)](https://godoc.org/github.com/zach-klippenstein/adbfs/fs)

A FUSE filesystem that uses [goadb](https://github.com/zach-klippenstein/goadb) to expose Android devices' filesystems.

## Quick Start

### Installation

adbfs depends on fuse. For OS X, install osxfuse.
Then run:

```
$ go get github.com/zach-klippenstein/adbfs
$ cd `go list -f '{{.Dir}}' github.com/zach-klippenstein/adbfs`
$ ./install.sh
```

### Let's mount some devices!

The easiest way to start mounting devices is to use `adbfs-automount` to watch for newly-connected devices
and automatically mount them.

```
$ mkdir ~/mnt
$ adbfs-automount
```

You probably want to run this as a service when you login (e.g. create a LaunchAgent on OSX).
For example, on OSX, paste this into `~/Library/LaunchAgents/com.adbfs-automount.plist` (substituting "zach" for your own username, of course):
```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.adbfs-automount</string>
  <key>ProgramArguments</key>
  <array>
    <string>/Users/zach/go/bin/adbfs-automount</string>
    <string>-adbfs=/Users/zach/go/bin/adbfs</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>/Users/zach/adbfs.log</string>
  <key>StandardErrorPath</key>
  <string>/Users/zach/adbfs.log</string>
</dict>
</plist>
```
then run
```
$ launchctl load ~/Library/LaunchAgents/com.adbfs-automount.plist
$ launchctl start com.adbfs-automounter
```

## adfs

### Usage

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

## adbfs-automount

`adbfs-automount` listens for new device connections to adb and runs an instance of `adbfs` for each device to mount
it. Most arguments are passed through to `adbfs`, but there are a few arguments specific to the automounter:

`-root`: the directory under which to mount devices. If this is not specified, it will try to figure out
a good path. On OSX, `~/mnt` is used if it exists, else `/Volumes`. On Linux, it tries `~/mnt` then `/mnt`.

`-adbfs`: path to the adbfs executable to run. If not specified, will search `$PATH`. The executable _must_ be built
from the same SHA as `adbfs-automount`, which will exit with an error if this is not the case.

## Running from Source

Don't use `go` directly. Use the `./go.sh` script instead, which sets the build SHA variable used for
printing version.

```
$ ./go.sh run cmd/adbfs/main.go …
```
