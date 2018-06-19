package driver

import (
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/grootfs/base_image_puller/unpacker"
	"code.cloudfoundry.org/lager"
	errorspkg "github.com/pkg/errors"
)

// Unpack unpacks a layer given stream. It's assumed to be packed using tar.
func (d *Driver) Unpack(logger lager.Logger, layerID string, parentIDs []string, layerTar io.Reader) (int64, error) {
	lockFile, err := d.exclusiveLock.Lock(LockKey)
	if err != nil {
		return 0, errorspkg.Wrap(err, "obtaining a lock")
	}
	defer func() {
		if err = d.exclusiveLock.Unlock(lockFile); err != nil {
			logger.Error("failed-to-unlock", err)
		}
	}()

	return d.unpackLayer(logger, layerID, parentIDs, layerTar.(io.ReadCloser))
}

func (d *Driver) unpackLayer(logger lager.Logger, layerID string, parentIDs []string, stream io.ReadCloser) (int64, error) {
	logger = logger.Session("unpacking-layer", lager.Data{})
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
		TargetPath:  volumePath,
		Stream:      stream,
		UIDMappings: savedMappings.UIDMappings,
		GIDMappings: savedMappings.GIDMappings,

		// this is not required, seems to be used only within this experimental context
		// https://github.com/cloudfoundry/grootfs/blob/9b2998b14aea58fa7f8f5b8bbc1ef2070c51095f/fetcher/layer_fetcher/layer_fetcher.go#L19
		BaseDirectory: "",
	}

	volSize, err := d.unpackLayerToTemporaryDirectory(logger, unpackSpec, layerID, parentIDs, tempVolumeName)
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
	if err := d.WriteVolumeMeta(logger, chainID, base_image_puller.VolumeMeta{Size: volSize}); err != nil {
		return errorspkg.Wrapf(err, "writing volume `%s` metadata", chainID)
	}

	finalVolumePath := strings.Replace(volumePath, tempVolumeName, chainID, 1)
	if err := d.MoveVolume(logger, volumePath, finalVolumePath); err != nil {
		return errorspkg.Wrapf(err, "failed to move volume to its final location")
	}

	return nil
}

func (d *Driver) unpackLayerToTemporaryDirectory(logger lager.Logger, unpackSpec base_image_puller.UnpackSpec, layerID string, parentIDs []string, tempVolumeName string) (volSize int64, err error) {
	defer d.metricsEmitter.TryEmitDurationFrom(logger, "UnpackTime", time.Now())

	var unpackOutput base_image_puller.UnpackOutput

	// SUSE: Always create a tar unpacker
	tarUnpacker, err := unpacker.NewTarUnpacker(
		unpacker.UnpackStrategy{
			Name:               "btrfs",
			WhiteoutDevicePath: path.Join(d.conf.StorePath, "whiteout_dev"),
		},
	)

	if unpackOutput, err = tarUnpacker.Unpack(logger, unpackSpec); err != nil {
		if errD := d.DestroyVolume(logger, tempVolumeName); errD != nil {
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

func cleanWhiteoutDir(path string) error {
	contents, err := ioutil.ReadDir(path)
	if err != nil {
		return errorspkg.Wrap(err, "reading whiteout directory")
	}

	for _, content := range contents {
		if err := os.RemoveAll(filepath.Join(path, content.Name())); err != nil {
			return errorspkg.Wrap(err, "cleaning up whiteout directory")
		}
	}

	return nil
}

func (d *Driver) handleOpaqueWhiteouts(logger lager.Logger, id string, opaqueWhiteouts []string) error {
	volumePath, err := d.VolumePath(logger, id)
	if err != nil {
		return err
	}

	for _, opaqueWhiteout := range opaqueWhiteouts {
		parentDir := path.Dir(filepath.Join(volumePath, opaqueWhiteout))
		if err := cleanWhiteoutDir(parentDir); err != nil {
			return err
		}
	}

	return nil
}
