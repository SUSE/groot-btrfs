package driver

import (
	"archive/tar"
	"fmt"
	errorspkg "github.com/pkg/errors"
	"io"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/grootfs/base_image_puller/unpacker"
	"code.cloudfoundry.org/lager"
	"github.com/pkg/errors"
)

type UnpackStrategy struct {
	Name               string
	WhiteoutDevicePath string
}

func (d *Driver) unpackLayer(logger lager.Logger, layerID string, parentIDs []string, stream io.ReadCloser) error {
	logger = logger.Session("unpacking-layer", lager.Data{"LayerInfo": "TODO"})
	logger.Debug("starting")
	defer logger.Debug("ending")

	tempVolumeName, volumePath, err := d.createTemporaryVolumeDirectory(logger, layerID, parentIDs)
	if err != nil {
		return err
	}

	uidMappings, err := d.UIDMappings()
	if err != nil {
		return err
	}

	gidMappings, err := d.GIDMappings()
	if err != nil {
		return err
	}

	unpackSpec := base_image_puller.UnpackSpec{
		TargetPath:    volumePath,
		Stream:        stream,
		UIDMappings:   MappingListToIDMappingSpec(uidMappings),
		GIDMappings:   MappingListToIDMappingSpec(gidMappings),
		BaseDirectory: "", // TODO: is this ok? Looks like groot-windows doesn't use this?
	}

	volSize, err := d.unpackLayerToTemporaryDirectory(logger, unpackSpec, layerID, parentIDs)
	if err != nil {
		return err
	}

	return d.finalizeVolume(logger, tempVolumeName, volumePath, layerID, volSize)
}

func (d *Driver) createTemporaryVolumeDirectory(logger lager.Logger, layerID string, parentIDs []string) (string, string, error) {
	tempVolumeName := fmt.Sprintf("%s-incomplete-%d-%d", layerID, time.Now().UnixNano(), rand.Int())
	volumePath, err := d.CreateVolume(logger,
		parentIDs[len(parentIDs)-1],
		tempVolumeName,
	)

	if err != nil {
		return "", "", errorspkg.Wrapf(err, "creating volume for layer `%s`", layerID)
	}
	logger.Debug("volume-created", lager.Data{"volumePath": volumePath})

	UIDMappings, err := d.UIDMappings()
	if err != nil {
		return "", "", errors.Wrapf(err, "Can't map UID: %s", err.Error())
	}

	GIDMappings, err := d.UIDMappings()
	if err != nil {
		return "", "", errors.Wrapf(err, "Can't map GID: %s", err.Error())
	}
	ownerUID, ownerGID := d.parseOwner(UIDMappings, GIDMappings)

	if ownerUID != 0 || ownerGID != 0 {
		err = os.Chown(volumePath, ownerUID, ownerGID)
		if err != nil {
			return "", "", errorspkg.Wrapf(err, "changing volume ownership to %d:%d", ownerUID, ownerGID)
		}
	}

	return tempVolumeName, volumePath, nil
}

func (d *Driver) finalizeVolume(logger lager.Logger, tempVolumeName, volumePath, chainID string, volSize int64) error {
	if err := d.WriteVolumeMeta(logger, chainID, base_image_puller.VolumeMeta{Size: volSize}); err != nil {
		return errorspkg.Wrapf(err, "writing volume `%s` metadata", chainID)
	}

	finalVolumePath := strings.Replace(volumePath, tempVolumeName, chainID, 1)
	if err := p.volumeDriver.MoveVolume(logger, volumePath, finalVolumePath); err != nil {
		return errorspkg.Wrapf(err, "failed to move volume to its final location")
	}

	return nil
}

func (d *Driver) unpackLayerToTemporaryDirectory(logger lager.Logger, unpackSpec UnpackSpec, layerInfo, parentLayerInfo groot.LayerInfo) (volSize int64, err error) {
	defer p.metricsEmitter.TryEmitDurationFrom(logger, MetricsUnpackTimeName, time.Now())

	if unpackSpec.BaseDirectory != "" {
		parentPath, err := p.volumeDriver.VolumePath(logger, parentLayerInfo.ChainID)
		if err != nil {
			return 0, err
		}

		if err := ensureBaseDirectoryExists(unpackSpec.BaseDirectory, unpackSpec.TargetPath, parentPath); err != nil {
			return 0, err
		}
	}

	var unpackOutput UnpackOutput
	if unpackOutput, err = p.unpacker.Unpack(logger, unpackSpec); err != nil {
		if errD := p.volumeDriver.DestroyVolume(logger, layerInfo.ChainID); errD != nil {
			logger.Error("volume-cleanup-failed", errD)
		}
		return 0, errorspkg.Wrapf(err, "unpacking layer `%s`", layerInfo.BlobID)
	}

	if err := p.volumeDriver.HandleOpaqueWhiteouts(logger, path.Base(unpackSpec.TargetPath), unpackOutput.OpaqueWhiteouts); err != nil {
		logger.Error("handling-opaque-whiteouts", err)
		return 0, errorspkg.Wrap(err, "handling opaque whiteouts")
	}

	logger.Debug("layer-unpacked")
	return unpackOutput.BytesWritten, nil
}

func (d *Driver) Unpack(logger lager.Logger, layerID string, parentIDs []string, layerTar io.Reader) (int64, error) {
	tarUnpacker, err := unpacker.NewTarUnpacker(
		unpacker.UnpackStrategy{
			Name:               "btrfs",
			WhiteoutDevicePath: path.Join(d.conf.StorePath, "whiteout_dev"),
		},
	)

	if err != nil {
		return 0, err
	}

	unpackSpec := base_image_puller.UnpackSpec{
		TargetPath:    d.conf.StorePath,
		Stream:        layerTar,
		UIDMappings:   d.conf.UIDMapping,
		GIDMappings:   d.conf.GIDMapping,
		BaseDirectory: layerInfo.BaseDirectory,
	}

	a, err := tarUnpacker.Unpack(logger)

	if err != nil {
		return 0, err
	}

	return a.BytesWritten, nil

	//	logger = logger.Session("unpacking-with-tar", lager.Data{"layerID": layerID})
	//	logger.Info("starting")
	//	defer logger.Info("ending")

	//	outputDir := filepath.Join(d.conf.StorePath, layerID)

	//	if err := safeMkdir(outputDir, 0755); err != nil {
	//		return 0, err
	//	}

	//	runtime.LockOSThread()
	//	defer runtime.UnlockOSThread()
	//	if err := chroot(outputDir); err != nil {
	//		return 0, errors.Wrap(err, "failed to chroot")
	//	}

	//	// Create /tmp directory
	//	if err := os.MkdirAll("/tmp", 777); err != nil {
	//		return 0, errors.Wrap(err, "could not create /tmp directory in chroot")
	//	}

	//	tarReader := tar.NewReader(layerTar)
	//	opaqueWhiteouts := []string{}
	//	var totalBytesUnpacked int64
	//	for {
	//		tarHeader, err := tarReader.Next()
	//		if err == io.EOF {
	//			break
	//		} else if err != nil {
	//			return 0, err
	//		}

	//		// We need this BaseDirectory: layer.Annotations[cfBaseDirectoryAnnotation],
	//		// https://github.com/cloudfoundry/grootfs/blob/master/fetcher/layer_fetcher/layer_fetcher.go#L19
	//		// For that, we need to get the layerInfo using the layerID
	//		//entryPath := filepath.Join(spec.BaseDirectory, tarHeader.Name)
	//		// TODO: Fix this
	//		entryPath := filepath.Join("/", tarHeader.Name)

	//		if strings.Contains(tarHeader.Name, ".wh..wh..opq") {
	//			opaqueWhiteouts = append(opaqueWhiteouts, entryPath)
	//			continue
	//		}

	//		if strings.Contains(tarHeader.Name, ".wh.") {
	//			if err := removeWhiteout(entryPath); err != nil {
	//				return 0, err
	//			}
	//			continue
	//		}

	//		entrySize, err := d.handleEntry(logger, entryPath, tarReader, tarHeader)
	//		if err != nil {
	//			return 0, err
	//		}

	//		totalBytesUnpacked += entrySize
	//	}

	//	return totalBytesUnpacked, nil
	//	// TODO: Do we need to process whiteouts further?
}

func safeMkdir(path string, perm os.FileMode) error {
	if _, err := os.Stat(path); err != nil {
		if err := os.Mkdir(path, perm); err != nil {
			return errors.Wrapf(err, "making destination directory `%s`", path)
		}
	}
	return nil
}

func chroot(path string) error {
	if err := syscall.Chroot(path); err != nil {
		return err
	}

	if err := os.Chdir("/"); err != nil {
		return err
	}

	return nil
}

func removeWhiteout(path string) error {
	toBeDeletedPath := strings.Replace(path, ".wh.", "", 1)
	if err := os.RemoveAll(toBeDeletedPath); err != nil {
		return errors.Wrap(err, "deleting whiteout file")
	}

	return nil
}

func (d *Driver) handleEntry(logger lager.Logger, entryPath string, tarReader *tar.Reader, tarHeader *tar.Header) (entrySize int64, err error) {
	switch tarHeader.Typeflag {
	case tar.TypeBlock, tar.TypeChar:
		// ignore devices
		return 0, nil

	case tar.TypeLink:
		if err = d.createLink(logger, entryPath, tarHeader); err != nil {
			return 0, err
		}

	case tar.TypeSymlink:
		if err = d.createSymlink(logger, entryPath, tarHeader); err != nil {
			return 0, err
		}

	case tar.TypeDir:
		if err = d.createDirectory(logger, entryPath, tarHeader); err != nil {
			return 0, err
		}

	case tar.TypeReg, tar.TypeRegA:
		if entrySize, err = d.createRegularFile(logger, entryPath, tarHeader, tarReader); err != nil {
			return 0, err
		}
	}

	return entrySize, nil
}

func (d *Driver) createLink(logger lager.Logger, path string, tarHeader *tar.Header) error {
	return os.Link(tarHeader.Linkname, path)
}

func (d *Driver) createSymlink(logger lager.Logger, path string, tarHeader *tar.Header) error {
	if _, err := os.Lstat(path); err == nil {
		if err := os.Remove(path); err != nil {
			return errors.Wrapf(err, "removing file `%s`", path)
		}
	}

	if err := os.Symlink(tarHeader.Linkname, path); err != nil {
		return errors.Wrapf(err, "create symlink `%s` -> `%s`", tarHeader.Linkname, path)
	}

	if err := changeModTime(path, tarHeader.ModTime); err != nil {
		return errors.Wrapf(err, "setting the modtime for the symlink `%s`", path)
	}

	if os.Getuid() == 0 {
		UIDMappings, err := d.UIDMappings()
		if err != nil {
			return errors.Wrapf(err, "Can't map UID: %s", err.Error())
		}

		GIDMappings, err := d.UIDMappings()
		if err != nil {
			return errors.Wrapf(err, "Can't map GID: %s", err.Error())
		}

		uid := UIDMappings.Map(tarHeader.Uid)
		gid := GIDMappings.Map(tarHeader.Gid)

		if err := os.Lchown(path, uid, gid); err != nil {
			return errors.Wrapf(err, "chowning link %d:%d `%s`", uid, gid, path)
		}
	}

	return nil
}

func (d *Driver) createDirectory(logger lager.Logger, path string, tarHeader *tar.Header) error {
	if _, err := os.Stat(path); err != nil {
		if err = os.Mkdir(path, tarHeader.FileInfo().Mode()); err != nil {
			newErr := errors.Wrapf(err, "creating directory `%s`", path)

			if os.IsPermission(err) {
				dirName := filepath.Dir(tarHeader.Name)
				return errors.Errorf("'/%s' does not give write permission to its owner. This image can only be unpacked using uid and gid mappings, or by running as root.", dirName)
			}

			return newErr
		}
	}

	if os.Getuid() == 0 {
		UIDMappings, err := d.UIDMappings()
		if err != nil {
			return errors.Wrapf(err, "Can't map UID: %s", err.Error())
		}

		GIDMappings, err := d.UIDMappings()
		if err != nil {
			return errors.Wrapf(err, "Can't map GID: %s", err.Error())
		}

		uid := UIDMappings.Map(tarHeader.Uid)
		gid := GIDMappings.Map(tarHeader.Gid)

		if err := os.Chown(path, uid, gid); err != nil {
			return errors.Wrapf(err, "chowning directory %d:%d `%s`", uid, gid, path)
		}
	}

	// we need to explicitly apply perms because mkdir is subject to umask
	if err := os.Chmod(path, tarHeader.FileInfo().Mode()); err != nil {
		return errors.Wrapf(err, "chmoding directory `%s`", path)
	}

	if err := changeModTime(path, tarHeader.ModTime); err != nil {
		return errors.Wrapf(err, "setting the modtime for directory `%s`: %s", path)
	}

	return nil
}

func (d *Driver) createRegularFile(logger lager.Logger, path string, tarHeader *tar.Header, tarReader *tar.Reader) (int64, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, tarHeader.FileInfo().Mode())
	if err != nil {
		newErr := errors.Wrapf(err, "creating file `%s`", path)

		if os.IsPermission(err) {
			dirName := filepath.Dir(tarHeader.Name)
			return 0, errors.Errorf("'/%s' does not give write permission to its owner. This image can only be unpacked using uid and gid mappings, or by running as root.", dirName)
		}

		return 0, newErr
	}

	fileSize, err := io.Copy(file, tarReader)
	if err != nil {
		_ = file.Close()
		return 0, errors.Wrapf(err, "writing to file `%s`", path)
	}

	if err := file.Close(); err != nil {
		return 0, errors.Wrapf(err, "closing file `%s`", path)
	}

	if os.Getuid() == 0 {
		UIDMappings, err := d.UIDMappings()
		if err != nil {
			return 0, errors.Wrapf(err, "Can't map UID: %s", err.Error())
		}

		GIDMappings, err := d.UIDMappings()
		if err != nil {
			return 0, errors.Wrapf(err, "Can't map GID: %s", err.Error())
		}

		uid := UIDMappings.Map(tarHeader.Uid)
		gid := GIDMappings.Map(tarHeader.Gid)

		if err := os.Chown(path, uid, gid); err != nil {
			return 0, errors.Wrapf(err, "chowning file %d:%d `%s`", uid, gid, path)
		}
	}

	// we need to explicitly apply perms because mkdir is subject to umask
	if err := os.Chmod(path, tarHeader.FileInfo().Mode()); err != nil {
		return 0, errors.Wrapf(err, "chmoding file `%s`", path)
	}

	if err := changeModTime(path, tarHeader.ModTime); err != nil {
		return 0, errors.Wrapf(err, "setting the modtime for file `%s`", path)
	}

	return fileSize, nil
}
