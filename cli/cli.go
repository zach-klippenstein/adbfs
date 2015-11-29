// Command-line options and utilities used by multiple cmds.
package cli

import (
	"fmt"
	stdlog "log"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/zach-klippenstein/goadb"
	"gopkg.in/alecthomas/kingpin.v2"
)

type AdbfsConfig struct {
	// Command-line arguments. Each variable in this block should have a line in AsArgs().
	AdbPort            int
	ConnectionPoolSize int
	LogLevel           string
	CacheTtl           time.Duration
	ServeDebug         bool

	Logger *logrus.Logger

	appName string
	verbose bool
}

var Config AdbfsConfig

var buildSHA string

const (
	defaultPoolSize = 2
	defaultCacheTtl = 300 * time.Millisecond
	defaultLogLevel = logrus.InfoLevel
)

func init() {
	if buildSHA == "" {
		panic("no build SHA set.")
	}

	kingpin.HelpFlag.Short('h')
	kingpin.Flag("port", "Port to connect to adb server on.").Default(strconv.Itoa(goadb.AdbPort)).IntVar(&Config.AdbPort)
	kingpin.Flag("pool", "Size of the connection pool. Not used for open files.").Default(strconv.Itoa(defaultPoolSize)).IntVar(&Config.ConnectionPoolSize)
	kingpin.Flag("cachettl", "Duration to keep cached file info.").Default(defaultCacheTtl.String()).DurationVar(&Config.CacheTtl)
	kingpin.Flag("debug", "If set, will start an HTTP server to expose profiling and trace logs. Off by default.").BoolVar(&Config.ServeDebug)

	logLevels := []string{
		logrus.PanicLevel.String(),
		logrus.FatalLevel.String(),
		logrus.ErrorLevel.String(),
		logrus.WarnLevel.String(),
		logrus.InfoLevel.String(),
		logrus.DebugLevel.String(),
	}
	kingpin.Flag("log", fmt.Sprintf("Detail of logs to show. Options are: %v", logLevels)).Default(defaultLogLevel.String()).EnumVar(&Config.LogLevel, logLevels...)
	kingpin.Flag("verbose", "Alias for --log=debug.").Short('v').BoolVar(&Config.verbose)
}

// AsArgs returns a string array suitable to be passed to exec.Command that copies
// the arguments defined in this package.
func (c *AdbfsConfig) AsArgs() []string {
	return []string{
		fmt.Sprintf("--port=%v", c.AdbPort),
		fmt.Sprintf("--pool=%v", c.ConnectionPoolSize),
		fmt.Sprintf("--log=%v", c.LogLevel),
		fmt.Sprintf("--cachettl=%v", c.CacheTtl),
		FormatBoolFlag("debug", c.ServeDebug),
	}
}

// FormatBoolFlag returns a string that will set the flag name to value when passed as a command line arg.
func FormatBoolFlag(name string, value bool) string {
	if value {
		return "--" + name
	}
	return "--no-" + name
}

// Initialize sets the app name. Must be called before flag.Parse()
func Initialize(appName string) {
	if Config.appName != "" {
		panic("trying to initialize twice")
	}
	Config.appName = appName
	kingpin.Version(Config.versionString())

	kingpin.Parse()
	Config.initializeLogger()
}

// ClientConfig returns a goadb.ClientConfig from CLI arguments.
func (c *AdbfsConfig) ClientConfig() goadb.ClientConfig {
	return goadb.ClientConfig{
		Dialer: goadb.NewDialer("", c.AdbPort),
	}
}

// CheckExecutableSameVersion logs an error and exits if the executable at path doesn't call itself
// appName and have the same build SHA as the calling executable.
func CheckExecutableSameBuildSHA(appName, path string) {
	expectedVersion := versionStringForApp(appName)

	checkOutput, err := exec.Command(path, "--version").CombinedOutput()
	if err != nil {
		if !hasExitStatus(err, 2) {
			// flag exits with 2 on help.
			Config.Logger.Fatalln("error accessing adbfs:", err)
		}
	}

	version := strings.TrimSuffix(string(checkOutput), "\n")
	Config.Logger.Debugln("found", version)
	if version != expectedVersion {
		Config.Logger.Fatalf("%s executable not recognized: expected '%s', found '%s'", appName, expectedVersion, version)
	}
}

func (c *AdbfsConfig) versionString() string {
	if c.appName == "" {
		panic("app name not set")
	}

	return versionStringForApp(c.appName)
}

func versionStringForApp(appName string) string {
	return fmt.Sprintf("%s v%s", appName, buildSHA)
}

func hasExitStatus(err error, exitStatus int) bool {
	if exitError, ok := err.(*exec.ExitError); ok {
		if waitStatus, ok := exitError.Sys().(syscall.WaitStatus); ok {
			return exitStatus == waitStatus.ExitStatus()
		}
	}
	return false
}

func (c *AdbfsConfig) initializeLogger() {
	log := logrus.StandardLogger()
	c.Logger = log

	if c.verbose {
		log.Level = logrus.DebugLevel
	} else {
		logLevel, err := logrus.ParseLevel(c.LogLevel)
		if err != nil {
			log.Fatal(err)
		}
		log.Level = logLevel
	}

	log.Formatter = &logrus.TextFormatter{
		FullTimestamp: true,
		// RFC 3339 with milliseconds.
		TimestampFormat: "2006-01-02T15:04:05.000000000Z07:00",
	}

	// Redirect standard logger (used by fuse) to our logger.
	stdlog.SetOutput(log.Writer())
	// Disable standard log timestamps, logrus has its own.
	stdlog.SetFlags(0)

	c.logHeader()
}

func (c *AdbfsConfig) logHeader() {
	if c.appName == "" {
		panic("app name not set")
	}
	c.Logger.Infof(c.versionString())
}
