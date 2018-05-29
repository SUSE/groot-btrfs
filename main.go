package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"code.cloudfoundry.org/groot"
	"github.com/SUSE/groot-btrfs/driver"
	"github.com/containers/storage/pkg/reexec"
	"github.com/urfave/cli"
)

var version string

func init() {
	rand.Seed(time.Now().UnixNano())
	if reexec.Init() {
		os.Exit(0)
	}
}

func main() {
	// TODO: ask upstream to expose version or the entire urfave/cli App object
	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Fprintf(c.App.Writer, "groot-btrfs version %v\n", version)
	}

	driverConfig := &driver.Config{}

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
			Name:        "metron-endpoint",
			Usage:       "Metron endpoint used to send metrics",
			Value:       "",
			Destination: &driverConfig.MetronEndpoint,
		},
		cli.Int64Flag{
			Name:        "threshold-bytes",
			Usage:       "Disk usage of the store directory at which cleanup should trigger",
			Destination: &driverConfig.CleanThresholdBytes,
		},
		cli.StringFlag{
			Name:        "drax-bin",
			Value:       "",
			Usage:       "The path to the drax cli binary",
			Destination: &driverConfig.DraxBinPath,
		},
	}

	driver := driver.NewDriver(driverConfig)

	groot.Run(driver, os.Args, driverFlags)
}
