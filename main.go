package main

import (
	"math/rand"
	"os"
	"time"

	"code.cloudfoundry.org/groot"
	"github.com/SUSE/groot-btrfs/driver"
	"github.com/containers/storage/pkg/reexec"
	"github.com/urfave/cli"
)

func init() {
	rand.Seed(time.Now().UnixNano())
	if reexec.Init() {
		os.Exit(0)
	}
}

func main() {

	driverConfig := &driver.DriverConfig{}

	driverFlags := []cli.Flag{
		cli.StringFlag{
			Name:        "store-path",
			Value:       "",
			Usage:       "Store path",
			Destination: &driverConfig.StorePath,
		},
		cli.StringFlag{
			Name:        "volumes-dir-Name",
			Value:       "volumes",
			Usage:       "Volumes directory name",
			Destination: &driverConfig.VolumesDirName,
		},
		cli.StringFlag{
			Name:        "btrfs-progs-path",
			Value:       "",
			Usage:       "The path to btrfs progs",
			Destination: &driverConfig.BtrfsProgsPath,
		},

		cli.StringFlag{
			Name:        "drax-bin",
			Value:       "",
			Usage:       "The path to the drax cli binary",
			Destination: &driverConfig.DraxBinPath,
		},
		cli.StringSliceFlag{
			Name:  "uid-mapping",
			Usage: "UID mapping for image translation, e.g.: <Namespace UID>:<Host UID>:<Size>",
		},
		cli.StringSliceFlag{
			Name:  "gid-mapping",
			Usage: "GID mapping for image translation, e.g.: <Namespace GID>:<Host GID>:<Size>",
		},
	}

	driver := driver.NewDriver(driverConfig)

	groot.Run(driver, os.Args, driverFlags)
}
