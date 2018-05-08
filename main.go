package main

import (
	"os"

	"code.cloudfoundry.org/groot"
	"github.com/suse/groot-btrfs/driver"
	"github.com/suse/groot-btrfs/hcs"
	"github.com/suse/groot-btrfs/privilege"
	"github.com/suse/groot-btrfs/tarstream"
	"github.com/suse/groot-btrfs/volume"
	"github.com/urfave/cli"
)

func main() {
	driver := driver.New(hcs.NewClient(), tarstream.New(), &privilege.Elevator{}, &volume.Limiter{})

	driverFlags := []cli.Flag{
		cli.StringFlag{
			Name:        "volumes-dir-name",
			Value:       "",
			Usage:       "Volumes directory name",
			Destination: &driver.VolumesDirName,
		},

		cli.StringFlag{
			Name:  "store",
			Value: "",
			Usage: "ignored for backward compatibility with Guardian",
		}}
	groot.Run(driver, os.Args, driverFlags)
}
