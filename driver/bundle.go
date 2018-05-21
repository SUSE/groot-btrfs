package driver

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/lager"
	"github.com/SUSE/groot-btrfs/dependency_manager"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	errorspkg "github.com/pkg/errors"
)

func (d *Driver) Bundle(logger lager.Logger, bundleID string, layerIDs []string, diskLimit int64) (specs.Spec, error) {
	logger = logger.Session("btrfs-creating-snapshot", lager.Data{"IDs": layerIDs})
	logger.Info("starting")
	defer logger.Info("ending")

	imagePath := d.imagePath(bundleID)

	toPath := filepath.Join(imagePath, "rootfs")

	baseVolumePath := filepath.Join(d.conf.StorePath, d.conf.VolumesDirName, layerIDs[len(layerIDs)-1])

	spec := specs.Spec{
		Version: specs.Version,
		Root: &specs.Root{
			Path: baseVolumePath,
		},
		Linux: &specs.Linux{},
	}

	if err := os.MkdirAll(imagePath, 0755); err != nil {
		logger.Error("creating-imagepath-folder-failed", err, lager.Data{"imagepath": imagePath})
		return specs.Spec{}, errorspkg.Wrap(err, "creating imagepath folder")
	}

	if err := os.Chmod(imagePath, 0755); err != nil {
		logger.Error("chmoding-imagepath-folder", err)
		return specs.Spec{}, errorspkg.Wrap(err, "chmoding imagepath folder")
	}

	cmd := exec.Command(d.conf.BtrfsBinPath(), "subvolume", "snapshot", baseVolumePath, toPath)
	logger.Debug("starting-btrfs", lager.Data{"path": cmd.Path, "args": cmd.Args})
	if contents, err := cmd.CombinedOutput(); err != nil {
		return specs.Spec{}, errorspkg.Errorf(
			"creating btrfs snapshot from `%s` to `%s` (%s): %s",
			baseVolumePath, toPath, err, string(contents),
		)
	}

	if err := os.Chmod(toPath, 0755); err != nil {
		logger.Error("chmoding-snapshot", err)
		return specs.Spec{}, errorspkg.Wrap(err, "chmoding snapshot")
	}

	if err := d.applyDiskLimit(logger, diskLimit); err != nil {
		logger.Error("apply-disk-limit", err)
		return specs.Spec{}, errorspkg.Wrap(err, "applying disk limit")
	}

	dependencyManager := dependency_manager.NewDependencyManager(
		filepath.Join(d.conf.StorePath, store.MetaDirName, "dependencies"),
	)

	imageRefName := fmt.Sprintf(ImageReferenceFormat, bundleID)
	if err := dependencyManager.Register(imageRefName, layerIDs); err != nil {
		if destroyErr := d.Delete(logger, bundleID); destroyErr != nil {
			logger.Error("failed-to-destroy-image", destroyErr)
		}

		return specs.Spec{}, errorspkg.Wrap(err, "failed to register bundle")
	}

	return spec, d.applyDiskLimit(logger, diskLimit)
}
