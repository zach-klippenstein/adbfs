// See package fs for the filesystem implementation.
package main

import (
	"flag"
	"log"
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

func main() {
	flag.Parse()
	log := logrus.StandardLogger()

	logLevel, err := logrus.ParseLevel(*logLevel)
	if err != nil {
		log.Fatal(err)
	}
	log.Level = logLevel

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
	deviceDescriptor := goadb.DeviceWithSerial(*deviceSerial)
	clientFactory := func() fs.DeviceClient {
		return goadb.NewDeviceClient(clientConfig, deviceDescriptor)
	}

	var fsImpl pathfs.FileSystem
	fsImpl, err = fs.NewAdbFileSystem(fs.Config{
		Mountpoint:    absoluteMountpoint,
		ClientFactory: clientFactory,
		Log:           log,
	})
	if err != nil {
		log.Fatal(err)
	}
	// loggingFs := fs.NewLoggingFileSystem(fsImpl, log)

	fs := pathfs.NewPathNodeFs(fsImpl, nil)

	server, _, err := nodefs.MountRoot(absoluteMountpoint, fs.Root(), nil)
	if err != nil {
		log.Fatal(err)
	}
	defer server.Unmount()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, os.Kill)

	serverDone := make(chan struct{})
	go func() {
		server.Serve()
		close(serverDone)
	}()

	serverReady := make(chan struct{})
	go func() {
		server.WaitMount()
		close(serverReady)
	}()

	// Wait for server to come up.
	select {
	case <-serverReady:
		log.Println("filesystem ready")
	case <-serverDone:
		log.Println("server finished prematurely")
	}

	// Wait for server to finish.
	for {
		select {
		case <-serverDone:
			log.Println("unmounted")
			os.Exit(0)
		case signal := <-signals:
			HandleSignal(signal, server)
		}
	}
}

func HandleSignal(signal os.Signal, server *fuse.Server) {
	log.Println("got signal", signal)
	switch signal {
	case os.Kill, os.Interrupt:
		log.Println("unmounting filesystem…")
		server.Unmount()
	}
}

func resolvePathFromWorkingDir(relative string) (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	return filepath.Join(wd, relative), nil
}
