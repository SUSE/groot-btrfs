package driver

import (
	"code.cloudfoundry.org/groot"
	"code.cloudfoundry.org/lager"
)

func (d *Driver) WriteMetadata(logger lager.Logger, bundleID string, volumeData groot.ImageMetadata) error {
	return nil
}
