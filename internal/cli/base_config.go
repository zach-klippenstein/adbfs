package cli

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/zach-klippenstein/goadb"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	DefaultPoolSize = 2
	DefaultCacheTtl = 300 * time.Millisecond
	DefaultLogLevel = logrus.InfoLevel
)

type BaseConfig struct {
	// Command-line arguments. Each variable in this block should have a line in AsArgs().
	AdbPort            int
	ConnectionPoolSize int
	LogLevel           string
	Verbose            bool
	CacheTtl           time.Duration
	ServeDebug         bool
}

const (
	AdbPortFlag            = "port"
	ConnectionPoolSizeFlag = "pool"
	CacheTtlFlag           = "cachettl"
	LogLevelFlag           = "log"
	VerboseFlag            = "verbose"
	ServeDebugFlag         = "debug"
)

func registerBaseFlags(config *BaseConfig) {
	kingpin.Flag(AdbPortFlag, "Port to connect to adb server on.").Default(strconv.Itoa(goadb.AdbPort)).IntVar(&config.AdbPort)
	kingpin.Flag(ConnectionPoolSizeFlag, "Size of the connection pool. Not used for open files.").Default(strconv.Itoa(DefaultPoolSize)).IntVar(&config.ConnectionPoolSize)
	kingpin.Flag(CacheTtlFlag, "Duration to keep cached file info.").Default(DefaultCacheTtl.String()).DurationVar(&config.CacheTtl)
	kingpin.Flag(ServeDebugFlag, "If set, will start an HTTP server to expose profiling and trace logs. Off by default.").BoolVar(&config.ServeDebug)

	logLevels := []string{
		logrus.PanicLevel.String(),
		logrus.FatalLevel.String(),
		logrus.ErrorLevel.String(),
		logrus.WarnLevel.String(),
		logrus.InfoLevel.String(),
		logrus.DebugLevel.String(),
	}
	kingpin.Flag(LogLevelFlag, fmt.Sprintf("Detail of logs to show. Options are: %v", logLevels)).Default(DefaultLogLevel.String()).EnumVar(&config.LogLevel, logLevels...)
	kingpin.Flag(VerboseFlag, "Alias for --log=debug.").Short('v').BoolVar(&config.Verbose)
}

// AsArgs returns a string array suitable to be passed to exec.Command that copies
// the arguments defined in this package.
func (c *BaseConfig) AsArgs() []string {
	return []string{
		formatFlag(AdbPortFlag, c.AdbPort),
		formatFlag(ConnectionPoolSizeFlag, c.ConnectionPoolSize),
		formatFlag(LogLevelFlag, c.LogLevel),
		formatFlag(CacheTtlFlag, c.CacheTtl),
		formatFlag(ServeDebugFlag, c.ServeDebug),
		formatFlag(VerboseFlag, c.Verbose),
	}
}

// ClientConfig returns a goadb.ClientConfig from CLI arguments.
func (c *BaseConfig) ClientConfig() goadb.ClientConfig {
	return goadb.ClientConfig{
		Dialer: goadb.NewDialer("", c.AdbPort),
	}
}

func (c *BaseConfig) createLogger() *logrus.Logger {
	log := logrus.StandardLogger()

	if c.Verbose {
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

	return log
}
