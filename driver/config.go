package driver

import (
	"path/filepath"

	"code.cloudfoundry.org/grootfs/store"
)

// Config contains all the configurations required for the driver
type Config struct {
	VolumesDirName      string
	BtrfsProgsPath      string
	MetronEndpoint      string
	CleanThresholdBytes int64
	DraxBinPath         string
	StorePath           string
}

// BtrfsBinPath calculates the path to the btrfs CLI.
func (c *Config) BtrfsBinPath() string {
	return filepath.Join(c.BtrfsProgsPath, "btrfs")
}

// MkfsBinPath calculates the path to the mkfs.btrfs CLI.
func (c *Config) MkfsBinPath() string {
	return filepath.Join(c.BtrfsProgsPath, "mkfs.btrfs")
}

// LockDir calculates the path to the directory used for the locking mechanism
func (c *Config) LockDir() string {
	return filepath.Join(c.StorePath, store.LocksDirName)
}
