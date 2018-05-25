package main

import (
	"os"

	"github.com/SUSE/groot-btrfs/drax/commands"

	"github.com/urfave/cli"
)

var version string

func main() {
	drax := cli.NewApp()
	drax.Name = "drax"
	drax.Usage = "The destroyer"
	drax.Version = version
	drax.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "btrfs-bin",
			Usage: "Path to btrfs bin. (If not provided will use $PATH)",
			Value: "btrfs",
		},
	}

	drax.Before = func(ctx *cli.Context) error {
		cli.ErrWriter = os.Stdout

		return nil
	}

	drax.Commands = []cli.Command{
		commands.LimitCommand,
		commands.ListCommand,
		commands.DestroyCommand,
		commands.StatsCommand,
	}

	drax.Run(os.Args)
}
