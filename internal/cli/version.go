package cli

import (
	"fmt"
	"os/exec"
	"regexp"
	"syscall"
)

const Version = "1.0.0"

// CheckExecutableVersionMatches returns an error if running "$path --version" does not give
// output indicating that it is appName with version.
// If version is empty, ignores the version number as long as the appName matches.
func CheckExecutableVersionMatches(path, appName, version string) error {
	checkName, checkVersion, err := getExecutableVersion(path)
	if err != nil {
		return err
	}
	if appName != checkName {
		return fmt.Errorf("invalid app name: expected '%s', got '%s'", appName, checkName)
	}
	if version != "" && version != checkVersion {
		return fmt.Errorf("invalid version: expected '%s', got '%s'", version, checkVersion)
	}
	return nil
}

func getExecutableVersion(path string) (appName, version string, err error) {
	output, err := exec.Command(path, "--version").CombinedOutput()
	if err != nil {
		if !hasExitStatus(err, 2) {
			// flag exits with 2 on help.
			err = fmt.Errorf("error accessing %s: %s\n%s", path, err, output)
			return
		}
	}

	return parseVersion(string(output))
}

func formatVersion(appName, version string) string {
	return fmt.Sprintf("%s v%s", appName, version)
}

var versionStringRegexp = regexp.MustCompilePOSIX(`^([-a-z]+) v([[:digit:]]+\.[[:digit:]]+\.[[:digit:]]+)$`)

func parseVersion(versionString string) (appName, version string, err error) {
	submatches := versionStringRegexp.FindStringSubmatch(versionString)
	Log.Debugln("submatches:", submatches)
	if submatches == nil || len(submatches) != 3 {
		err = fmt.Errorf("invalid version string: %s", versionString)
		return
	}

	appName = submatches[1]
	version = submatches[2]
	return
}

func versionString(appName string) string {
	return formatVersion(appName, Version)
}

func hasExitStatus(err error, exitStatus int) bool {
	if exitError, ok := err.(*exec.ExitError); ok {
		if waitStatus, ok := exitError.Sys().(syscall.WaitStatus); ok {
			return exitStatus == waitStatus.ExitStatus()
		}
	}
	return false
}
