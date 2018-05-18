package driver

import (
	"code.cloudfoundry.org/groot"
	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/grootfs/store/filesystems"
	"code.cloudfoundry.org/lager"

	errorspkg "github.com/pkg/errors"
)

func (d *Driver) WriteMetadata(logger lager.Logger, bundleID string, volumeData groot.ImageMetadata) error {
	if err := d.writeVolumeMeta(logger, bundleID, base_image_puller.VolumeMeta{Size: volumeData.Size}); err != nil {
		return errorspkg.Wrapf(err, "writing bundle `%s` metadata", bundleID)
	}

	return nil
}

func (d *Driver) writeVolumeMeta(logger lager.Logger, id string, metadata base_image_puller.VolumeMeta) error {
	logger = logger.Session("btrfs-writing-volume-metadata", lager.Data{"volumeID": id})
	logger.Debug("starting")
	defer logger.Debug("ending")

	return filesystems.WriteVolumeMeta(logger, d.conf.StorePath, id, metadata)
}
