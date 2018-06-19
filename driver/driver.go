package driver

import (
	"io/ioutil"
	"os"
	"path"

	wearegroot "code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/metrics"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/store/locksmith"
	"code.cloudfoundry.org/lager"
	errorspkg "github.com/pkg/errors"
)

const (
	// BtrfsType is BTRFS_SUPER_MAGIC f_type constant
	// see `man statfs` for more details
	BtrfsType = 0x9123683E

	// Filename for locking
	LockKey = "groot-btrfs-lock"

	utimeOmit         int64 = ((1 << 30) - 2)
	atSymlinkNoFollow int   = 0x100
)

// Driver represents a BTRFS groot Driver
type Driver struct {
	exclusiveLock  *locksmith.FileSystem
	sharedLock     *locksmith.FileSystem
	metricsEmitter *metrics.Emitter
	conf           *Config
}

// NewDriver creates a new Driver
func NewDriver(conf *Config) *Driver {

	logger := lager.NewLogger("groot-btrfs-metrics-emmiter-init")
	metricsEmitter := metrics.NewEmitter(logger, conf.MetronEndpoint)

	os.MkdirAll(conf.LockDir(), 0755)

	return &Driver{
		conf:           conf,
		metricsEmitter: metricsEmitter,
		exclusiveLock:  locksmith.NewExclusiveFileSystem(conf.LockDir()).WithMetrics(metricsEmitter),
		sharedLock:     locksmith.NewSharedFileSystem(conf.LockDir()).WithMetrics(metricsEmitter),
	}
}

// ImageIDs returns the list of existing image IDs
func (d *Driver) ImageIDs(logger lager.Logger) ([]string, error) {
	images := []string{}

	existingImages, err := ioutil.ReadDir(path.Join(d.conf.StorePath, store.ImageDirName))
	if err != nil {
		return nil, errorspkg.Wrap(err, "failed to read images dir")
	}

	for _, imageInfo := range existingImages {
		images = append(images, imageInfo.Name())
	}

	return images, nil
}

func (d *Driver) readMappings() (wearegroot.IDMappings, error) {
	storeNamespacer := wearegroot.NewStoreNamespacer(d.conf.StorePath)
	idMappings, err := storeNamespacer.Read()
	if err != nil {
		return wearegroot.IDMappings{}, err
	}

	return idMappings, nil
}

func (d *Driver) parseOwner(idMappings wearegroot.IDMappings) (int, int) {
	uid := os.Getuid()
	gid := os.Getgid()

	for _, mapping := range idMappings.UIDMappings {
		if mapping.Size == 1 && mapping.NamespaceID == 0 {
			uid = int(mapping.HostID)
			break
		}
	}

	for _, mapping := range idMappings.GIDMappings {
		if mapping.Size == 1 && mapping.NamespaceID == 0 {
			gid = int(mapping.HostID)
			break
		}
	}

	return uid, gid
}

func (d *Driver) imagePath(id string) string {
	return path.Join(d.conf.StorePath, store.ImageDirName, id)
}
