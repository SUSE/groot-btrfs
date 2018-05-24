package driver

import (
	"path/filepath"

	depman "github.com/SUSE/groot-btrfs/dependencymanager"

	wearegroot "code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/metrics"
	"code.cloudfoundry.org/grootfs/store"
	gc "code.cloudfoundry.org/grootfs/store/garbage_collector"
	"code.cloudfoundry.org/grootfs/store/locksmith"
	"code.cloudfoundry.org/lager"
)

// clean cleans volumes if they are not being used
func (d *Driver) clean(logger lager.Logger) (bool, error) {
	lockDir := filepath.Join(d.conf.StorePath, store.LocksDirName)
	iamLocksmith := locksmith.NewExclusiveFileSystem(lockDir)

	dependencyManager := depman.NewDependencyManager(d.dependenciesPath())
	garbageCollector := gc.NewGC(d, d, dependencyManager)
	storeMeasurer := store.NewStoreMeasurer(d.conf.StorePath, d, garbageCollector)

	metricsEmitter := metrics.NewEmitter(logger, d.conf.MetronEndpoint)

	cleaner := wearegroot.IamCleaner(
		iamLocksmith,
		storeMeasurer,
		garbageCollector,
		metricsEmitter,
	)

	return cleaner.Clean(logger, d.conf.CleanThresholdBytes)
}
