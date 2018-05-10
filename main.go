package main

import (
	"code.cloudfoundry.org/groot"
	"github.com/suse/groot-btrfs/driver"
	"github.com/urfave/cli"
	"os"
	"path/filepath"
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
		}}

	driverConfig.BtrfsBinPath = filepath.Join(btrfsProgsPath, "btrfs")
	driverConfig.MkfsBinPath = filepath.Join(btrfsProgsPath, "mkfs.btrfs")

	driver := driver.NewDriver(driverConfig)

	groot.Run(driver, os.Args, driverFlags)
}
