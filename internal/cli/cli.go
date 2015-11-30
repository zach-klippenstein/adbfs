// Command-line options and utilities used by multiple cmds.
package cli

import (
	"fmt"
	"log"

	"github.com/Sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
)

var Log *logrus.Logger

var buildSHA string

func init() {
	kingpin.HelpFlag.Short('h')
}

// Initialize sets the app name. Must be called before flag.Parse()
func Initialize(appName string, baseConfig *BaseConfig) {
	if appName == "" {
		panic("appName cannot be empty")
	}
	kingpin.Version(versionString(appName))

	kingpin.Parse()
	initializeLogger(appName, baseConfig)
}

func BuildSHA() string {
	if buildSHA == "" {
		panic("build SHA not set")
	}
	return buildSHA
}

func initializeLogger(appName string, baseConfig *BaseConfig) {
	Log = baseConfig.createLogger()

	// Redirect standard logger (used by fuse) to our logger.
	log.SetOutput(Log.Writer())
	// Disable standard log timestamps, logrus has its own.
	log.SetFlags(0)

	Log.Println(versionString(appName))
}

func versionString(appName string) string {
	return formatVersion(appName, BuildSHA())
}

func formatFlag(name string, value interface{}) string {
	switch value := value.(type) {
	case bool:
		if value {
			return "--" + name
		}
		return "--no-" + name
	default:
		return fmt.Sprintf("--%s=%v", name, value)
	}
}
