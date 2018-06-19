package driver

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/store/filesystems"
	"code.cloudfoundry.org/lager"
	errorspkg "github.com/pkg/errors"
)

// MoveVolume uses os.Rename to rename a volume
func (d *Driver) MoveVolume(logger lager.Logger, from, to string) error {
	logger = logger.Session("btrfs-moving-volume", lager.Data{"from": from, "to": to})
	logger.Debug("starting")
	defer logger.Debug("ending")

	if err := os.Rename(from, to); err != nil {
		if !os.IsExist(err) {
			logger.Error("moving-volume-failed", err)
			return errorspkg.Wrap(err, "moving volume")
		}
	}

	return nil
}

// VolumePath returns the path to a specific volume
func (d *Driver) VolumePath(logger lager.Logger, id string) (string, error) {
	volPath := filepath.Join(d.conf.StorePath, d.conf.VolumesDirName, id)
	_, err := os.Stat(volPath)
	if err == nil {
		return volPath, nil
	}

	return "", errorspkg.Wrapf(err, "volume does not exist `%s`", id)
}

// DestroyVolume destroys a BTRFS volume
func (d *Driver) DestroyVolume(logger lager.Logger, id string) error {
	logger = logger.Session("btrfs-destroying-volume", lager.Data{"volumeID": id})
	logger.Info("starting")
	defer logger.Info("ending")

	volumeMetaFilePath := filesystems.VolumeMetaFilePath(d.conf.StorePath, id)

	if _, err := os.Stat(volumeMetaFilePath); err == nil {
		if err := os.Remove(volumeMetaFilePath); err != nil {
			logger.Info("deleting-metadata-file-failed", lager.Data{"path": volumeMetaFilePath, "err": err})
		}
	}

	return d.destroyBtrfsVolume(logger, filepath.Join(d.conf.StorePath, "volumes", id))
}

// VolumeSize returns the size of a volume in bytes
func (d *Driver) VolumeSize(logger lager.Logger, id string) (int64, error) {
	logger = logger.Session("btrfs-volume-size", lager.Data{"volumeID": id})
	logger.Debug("starting")
	defer logger.Debug("ending")

	return filesystems.VolumeSize(logger, d.conf.StorePath, id)
}

// Volumes returns the list of existing volume IDs
func (d *Driver) Volumes(logger lager.Logger) ([]string, error) {
	logger = logger.Session("btrfs-listing-volumes")
	logger.Debug("starting")
	defer logger.Debug("ending")

	volumes := []string{}
	existingVolumes, err := ioutil.ReadDir(path.Join(d.conf.StorePath, store.VolumesDirName))
	if err != nil {
		return nil, errorspkg.Wrap(err, "failed to list volumes")
	}

	for _, volumeInfo := range existingVolumes {
		volumes = append(volumes, volumeInfo.Name())
	}
	return volumes, nil
}

func (d *Driver) createVolume(logger lager.Logger, parentID, id string) (string, error) {
	logger = logger.Session("btrfs-creating-volume", lager.Data{"parentID": parentID, "id": id})
	logger.Info("starting")
	defer logger.Info("ending")

	var cmd *exec.Cmd
	volPath := filepath.Join(d.conf.StorePath, d.conf.VolumesDirName, id)

	if parentID == "" {
		cmd = exec.Command(d.conf.BtrfsBinPath(), "subvolume", "create", volPath)
	} else {
		parentVolPath := filepath.Join(d.conf.StorePath, d.conf.VolumesDirName, parentID)
		cmd = exec.Command(d.conf.BtrfsBinPath(), "subvolume", "snapshot", parentVolPath, volPath)
	}

	logger.Debug("starting-btrfs", lager.Data{"path": cmd.Path, "args": cmd.Args})
	if contents, err := cmd.CombinedOutput(); err != nil {
		return "", errorspkg.Wrapf(err, "creating btrfs volume `%s` %s", volPath, string(contents))
	}

	return volPath, nil
}
