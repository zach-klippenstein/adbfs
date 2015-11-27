// Common command-line options.
package cli

import (
	"flag"
	"fmt"
	stdlog "log"
	"os"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/zach-klippenstein/goadb"
)

type AdbfsConfig struct {
	// Command-line arguments. Each variable in this block should have a line in AsArgs().
	AdbPort            int
	ConnectionPoolSize int
	LogLevel           string
	CacheTtl           time.Duration
	ServeDebug         bool

	appName string
}

var Config AdbfsConfig

var buildSHA string

func init() {
	if buildSHA == "" {
		panic("no build SHA set.")
	}

	flag.IntVar(&Config.AdbPort, "port", goadb.AdbPort, "Port to connect to adb server on.")
	flag.IntVar(&Config.ConnectionPoolSize, "poolsize", 2, "Size of the connection pool. Not used for open files.")
	flag.StringVar(&Config.LogLevel, "loglevel", "info", "Detail of logs to show.")
	flag.DurationVar(&Config.CacheTtl, "cachettl", 300*time.Millisecond, "Duration to keep cached file info.")
	flag.BoolVar(&Config.ServeDebug, "debug", false, "If true, will start an HTTP server to expose profiling and trace logs.")

	// Add the version string to the usage message.
	oldUsage := flag.Usage
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, Config.VersionString())
		oldUsage()
	}
}

// BuildSHA is set by the install script at build time.
func BuildSHA() string {
	return buildSHA
}

// Initialize sets the app name. Must be called before flag.Parse()
func Initialize(appName string) {
	if Config.appName != "" {
		panic("trying to initialize twice")
	}
	Config.appName = appName
}

// AsArgs returns a string array suitable to be passed to exec.Command that copies
// the arguments defined in this package.
func (c *AdbfsConfig) AsArgs() []string {
	return []string{
		fmt.Sprintf("-port=%v", c.AdbPort),
		fmt.Sprintf("-poolsize=%v", c.ConnectionPoolSize),
		fmt.Sprintf("-loglevel=%v", c.LogLevel),
		fmt.Sprintf("-cachettl=%v", c.CacheTtl),
		fmt.Sprintf("-debug=%v", c.ServeDebug),
	}
}

// ClientConfig returns a goadb.ClientConfig from CLI arguments.
func (c *AdbfsConfig) ClientConfig() goadb.ClientConfig {
	return goadb.ClientConfig{
		Dialer: goadb.NewDialer("", c.AdbPort),
	}
}

func (c *AdbfsConfig) Logger() *logrus.Logger {
	log := logrus.StandardLogger()

	logLevel, err := logrus.ParseLevel(c.LogLevel)
	if err != nil {
		log.Fatal(err)
	}
	log.Level = logLevel

	log.Formatter = &logrus.TextFormatter{
		FullTimestamp: true,
		// RFC 3339 with milliseconds.
		TimestampFormat: "2006-01-02T15:04:05.000000000Z07:00",
	}

	// Redirect standard logger (used by fuse) to our logger.
	stdlog.SetOutput(log.Writer())
	// Disable standard log timestamps, logrus has its own.
	stdlog.SetFlags(0)

	c.logHeader(log)

	return log
}

func (c *AdbfsConfig) VersionString() string {
	if c.appName == "" {
		panic("app name not set")
	}

	return c.VersionStringForApp(c.appName)
}

func (c *AdbfsConfig) VersionStringForApp(appName string) string {
	return fmt.Sprintf("%s v%s", appName, buildSHA)
}

func (c *AdbfsConfig) logHeader(log *logrus.Logger) {
	if c.appName == "" {
		panic("app name not set")
	}

	log.Infof(c.VersionString())
}
