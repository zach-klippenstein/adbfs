/*
Connects to the adb server to listen for new devices, and mounts devices under a certain directory
when connected.
*/
package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"

	"github.com/zach-klippenstein/adbfs/internal/cli"
	"github.com/zach-klippenstein/goadb"
	"golang.org/x/net/context"
)

const appName = "adbfs-automount"

var (
	config cli.AutomountConfig
	server goadb.Server
)

func init() {
	cli.RegisterAutomountFlags(&config)
}

func main() {
	exitCode := mainWithExitCode()
	os.Exit(exitCode)
}

// Allows us to avoid calling os.Exit so we can run deferred functions as normal.
func mainWithExitCode() int {
	cli.Initialize(appName, &config.BaseConfig)
	eventLog := cli.NewEventLog(appName, "")
	defer eventLog.Finish()

	config.InitializePaths()
	eventLog.Infof("using mount root %s", config.MountRoot)

	if config.ReadOnly {
		eventLog.Infof("mounting as read-only filesystem")
	} else {
		eventLog.Infof("mounting as writable filesystem")
	}

	var err error
	server, err = goadb.NewServer(config.ServerConfig())
	if err != nil {
		eventLog.Errorf("error initializing adb server: %s", err)
		return 1
	}

	deviceWatcher := goadb.NewDeviceWatcher(server)
	defer deviceWatcher.Shutdown()

	signals := make(chan os.Signal)
	signal.Notify(signals, os.Kill, os.Interrupt)

	processes := cli.NewProcessTracker()
	defer func() {
		eventLog.Infof("shutting down all mount processes…")
		processes.Shutdown()
		eventLog.Infof("all processes shutdown.")
	}()

	cli.Log.Info("automounter ready.")
	defer cli.Log.Info("exiting.")

	for {
		select {
		case event, ok := <-deviceWatcher.C():
			if !ok {
				// DeviceWatcher gave up.
				eventLog.Errorf("device watcher quit unexpectedly: %s", deviceWatcher.Err())
				return 1
			}

			if event.CameOnline() {
				eventLog.Debugf("device connected: %s", event.Serial)
				processes.Go(event.Serial, mountDevice)
			} else if event.WentOffline() {
				eventLog.Debugf("device disconnected: %s", event.Serial)
			} else {
				eventLog.Debugf("unknown device event: %+v", event)
			}
		case signal := <-signals:
			eventLog.Debugf("got signal %v", signal)
			if signal == os.Kill || signal == os.Interrupt {
				return 0
			}
		}
	}
}

func mountDevice(serial string, context context.Context) {
	eventLog := cli.NewEventLog(appName, "device:"+serial)

	defer func() {
		eventLog.Debugf("device mount process finished: %s", serial)
		eventLog.Finish()
	}()

	adbClient := goadb.NewDeviceClient(server, goadb.DeviceWithSerial(serial))
	deviceInfo, err := adbClient.GetDeviceInfo()
	if err != nil {
		eventLog.Errorf("error getting device info for %s: %s", serial, err)
		return
	}

	mountpoint, err := cli.NewMountpointForDevice(deviceInfo, config.MountRoot, serial)
	if err != nil {
		eventLog.Errorf("error creating mountpoint for %s: %s", serial, err)
		return
	}
	defer RemoveLoggingError(mountpoint)

	eventLog.Infof("mounting %s on %s", serial, mountpoint)
	cmd := NewMountProcess(config.PathToAdbfs, cli.AdbfsConfig{
		BaseConfig:   config.BaseConfig,
		DeviceSerial: serial,
		Mountpoint:   mountpoint,
	})

	eventLog.Debugf("launching adbfs: %s", CommandLine(cmd))
	if err := cmd.Start(); err != nil {
		eventLog.Errorf("error starting adbfs process: %s", err)
		return
	}

	eventLog.Infof("device %s mounted with PID %d", serial, cmd.Process.Pid)

	// If we're told to stop, kill the mount process.
	go func() {
		<-context.Done()
		cmd.Process.Kill()
	}()

	handlerBinding := map[string]string{
		cli.PathHandlerVar:   mountpoint,
		cli.SerialHandlerVar: serial,
		cli.ModelHandlerVar:  deviceInfo.Model,
	}
	cli.FireHandlers(config.OnMountHandlers, handlerBinding)
	defer cli.FireHandlers(config.OnUnmountHandlers, handlerBinding)

	if err := cmd.Wait(); err != nil {
		if err, ok := err.(*exec.ExitError); ok {
			eventLog.Errorf("adbfs exited with %+v", err)
		} else {
			eventLog.Errorf("lost connection with adbfs process:", err)
		}
		return
	}

	eventLog.Infof("mount process for device %s stopped", serial)
}

func RemoveLoggingError(path string) {
	cli.Log.Debugln("removing", path)
	if err := os.Remove(path); err != nil {
		cli.Log.Errorf("error removing %s: %s", path, err)
	} else {
		cli.Log.Debug("removed successfully.")
	}
}

func NewMountProcess(pathToAdbfs string, config cli.AdbfsConfig) *exec.Cmd {
	return &exec.Cmd{
		Path:   pathToAdbfs,
		Args:   config.AsArgs(),
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
}

func CommandLine(cmd *exec.Cmd) string {
	return fmt.Sprintf("%s %s", cmd.Path, strings.Join(cmd.Args, " "))
}
