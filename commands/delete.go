package commands // import "github.com/SUSE/groot-btrfs/commands"

import (
	"fmt"
	"path/filepath"

	"code.cloudfoundry.org/lager"
	"github.com/SUSE/groot-btrfs/commands/config"
	"github.com/SUSE/groot-btrfs/commands/idfinder"
	"github.com/SUSE/groot-btrfs/groot"
	"github.com/SUSE/groot-btrfs/metrics"
	"github.com/SUSE/groot-btrfs/store"
	"github.com/SUSE/groot-btrfs/store/dependency_manager"
	"github.com/SUSE/groot-btrfs/store/garbage_collector"
	"github.com/SUSE/groot-btrfs/store/image_cloner"
	errorspkg "github.com/pkg/errors"
	"github.com/urfave/cli"
)

var DeleteCommand = cli.Command{
	Name:        "delete",
	Usage:       "delete <id|image path>",
	Description: "Deletes a container image",

	Action: func(ctx *cli.Context) error {
		logger := ctx.App.Metadata["logger"].(lager.Logger)
		logger = logger.Session("delete")
		newExitError := newErrorHandler(logger, "delete")

		if ctx.NArg() != 1 {
			logger.Error("parsing-command", errorspkg.New("id was not specified"))
			return newExitError("id was not specified", 1)
		}

		configBuilder := ctx.App.Metadata["configBuilder"].(*config.Builder)
		cfg, err := configBuilder.Build()
		logger.Debug("delete-config", lager.Data{"currentConfig": cfg})
		if err != nil {
			logger.Error("config-builder-failed", err)
			return newExitError(err.Error(), 1)
		}

		storePath := cfg.StorePath
		idOrPath := ctx.Args().First()
		id, err := idfinder.FindID(storePath, idOrPath)
		if err != nil {
			logger.Debug("id-not-found-skipping", lager.Data{"id": idOrPath, "storePath": storePath, "errorMessage": err.Error()})
			fmt.Println(err)
			return nil
		}

		fsDriver, err := createFileSystemDriver(cfg)
		if err != nil {
			logger.Error("failed-to-initialise-filesystem-driver", err)
			return newExitError(err.Error(), 1)
		}

		imageDriver, err := createImageDriver(cfg, fsDriver)
		if err != nil {
			logger.Error("failed-to-initialise-image-driver", err)
			return newExitError(err.Error(), 1)
		}

		imageCloner := image_cloner.NewImageCloner(imageDriver, storePath)
		dependencyManager := dependency_manager.NewDependencyManager(
			filepath.Join(storePath, store.MetaDirName, "dependencies"),
		)
		metricsEmitter := metrics.NewEmitter()
		deleter := groot.IamDeleter(imageCloner, dependencyManager, metricsEmitter)

		sm := store.NewStoreMeasurer(storePath, fsDriver)
		gc := garbage_collector.NewGC(fsDriver, imageCloner, dependencyManager)

		defer func() {
			unusedVols, err := gc.UnusedVolumes(logger, nil)
			if err != nil {
				logger.Error("getting-unused-layers-failed", err)
				return
			}
			metricsEmitter.TryEmitUsage(logger, "UnusedLayersSize", sm.CacheUsage(logger, unusedVols), "bytes")
		}()

		err = deleter.Delete(logger, id)
		if err != nil {
			logger.Error("deleting-image-failed", err)
			return newExitError(err.Error(), 1)
		}

		fmt.Printf("Image %s deleted\n", id)
		metricsEmitter.TryIncrementRunCount("delete", nil)
		return nil
	},
}
