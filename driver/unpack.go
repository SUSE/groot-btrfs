package driver

import (
	"fmt"
	"io"
	"math/rand"
	"os"
	"path"
	"strings"
	"time"

	errorspkg "github.com/pkg/errors"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/grootfs/base_image_puller/unpacker"
	"code.cloudfoundry.org/grootfs/metrics"
	"code.cloudfoundry.org/lager"
)

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
	metricsEmitter := metrics.NewEmitter(logger, d.conf.MetronEndpoint)
	defer metricsEmitter.TryEmitDurationFrom(logger, "UnpackTime", time.Now())

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

// Unpack unpacks a layer given stream. It's assumed to be packed using tar.
func (d *Driver) Unpack(logger lager.Logger, layerID string, parentIDs []string, layerTar io.Reader) (int64, error) {
	return d.unpackLayer(logger, layerID, parentIDs, layerTar.(io.ReadCloser))
}
