package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/zach-klippenstein/goadb"
)

// Permissions for mountpoint directories.
const MountpointPerm os.FileMode = 0700

// When creating directory names from device info, all special characters are replaced
// with single underscores. See mountpoint_test.go for examples.
var dirNameCleanerRegexp = regexp.MustCompilePOSIX(`[^-[:alnum:]]+`)

func NewMountpointForDevice(deviceInfo *adb.DeviceInfo, mountRoot, serial string) (mountpoint string, err error) {
	dirName := buildDirNameForDevice(deviceInfo)
	mountpoint = filepath.Join(mountRoot, dirName)

	if doesFileExist(mountpoint) {
		err = fmt.Errorf("directory exists: %s", serial, mountpoint)
		return
	}

	err = os.Mkdir(mountpoint, MountpointPerm)
	return
}

func doesFileExist(path string) bool {
	_, err := os.Stat(path)
	return err == os.ErrNotExist
}

func buildDirNameForDevice(deviceInfo *adb.DeviceInfo) string {
	rawName := fmt.Sprintf("%s-%s", deviceInfo.Model, deviceInfo.Serial)
	return dirNameCleanerRegexp.ReplaceAllLiteralString(rawName, "_")
}
