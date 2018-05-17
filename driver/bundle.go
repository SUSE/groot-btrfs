package driver

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/lager"

	"code.cloudfoundry.org/grootfs/store"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	errorspkg "github.com/pkg/errors"
)

//func (d *Driver) CreateImage(logger lager.Logger, spec image_cloner.ImageDriverSpec) (groot.MountInfo, error) {
func (d *Driver) Bundle(logger lager.Logger, bundleID string, layerIDs []string, diskLimit int64) (specs.Spec, error) {
	logger = logger.Session("btrfs-creating-snapshot", lager.Data{"IDs": layerIDs})
	logger.Info("starting")
	defer logger.Info("ending")

	imagePath := filepath.Join(d.conf.StorePath, store.ImageDirName, bundleID)

	toPath := filepath.Join(imagePath, "rootfs")
	baseVolumePath := filepath.Join(d.conf.StorePath, d.conf.VolumesDirName, layerIDs[len(layerIDs)-1])

	spec := specs.Spec{
		Version: specs.Version,
		Root: &specs.Root{
			Path: baseVolumePath,
		},
		Linux: &specs.Linux{},
	}

	if err := os.MkdirAll(toPath, 0755); err != nil {
		logger.Error("creating-rootfs-folder-failed", err, lager.Data{"rootfs": toPath})
		return specs.Spec{}, errorspkg.Wrap(err, "creating rootfs folder")
	}

	if err := os.Chmod(toPath, 0755); err != nil {
		logger.Error("chmoding-rootfs-folder", err)
		return specs.Spec{}, errorspkg.Wrap(err, "chmoding rootfs folder")
	}

	fmt.Println(baseVolumePath)
	fmt.Println(toPath)

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

	return spec, d.applyDiskLimit(logger, diskLimit)
}
