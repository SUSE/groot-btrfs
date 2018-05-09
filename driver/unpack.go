package driver

import (
	"code.cloudfoundry.org/lager"
	"io"
)

func (d *Driver) Unpack(logger lager.Logger, layerID string, parentIDs []string, layerTar io.Reader) (int64, error) {
	return 0, nil
}
