package driver

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/lager"
	"github.com/SUSE/groot-btrfs/dependency_manager"
	errorspkg "github.com/pkg/errors"
)

func (d *Driver) Delete(logger lager.Logger, bundleID string) error {
	//TODO: add metrics back to the implementation
	//defer d.metricsEmitter.TryEmitDurationFrom(logger, MetricImageDeletionTime, time.Now())

	logger = logger.Session("groot-deleting", lager.Data{"imageID": bundleID})
	logger.Info("starting")
	defer logger.Info("ending")

	if err := d.Destroy(logger, bundleID); err != nil {
		return err
	}

	dependencyManager := dependency_manager.NewDependencyManager(d.dependenciesPath())

	// TODO: Do we also will need to implement a garbage collector?
	imageRefName := fmt.Sprintf(ImageReferenceFormat, bundleID)
	if err := dependencyManager.Deregister(imageRefName); err != nil {
		if !os.IsNotExist(errorspkg.Cause(err)) {
			logger.Error("failed-to-deregister-dependencies", err)
			return err
		}
	}

	return nil
}

func (d *Driver) Destroy(logger lager.Logger, id string) error {
	logger = logger.Session("deleting-image", lager.Data{"storePath": d.conf.StorePath, "id": id})
	logger.Info("starting")
	defer logger.Info("ending")

	if ok, err := d.Exists(id); !ok {
		logger.Error("checking-image-path-failed", err)
		if err != nil {
			return errorspkg.Wrapf(err, "unable to check image: %s", id)
		}
		return errorspkg.Errorf("image not found: %s", id)
	}

	imagePath := d.imagePath(id)
	var volDriverErr error
	if volDriverErr = d.DestroyImage(logger, imagePath); volDriverErr != nil {
		logger.Error("destroying-image-failed", volDriverErr)
	}

	if _, err := os.Stat(imagePath); err == nil {
		logger.Error("deleting-image-dir-failed", err, lager.Data{"volumeDriverError": volDriverErr})
		return errors.New("deleting image path")
	}

	return nil
}

func (d *Driver) Exists(id string) (bool, error) {
	imagePath := d.imagePath(id)

	if _, err := os.Stat(imagePath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, errorspkg.Wrapf(err, "checking if image `%s` exists", id)
	}

	return true, nil
}

func (d *Driver) DestroyImage(logger lager.Logger, imagePath string) error {
	logger = logger.Session("btrfs-destroying-image", lager.Data{"imagePath": imagePath})
	logger.Info("starting")
	defer logger.Info("ending")

	snapshotMountPath := filepath.Join(imagePath, "rootfs")
	if _, err := os.Stat(filepath.Join(imagePath, "snapshot")); err == nil {
		if err := os.Remove(snapshotMountPath); err != nil {
			logger.Error("removing-rootfs-folder-failed", err)
			return errorspkg.Wrap(err, "remove rootfs folder")
		}
		snapshotMountPath = filepath.Join(imagePath, "snapshot")
	}

	err := d.destroyBtrfsVolume(logger, snapshotMountPath)
	if err != nil && strings.Contains(err.Error(), "Directory not empty") {
		subvolumes, listErr := d.listSubvolumes(logger, imagePath)
		if listErr != nil {
			logger.Error("listing-subvolumes-failed", listErr)
			return errorspkg.Wrap(listErr, "list subvolumes")
		}

		for _, subvolume := range subvolumes {
			if err := d.destroyBtrfsVolume(logger, subvolume); err != nil {
				return err
			}
		}
		err = nil
	}

	if err := os.RemoveAll(imagePath); err != nil {
		logger.Error("removing-image-path", err)
	}

	return err
}

func (d *Driver) listSubvolumes(logger lager.Logger, path string) ([]string, error) {
	logger = logger.Session("listing-subvolumes", lager.Data{"path": path})
	logger.Debug("starting")
	defer logger.Debug("ending")

	args := []string{
		"--btrfs-bin", d.conf.BtrfsBinPath(),
		"list",
		path,
	}

	stdoutBuffer, err := d.runDrax(logger, args...)
	if err != nil {
		return nil, err
	}

	contents, err := ioutil.ReadAll(stdoutBuffer)
	if err != nil {
		return nil, errorspkg.Wrapf(err, "read drax read output")
	}

	return strings.Split(string(contents), "\n"), nil
}

func (d *Driver) Deregister(id string) error {
	return os.Remove(d.filePath(id))
}

func (d *Driver) filePath(id string) string {
	escapedId := strings.Replace(id, "/", "__", -1)
	return filepath.Join(d.dependenciesPath(), fmt.Sprintf("%s.json", escapedId))
}

func (d *Driver) dependenciesPath() string {
	return filepath.Join(d.conf.StorePath, store.MetaDirName, "dependencies")
}
