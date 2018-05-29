package driver

import (
	"fmt"
	"io"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	errorspkg "github.com/pkg/errors"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/grootfs/base_image_puller/unpacker"
	"code.cloudfoundry.org/lager"
)

func (d *Driver) unpackLayer(logger lager.Logger, layerID string, parentIDs []string, stream io.ReadCloser) (int64, error) {
	logger = logger.Session("unpacking-layer", lager.Data{"LayerInfo": "TODO"})
	logger.Debug("starting")
	defer logger.Debug("ending")

	tempVolumeName, volumePath, err := d.createTemporaryVolumeDirectory(logger, layerID, parentIDs)
	if err != nil {
		return 0, err
	}

	savedMappings, err := d.readMappings()
	if err != nil {
		return 0, err
	}

	unpackSpec := base_image_puller.UnpackSpec{
		TargetPath:    volumePath,
		Stream:        stream,
		UIDMappings:   savedMappings.UIDMappings,
		GIDMappings:   savedMappings.GIDMappings,
		BaseDirectory: "", // TODO: is this ok? Looks like groot-windows doesn't use this?
	}

	volSize, err := d.unpackLayerToTemporaryDirectory(logger, unpackSpec, layerID, parentIDs)
	if err != nil {
		return 0, err
	}

	return volSize, d.finalizeVolume(logger, tempVolumeName, volumePath, layerID, volSize)
}

func (d *Driver) createTemporaryVolumeDirectory(logger lager.Logger, layerID string, parentIDs []string) (string, string, error) {
	tempVolumeName := fmt.Sprintf("%s-incomplete-%d-%d", layerID, time.Now().UnixNano(), rand.Int())

	parentID := ""
	if len(parentIDs) != 0 {
		parentID = parentIDs[len(parentIDs)-1]
	}

	volumePath, err := d.createVolume(logger, parentID, tempVolumeName)

	if err != nil {
		return "", "", errorspkg.Wrapf(err, "creating volume for layer `%s`", layerID)
	}
	logger.Debug("volume-created", lager.Data{"volumePath": volumePath})

	savedMappings, err := d.readMappings()
	if err != nil {
		return "", "", errorspkg.Wrapf(err, "reading id mappings for layer `%s`", layerID)
	}

	ownerUID, ownerGID := d.parseOwner(savedMappings)

	if ownerUID != 0 || ownerGID != 0 {
		err = os.Chown(volumePath, ownerUID, ownerGID)
		if err != nil {
			return "", "", errorspkg.Wrapf(err, "changing volume ownership to %d:%d", ownerUID, ownerGID)
		}
	}

	return tempVolumeName, volumePath, nil
}

func (d *Driver) finalizeVolume(logger lager.Logger, tempVolumeName, volumePath, chainID string, volSize int64) error {
	finalVolumePath := strings.Replace(volumePath, tempVolumeName, chainID, 1)
	if err := d.MoveVolume(logger, volumePath, finalVolumePath); err != nil {
		return errorspkg.Wrapf(err, "failed to move volume to its final location")
	}

	return nil
}

func (d *Driver) unpackLayerToTemporaryDirectory(logger lager.Logger, unpackSpec base_image_puller.UnpackSpec, layerID string, parentIDs []string) (volSize int64, err error) {
	// TODO: add metrics back to the implementation
	//	defer p.metricsEmitter.TryEmitDurationFrom(logger, MetricsUnpackTimeName, time.Now())

	if unpackSpec.BaseDirectory != "" {
		parentID := ""
		if len(parentIDs) != 0 {
			parentID = parentIDs[len(parentIDs)-1]
		}

		parentPath, err := d.VolumePath(logger, parentID)
		if err != nil {
			return 0, err
		}

		if err := ensureBaseDirectoryExists(unpackSpec.BaseDirectory, unpackSpec.TargetPath, parentPath); err != nil {
			return 0, err
		}
	}

	var unpackOutput base_image_puller.UnpackOutput

	// SUSE: Always create a tar unpacker
	tarUnpacker, err := unpacker.NewTarUnpacker(
		unpacker.UnpackStrategy{
			Name:               "btrfs",
			WhiteoutDevicePath: path.Join(d.conf.StorePath, "whiteout_dev"),
		},
	)

	if unpackOutput, err = tarUnpacker.Unpack(logger, unpackSpec); err != nil {
		if errD := d.DestroyVolume(logger, layerID); errD != nil {
			logger.Error("volume-cleanup-failed", errD)
		}
		return 0, errorspkg.Wrapf(err, "unpacking layer `%s` failed", layerID)
	}

	if err := d.handleOpaqueWhiteouts(logger, path.Base(unpackSpec.TargetPath), unpackOutput.OpaqueWhiteouts); err != nil {
		logger.Error("handling-opaque-whiteouts", err)
		return 0, errorspkg.Wrap(err, "handling opaque whiteouts")
	}

	logger.Debug("layer-unpacked")
	return unpackOutput.BytesWritten, nil
}

func ensureBaseDirectoryExists(baseDir, childPath, parentPath string) error {
	if baseDir == string(filepath.Separator) {
		return nil
	}

	if err := ensureBaseDirectoryExists(filepath.Dir(baseDir), childPath, parentPath); err != nil {
		return err
	}

	stat, err := os.Stat(filepath.Join(childPath, baseDir))
	if err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return errorspkg.Wrapf(err, "failed to stat base directory")
	}

	stat, err = os.Stat(filepath.Join(parentPath, baseDir))
	if err != nil {
		return errorspkg.Wrapf(err, "base directory not found in parent layer")
	}

	fullChildBaseDir := filepath.Join(childPath, baseDir)
	if err := os.Mkdir(fullChildBaseDir, stat.Mode()); err != nil {
		return errorspkg.Wrapf(err, "could not create base directory in child layer")
	}

	statt := stat.Sys().(*syscall.Stat_t)
	if err := os.Chown(fullChildBaseDir, int(statt.Uid), int(statt.Gid)); err != nil {
		return errorspkg.Wrapf(err, "could not chown base directory")
	}

	return nil
}

// Unpack unpacks a layer given stream. It's assumed to be packed using tar.
func (d *Driver) Unpack(logger lager.Logger, layerID string, parentIDs []string, layerTar io.Reader) (int64, error) {
	return d.unpackLayer(logger, layerID, parentIDs, layerTar.(io.ReadCloser))
}
