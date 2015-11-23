/*
Connects to the adb server to listen for new devices, and mounts devices under a certain directory
when connected.
*/
package main

import (
	"flag"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"io"

	"fmt"
	"regexp"

	"path/filepath"

	"github.com/Sirupsen/logrus"
	"github.com/zach-klippenstein/adbfs/cli"
	"github.com/zach-klippenstein/goadb"
)

var (
	mountRoot = flag.String("root", "", "directory in which to mount devices")
	adbfsPath = flag.String("adbfs", "", "path to adbfs executable. If not specified, PATH is searched.")

	log *logrus.Logger
)

type Config struct {
	adbfsBaseCommand exec.Cmd
	mountRoot        string

	// Permissions for mountpoint directories.
	mountpointPerm os.FileMode
}

// When creating directory names from device info, all special characters are replaced
// with single underscores. See main_test.go for examples.
var dirNameCleanerRegexp = regexp.MustCompilePOSIX(`[^-[:alnum:]]+`)

func main() {
	cli.Initialize("adbfs-automount")
	flag.Parse()
	log = cli.Config.Logger()

	config := &Config{
		adbfsBaseCommand: initializeAdbfsCommand(*adbfsPath),
		mountRoot:        initializeMountRoot(*mountRoot),
		mountpointPerm:   0700,
	}

	deviceWatcher := goadb.NewDeviceWatcher(cli.Config.ClientConfig())
	defer deviceWatcher.Shutdown()

	log.Info("automounter ready.")

	for {
		select {
		case event := <-deviceWatcher.C():
			if event.CameOnline() {
				log.Debugln("device connected:", event.Serial)
				go mountDevice(config, event.Serial)
			}
		}
	}
}

func initializeAdbfsCommand(path string) exec.Cmd {
	if path == "" {
		var err error
		path, err = exec.LookPath("adbfs")
		if err != nil {
			log.Fatalln("couldn't find adbfs executable in PATH.", err)
		}
	}

	log.Debugln("trying to use adbfs at", path)

	// Make sure we've got something that looks like adbfs with the right version.
	expectedVersion := cli.Config.VersionStringForApp("adbfs")
	checkOutput, err := exec.Command(path, "-h").CombinedOutput()
	if err != nil {
		if !hasExitStatus(err, 2) {
			// flag exits with 2 on help.
			log.Fatalln("error accessing adbfs:", err)
		}
	}

	version := strings.Split(string(checkOutput), "\n")[0]
	log.Debugln("found", version)
	if version != expectedVersion {
		log.Fatalf("adbfs executable doesn't look like adbfs: expected '%s', found '%s'", expectedVersion, version)
	}

	log.Infoln("using adbfs executable", path)

	return exec.Cmd{
		Path:   path,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
}

func initializeMountRoot(path string) string {
	if path == "" {
		log.Debug("no mount root specified, falling back to default…")
		path = cli.FindDefaultMountRoot()
	}

	validateMountRoot(path)
	log.Infoln("using mount root", path)
	return path
}

func validateMountRoot(path string) {
	info, err := os.Stat(path)
	if err != nil {
		log.Fatalln("could not read mount root", path, ":", err)
	}
	if !info.IsDir() {
		log.Fatalln(path, "is not a directory")
	}

	dir, err := os.Open(path)
	if err != nil {
		log.Fatalf("could not read mount root %s: %s", path, err)
	}

	// Only care if there are >0 entries, so don't read them all.
	entries, err := dir.Readdirnames(1)
	if err != nil && err != io.EOF {
		log.Fatalln("could not read mount root", path, ":", err)
	}
	if len(entries) != 0 {
		log.Warnln("mount root", path, "is not empty, is another instance already running?")
	}
}

func mountDevice(config *Config, serial string) {
	defer func() {
		log.Debugln("device mount process finished:", serial)
	}()

	deviceMountpoint, err := createMountpointForDevice(config, serial)
	if err != nil {
		log.Errorf("error creating mountpoint for %s: %s", serial, err)
		return
	}
	defer removeMountpoint(deviceMountpoint)

	log.Infof("mounting %s on %s", serial, deviceMountpoint)
	cmd := config.buildMountCommandForDevice(deviceMountpoint, serial)

	log.Debugf("launching adbfs: %s %s", cmd.Path, strings.Join(cmd.Args, " "))
	if err = cmd.Start(); err != nil {
		log.Errorln("error starting adbfs process:", err)
		return
	}

	log.Infof("device %s mounted with PID %d", serial, cmd.Process.Pid)

	if err = cmd.Wait(); err != nil {
		if err, ok := err.(*exec.ExitError); ok {
			log.Errorf("adbfs exited with %+v", err)
		} else {
			log.Errorf("lost connection with adbfs process:", err)
		}
		return
	}

	log.Infof("mount process for device %s stopped", serial)
}

func createMountpointForDevice(config *Config, serial string) (mountpoint string, err error) {
	adbClient := goadb.NewDeviceClient(cli.Config.ClientConfig(), goadb.DeviceWithSerial(serial))
	deviceInfo, err := adbClient.GetDeviceInfo()
	if err != nil {
		return
	}

	dirName := buildDirNameForDevice(deviceInfo)
	mountpoint = filepath.Join(config.mountRoot, dirName)

	if doesFileExist(mountpoint) {
		err = fmt.Errorf("directory exists: %s", serial, mountpoint)
		return
	}

	log.Debugf("creating %s with permissions %s", mountpoint, config.mountpointPerm)
	err = os.Mkdir(mountpoint, config.mountpointPerm)
	return
}

func removeMountpoint(path string) {
	log.Debugln("removing mountpoint", path)
	if err := os.Remove(path); err != nil {
		log.Errorf("error removing mountpoint %s: %s", path, err)
	} else {
		log.Debugln("mountpoint removed successfully.")
	}
}

func doesFileExist(path string) bool {
	_, err := os.Stat(path)
	return err == os.ErrNotExist
}

func buildDirNameForDevice(deviceInfo *goadb.DeviceInfo) string {
	rawName := fmt.Sprintf("%s-%s", deviceInfo.Model, deviceInfo.Serial)
	return dirNameCleanerRegexp.ReplaceAllLiteralString(rawName, "_")
}

func (c *Config) buildMountCommandForDevice(mountpoint, serial string) (cmd exec.Cmd) {
	// Copy the base command, don't mutate it.
	cmd = c.adbfsBaseCommand
	cmd.Args = append(cli.Config.AsArgs(),
		fmt.Sprintf("-device=%s", serial),
		fmt.Sprintf("-mountpoint=%s", mountpoint))
	return
}

func hasExitStatus(err error, exitStatus int) bool {
	if exitError, ok := err.(*exec.ExitError); ok {
		if waitStatus, ok := exitError.Sys().(syscall.WaitStatus); ok {
			return exitStatus == waitStatus.ExitStatus()
		}
	}
	return false
}
