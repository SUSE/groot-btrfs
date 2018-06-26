package commands // import "github.com/SUSE/groot-btrfs/commands"

import (
	"encoding/json"
	"fmt"
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/SUSE/groot-btrfs/commands/config"
	"github.com/SUSE/groot-btrfs/commands/idfinder"
	"github.com/SUSE/groot-btrfs/groot"
	"github.com/SUSE/groot-btrfs/metrics"
	imageClonerpkg "github.com/SUSE/groot-btrfs/store/image_cloner"
	errorspkg "github.com/pkg/errors"
	"github.com/urfave/cli"
)

var StatsCommand = cli.Command{
	Name:        "stats",
	Usage:       "stats [options] <id|image path>",
	Description: "Return filesystem stats",

	Action: func(ctx *cli.Context) error {
		logger := ctx.App.Metadata["logger"].(lager.Logger)
		logger = logger.Session("stats")
		newExitError := newErrorHandler(logger, "stats")

		if ctx.NArg() != 1 {
			logger.Error("parsing-command", errorspkg.New("invalid arguments"), lager.Data{"args": ctx.Args()})
			return newExitError(fmt.Sprintf("invalid arguments - usage: %s", ctx.Command.Usage), 1)
		}

		configBuilder := ctx.App.Metadata["configBuilder"].(*config.Builder)
		cfg, err := configBuilder.Build()
		logger.Debug("stats-config", lager.Data{"currentConfig": cfg})
		if err != nil {
			logger.Error("config-builder-failed", err)
			return newExitError(err.Error(), 1)
		}

		storePath := cfg.StorePath
		idOrPath := ctx.Args().First()
		id, err := idfinder.FindID(storePath, idOrPath)
		if err != nil {
			logger.Error("find-id-failed", err, lager.Data{"id": idOrPath, "storePath": storePath})
			return newExitError(err.Error(), 1)
		}

		fsDriver, err := createFileSystemDriver(cfg)
		if err != nil {
			return newExitError(err.Error(), 1)
		}
		imageCloner := imageClonerpkg.NewImageCloner(fsDriver, storePath)

		metricsEmitter := metrics.NewEmitter()
		statser := groot.IamStatser(imageCloner, metricsEmitter)
		stats, err := statser.Stats(logger, id)
		if err != nil {
			logger.Error("fetching-stats", err)
			return newExitError(err.Error(), 1)
		}

		_ = json.NewEncoder(os.Stdout).Encode(stats)
		metricsEmitter.TryIncrementRunCount("stats", nil)
		return nil
	},
}
