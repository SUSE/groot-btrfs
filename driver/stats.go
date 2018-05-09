package driver

import (
	"code.cloudfoundry.org/groot"
	"code.cloudfoundry.org/lager"
)

func (d *Driver) Stats(logger lager.Logger, bundleID string) (groot.VolumeStats, error) {
	return groot.VolumeStats{}, nil
}
