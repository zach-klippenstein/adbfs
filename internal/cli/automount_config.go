package cli

import (
	"os"

	"os/exec"

	"gopkg.in/alecthomas/kingpin.v2"
)

type AutomountConfig struct {
	BaseConfig

	MountRoot     string
	PathToAdbfs   string
	AllowAnyAdbfs bool
}

const (
	MountRootFlag     = "root"
	PathToAdbfsFlag   = "adbfs"
	AllowAnyAdbfsFlag = "disable-adbfs-verify"
)

func RegisterAutomountFlags(config *AutomountConfig) {
	registerBaseFlags(&config.BaseConfig)

	kingpin.Flag(MountRootFlag,
		"Directory in which to mount devices.").Short('r').PlaceHolder("/mnt").ExistingDirVar(&config.MountRoot)
	kingpin.Flag("adbfs",
		"Path to adbfs executable. If not specified, PATH is searched.").PlaceHolder("/usr/bin/adbfs").ExistingFileVar(&config.PathToAdbfs)
	kingpin.Flag(AllowAnyAdbfsFlag,
		"If true, the build SHA of adbfs won't be required to match that of this executable.").Hidden().BoolVar(&config.AllowAnyAdbfs)
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

	err := CheckExecutableVersionMatches(c.PathToAdbfs, "adbfs", BuildSHA())
	if err != nil {
		Log.Fatal(err)
	}
}
