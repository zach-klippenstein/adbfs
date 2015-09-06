/*
Another FUSE filesystem that can mount any device visible to your adb server.
Uses github.com/zach-klippenstein/goadb to interface with the server directly
instead of calling out to the adb client program.

See package fs for the filesystem implementation.
*/
package main

import (
	"flag"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/Sirupsen/logrus"
	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
	"github.com/zach-klippenstein/adbfs/fs"
	"github.com/zach-klippenstein/goadb"
)

var (
	deviceSerial = flag.String("device", "", "Device serial number to mount.")
	mountpoint   = flag.String("mountpoint", "", "Directory to mount the device on.")
	adbPort      = flag.Int("port", goadb.AdbPort, "Port to connect to adb server on.")
	logLevel     = flag.String("loglevel", "info", "Detail of logs to show.")
)

var (
	server    *fuse.Server
	log       *logrus.Logger
	unmounted bool
)

func main() {
	flag.Parse()
	initializeLogger()

	if *mountpoint == "" {
		log.Fatalln("Mountpoint must be specified. Run with -h.")
	}
	absoluteMountpoint, err := resolvePathFromWorkingDir(*mountpoint)
	if err != nil {
		log.Fatal(err)
	}

	clientConfig := goadb.ClientConfig{
		Dialer: goadb.NewDialer("", *adbPort),
	}

	fs := initializeFileSystem(clientConfig, absoluteMountpoint, handleDeviceDisconnected)

	server, _, err = nodefs.MountRoot(absoluteMountpoint, fs.Root(), nil)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("mounted %s on %s", *deviceSerial, absoluteMountpoint)
	defer unmountServer()

	serverDone := startServer()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, os.Kill)

	deviceWatcher, _ := goadb.NewDeviceWatcher(clientConfig)

	for {
		select {
		case event := <-deviceWatcher.C():
			if event.Serial == *deviceSerial && event.NewState == "offline" {
				handleDeviceDisconnected()
			}

		case signal := <-signals:
			log.Println("got signal", signal)
			switch signal {
			case os.Kill, os.Interrupt:
				log.Println("exiting...")
				return
			}

		case <-serverDone:
			return
		}
	}
}

func initializeLogger() {
	log = logrus.StandardLogger()

	logLevel, err := logrus.ParseLevel(*logLevel)
	if err != nil {
		log.Fatal(err)
	}
	log.Level = logLevel

	log.Formatter = &logrus.TextFormatter{
		FullTimestamp: true,
	}

	return
}

func initializeFileSystem(clientConfig goadb.ClientConfig, mountpoint string, deviceNotFoundHandler func()) *pathfs.PathNodeFs {
	clientFactory := fs.NewGoadbDeviceClientFactory(clientConfig, *deviceSerial)

	var fsImpl pathfs.FileSystem
	fsImpl, err := fs.NewAdbFileSystem(fs.Config{
		Mountpoint:    mountpoint,
		ClientFactory: clientFactory,
		Log:           log,
		DeviceNotFoundHandler: deviceNotFoundHandler,
	})
	if err != nil {
		log.Fatal(err)
	}

	return pathfs.NewPathNodeFs(fsImpl, nil)
}

func startServer() <-chan struct{} {
	serverDone := make(chan struct{}, 1)
	go func() {
		defer close(serverDone)
		server.Serve()
		log.Println("server finished.")
	}()

	// Wait for OS to finish initializing the mount.
	server.WaitMount()

	log.Println("server ready.")

	return serverDone
}

func unmountServer() {
	if server == nil {
		panic("attempted to unmount server before creating it")
	}

	if !unmounted {
		unmounted = true
		log.Println("unmounting...")
		server.Unmount()
		log.Println("unmounted.")
	}
}

func handleDeviceDisconnected() {
	// Server guaranteed to be non-nil by now, since we can't detect
	// device disconnection without a filesystem op, which can't happen
	// until the server has started.
	log.Infoln("device disconnected, unmounting...")
	unmountServer()
}

func resolvePathFromWorkingDir(relative string) (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	return filepath.Join(wd, relative), nil
}
