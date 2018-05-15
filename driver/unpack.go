package driver

import (
	"archive/tar"
	"code.cloudfoundry.org/lager"
	"github.com/pkg/errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

type UnpackStrategy struct {
	Name               string
	WhiteoutDevicePath string
}

func (d *Driver) Unpack(logger lager.Logger, layerID string, parentIDs []string, layerTar io.Reader) (int64, error) {
	logger = logger.Session("unpacking-with-tar", lager.Data{"layerID": layerID})
	logger.Info("starting")
	defer logger.Info("ending")

	outputDir := filepath.Join(d.conf.StorePath, layerID)

	if err := safeMkdir(outputDir, 0755); err != nil {
		return 0, err
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := chroot(outputDir); err != nil {
		return 0, errors.Wrap(err, "failed to chroot")
	}

	// Create /tmp directory
	if err := os.MkdirAll("/tmp", 777); err != nil {
		return 0, errors.Wrap(err, "could not create /tmp directory in chroot")
	}

	tarReader := tar.NewReader(layerTar)
	opaqueWhiteouts := []string{}
	var totalBytesUnpacked int64
	for {
		tarHeader, err := tarReader.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return 0, err
		}

		// We need this BaseDirectory: layer.Annotations[cfBaseDirectoryAnnotation],
		// https://github.com/cloudfoundry/grootfs/blob/master/fetcher/layer_fetcher/layer_fetcher.go#L19
		// For that, we need to get the layerInfo using the layerID
		//entryPath := filepath.Join(spec.BaseDirectory, tarHeader.Name)
		// TODO: Fix this
		entryPath := filepath.Join("/", tarHeader.Name)

		if strings.Contains(tarHeader.Name, ".wh..wh..opq") {
			opaqueWhiteouts = append(opaqueWhiteouts, entryPath)
			continue
		}

		if strings.Contains(tarHeader.Name, ".wh.") {
			if err := removeWhiteout(entryPath); err != nil {
				return 0, err
			}
			continue
		}

		entrySize, err := d.handleEntry(logger, entryPath, tarReader, tarHeader)
		if err != nil {
			return 0, err
		}

		totalBytesUnpacked += entrySize
	}

	return totalBytesUnpacked, nil
	// TODO: Do we need to process whiteouts further?
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
