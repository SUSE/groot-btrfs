package main

import (
	"os"
	"path/filepath"

	"github.com/SUSE/groot-btrfs/driver"

	"code.cloudfoundry.org/groot"
	"github.com/urfave/cli"
)

func main() {
	var btrfsProgsPath string

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
			Destination: &btrfsProgsPath,
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

	driverConfig.BtrfsBinPath = filepath.Join(btrfsProgsPath, "btrfs")
	driverConfig.MkfsBinPath = filepath.Join(btrfsProgsPath, "mkfs.btrfs")
	driver := driver.NewDriver(driverConfig)

	groot.Run(driver, os.Args, driverFlags)
}
