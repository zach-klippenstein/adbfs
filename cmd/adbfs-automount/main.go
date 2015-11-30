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
)

var config cli.AutomountConfig

func init() {
	cli.RegisterAutomountFlags(&config)
}

func main() {
	cli.Initialize("adbfs-automount", &config.BaseConfig)

	config.InitializePaths()
	cli.Log.Infoln("using mount root", config.MountRoot)

	deviceWatcher := goadb.NewDeviceWatcher(config.ClientConfig())
	defer deviceWatcher.Shutdown()

	signals := make(chan os.Signal)
	signal.Notify(signals, os.Kill, os.Interrupt)

	processes := cli.NewProcessTracker()
	defer processes.Shutdown()

	cli.Log.Info("automounter ready.")
	defer cli.Log.Info("exiting.")

	for {
		select {
		case event := <-deviceWatcher.C():
			if event.CameOnline() {
				cli.Log.Debugln("device connected:", event.Serial)
				processes.Go(event.Serial, mountDevice)
			}
		case signal := <-signals:
			cli.Log.Debugln("got signal", signal)
			if signal == os.Kill || signal == os.Interrupt {
				cli.Log.Info("shutting down all mount processes…")
				processes.Shutdown()
				cli.Log.Info("all processes shutdown.")
				return
			}
		}
	}
}

func mountDevice(serial string, stop <-chan struct{}) {
	defer func() {
		cli.Log.Debugln("device mount process finished:", serial)
	}()

	adbClient := goadb.NewDeviceClient(config.ClientConfig(), goadb.DeviceWithSerial(serial))
	deviceInfo, err := adbClient.GetDeviceInfo()
	if err != nil {
		cli.Log.Errorf("error getting device info for %s: %s", serial, err)
		return
	}

	mountpoint, err := cli.NewMountpointForDevice(deviceInfo, config.MountRoot, serial)
	if err != nil {
		cli.Log.Errorf("error creating mountpoint for %s: %s", serial, err)
		return
	}
	defer RemoveLoggingError(mountpoint)

	cli.Log.Infof("mounting %s on %s", serial, mountpoint)
	cmd := NewMountProcess(config.PathToAdbfs, cli.AdbfsConfig{
		BaseConfig:   config.BaseConfig,
		DeviceSerial: serial,
		Mountpoint:   mountpoint,
	})

	cli.Log.Debugln("launching adbfs:", CommandLine(cmd))
	if err := cmd.Start(); err != nil {
		cli.Log.Errorln("error starting adbfs process:", err)
		return
	}

	cli.Log.Infof("device %s mounted with PID %d", serial, cmd.Process.Pid)

	// If we're told to stop, kill the mount process.
	go func() {
		<-stop
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
			cli.Log.Errorf("adbfs exited with %+v", err)
		} else {
			cli.Log.Errorf("lost connection with adbfs process:", err)
		}
		return
	}

	cli.Log.Infof("mount process for device %s stopped", serial)
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
