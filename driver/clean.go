package driver

import (
	wearegroot "code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/store"
	gc "code.cloudfoundry.org/grootfs/store/garbage_collector"
	"code.cloudfoundry.org/lager"
	depman "github.com/SUSE/groot-btrfs/dependencymanager"
)

// clean cleans volumes if they are not being used
func (d *Driver) clean(logger lager.Logger) (bool, error) {
	dependencyManager := depman.NewDependencyManager(d.dependenciesPath())
	garbageCollector := gc.NewGC(d, d, dependencyManager)
	storeMeasurer := store.NewStoreMeasurer(d.conf.StorePath, d, garbageCollector)

	cleaner := wearegroot.IamCleaner(
		d.exclusiveLock,
		storeMeasurer,
		garbageCollector,
		d.metricsEmitter,
	)

	return cleaner.Clean(logger, d.conf.CleanThresholdBytes)
}
