package cli

import (
	"fmt"
	"os"
	"os/exec"

	"gopkg.in/alecthomas/kingpin.v2"
)

type AutomountConfig struct {
	BaseConfig

	MountRoot         string
	PathToAdbfs       string
	AllowAnyAdbfs     bool
	OnMountHandlers   []string
	OnUnmountHandlers []string
}

const (
	MountRootFlag        = "root"
	PathToAdbfsFlag      = "adbfs"
	AllowAnyAdbfsFlag    = "disable-adbfs-verify"
	OnMountHandlerFlag   = "on-mount"
	OnUnmountHandlerFlag = "on-unmount"
)

func RegisterAutomountFlags(config *AutomountConfig) {
	registerBaseFlags(&config.BaseConfig)

	kingpin.Flag(MountRootFlag,
		"Directory in which to mount devices.").
		Short('r').
		PlaceHolder("/mnt").
		ExistingDirVar(&config.MountRoot)
	kingpin.Flag("adbfs",
		"Path to adbfs executable. If not specified, PATH is searched.").
		PlaceHolder("/usr/bin/adbfs").
		ExistingFileVar(&config.PathToAdbfs)
	kingpin.Flag(AllowAnyAdbfsFlag,
		"If true, the build SHA of adbfs won't be required to match that of this executable.").
		Hidden().
		BoolVar(&config.AllowAnyAdbfs)
	kingpin.Flag(OnMountHandlerFlag,
		`Command(s) to run after a device is mounted. May be repeated.
The following environment variables will be defined:
`+describeHandlerVars()).
		PlaceHolder(fmt.Sprintf(`"open $%s"`, PathHandlerVar)).
		StringsVar(&config.OnMountHandlers)
	kingpin.Flag(OnUnmountHandlerFlag,
		`Command(s) to run after a device has been unmounted. May be repeated.
The following environment variables will be defined:
`+describeHandlerVars()).
		PlaceHolder(fmt.Sprintf(`"say unmounted $%s"`, ModelHandlerVar)).
		StringsVar(&config.OnUnmountHandlers)
}

func (c *AutomountConfig) InitializePaths() {
	c.initializeMountRoot()
	c.initializeAdbfs()
}

func (c *AutomountConfig) initializeMountRoot() {
	if c.MountRoot == "" {
		Log.Debug("no mount root specified, falling back to default…")
		c.MountRoot = FindDefaultMountRoot()
	}

	if c.MountRoot == "" {
		Log.Fatalln("no mount root specified.")
	}

	info, err := os.Stat(c.MountRoot)
	if err != nil {
		Log.Fatalf("could not read mount root %s: %s", c.MountRoot, err)
	}
	if !info.IsDir() {
		Log.Fatalln(c.MountRoot, "is not a directory")
	}
}

func (c *AutomountConfig) initializeAdbfs() {
	if c.PathToAdbfs == "" {
		var err error
		c.PathToAdbfs, err = exec.LookPath("adbfs")
		if err != nil {
			Log.Fatalln("couldn't find adbfs executable in PATH:", err)
		}
	}

	var expectedVersion string
	if c.AllowAnyAdbfs {
		expectedVersion = ""
	} else {
		expectedVersion = Version
	}

	err := CheckExecutableVersionMatches(c.PathToAdbfs, "adbfs", expectedVersion)
	if err != nil {
		Log.Fatal(err)
	}
}
