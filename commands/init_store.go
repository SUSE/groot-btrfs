package commands // import "github.com/SUSE/groot-btrfs/commands"

import (
	"fmt"
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/SUSE/groot-btrfs/commands/config"
	"github.com/SUSE/groot-btrfs/groot"
	"github.com/SUSE/groot-btrfs/store/manager"

	errorspkg "github.com/pkg/errors"
	"github.com/urfave/cli"
)

var InitStoreCommand = cli.Command{
	Name:        "init-store",
	Usage:       "init-store --store <path>",
	Description: "Initialize a Store Directory",

	Flags: []cli.Flag{
		cli.StringSliceFlag{
			Name:  "uid-mapping",
			Usage: "UID mapping for image translation, e.g.: <Namespace UID>:<Host UID>:<Size>",
		},
		cli.StringSliceFlag{
			Name:  "gid-mapping",
			Usage: "GID mapping for image translation, e.g.: <Namespace GID>:<Host GID>:<Size>",
		},
		cli.Int64Flag{
			Name:  "store-size-bytes",
			Usage: "Creates a new filesystem of the given size and mounts it to the given Store Directory",
		},
	},

	Action: func(ctx *cli.Context) error {
		logger := ctx.App.Metadata["logger"].(lager.Logger)
		logger = logger.Session("init-store")

		if ctx.NArg() != 0 {
			logger.Error("parsing-command", errorspkg.New("invalid arguments"), lager.Data{"args": ctx.Args()})
			return cli.NewExitError(fmt.Sprintf("invalid arguments - usage: %s", ctx.Command.Usage), 1)
		}

		configBuilder := ctx.App.Metadata["configBuilder"].(*config.Builder).
			WithStoreSizeBytes(ctx.Int64("store-size-bytes"))
		cfg, err := configBuilder.Build()
		logger.Debug("init-store", lager.Data{"currentConfig": cfg})
		if err != nil {
			logger.Error("config-builder-failed", err)
			return cli.NewExitError(err.Error(), 1)
		}

		storePath := cfg.StorePath
		storeSizeBytes := cfg.Init.StoreSizeBytes

		if os.Getuid() != 0 {
			err := errorspkg.Errorf("store %s can only be initialized by Root user", storePath)
			logger.Error("init-store-failed", err)
			return cli.NewExitError(err.Error(), 1)
		}

		fsDriver, err := createFileSystemDriver(cfg)
		if err != nil {
			logger.Error("failed-to-initialise-filesystem-driver", err)
			return cli.NewExitError(err.Error(), 1)
		}

		uidMappings, err := parseIDMappings(ctx.StringSlice("uid-mapping"))
		if err != nil {
			err = errorspkg.Errorf("parsing uid-mapping: %s", err)
			logger.Error("parsing-command", err)
			return cli.NewExitError(err.Error(), 1)
		}
		gidMappings, err := parseIDMappings(ctx.StringSlice("gid-mapping"))
		if err != nil {
			err = errorspkg.Errorf("parsing gid-mapping: %s", err)
			logger.Error("parsing-command", err)
			return cli.NewExitError(err.Error(), 1)
		}

		namespacer := groot.NewStoreNamespacer(storePath)
		spec := manager.InitSpec{
			UIDMappings:    uidMappings,
			GIDMappings:    gidMappings,
			StoreSizeBytes: storeSizeBytes,
		}

		manager := manager.New(storePath, namespacer, fsDriver, fsDriver, fsDriver)
		if err := manager.InitStore(logger, spec); err != nil {
			logger.Error("cleaning-up-store-failed", err)
			return cli.NewExitError(errorspkg.Cause(err).Error(), 1)
		}

		return nil
	},
}
