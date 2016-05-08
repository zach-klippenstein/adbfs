package cli

import "gopkg.in/alecthomas/kingpin.v2"

type AdbfsConfig struct {
	BaseConfig

	DeviceSerial string
	Mountpoint   string
}

const (
	DeviceSerialFlag = "device"
	MountpointFlag   = "mountpoint"
)

func RegisterAdbfsFlags(config *AdbfsConfig) {
	registerBaseFlags(&config.BaseConfig)

	kingpin.Flag(DeviceSerialFlag,
		"Serial number of device to mount.").
		Short('s').
		Required().
		StringVar(&config.DeviceSerial)
	kingpin.Flag(MountpointFlag,
		"Directory to mount the device on.").
		PlaceHolder("/mnt").
		Required().
		StringVar(&config.Mountpoint)
}

func (c *AdbfsConfig) AsArgs() []string {
	return append(c.BaseConfig.AsArgs(),
		formatFlag(DeviceSerialFlag, c.DeviceSerial),
		formatFlag(MountpointFlag, c.Mountpoint),
	)
}
