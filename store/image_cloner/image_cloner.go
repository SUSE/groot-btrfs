package image_cloner // import "github.com/SUSE/groot-btrfs/store/image_cloner"

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"code.cloudfoundry.org/lager"
	"github.com/SUSE/groot-btrfs/groot"
	"github.com/SUSE/groot-btrfs/store"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
	errorspkg "github.com/pkg/errors"
)

type ImageDriverSpec struct {
	BaseVolumeIDs      []string
	Mount              bool
	ImagePath          string
	DiskLimit          int64
	ExclusiveDiskLimit bool
}

//go:generate counterfeiter . ImageDriver
type ImageDriver interface {
	CreateImage(logger lager.Logger, spec ImageDriverSpec) (groot.MountInfo, error)
	DestroyImage(logger lager.Logger, path string) error
	FetchStats(logger lager.Logger, path string) (groot.VolumeStats, error)
}

type ImageCloner struct {
	imageDriver ImageDriver
	storePath   string
}

func NewImageCloner(imageDriver ImageDriver, storePath string) *ImageCloner {
	return &ImageCloner{
		imageDriver: imageDriver,
		storePath:   storePath,
	}
}

func (b *ImageCloner) ImageIDs(logger lager.Logger) ([]string, error) {
	images := []string{}

	existingImages, err := ioutil.ReadDir(path.Join(b.storePath, store.ImageDirName))
	if err != nil {
		return nil, errorspkg.Wrap(err, "failed to read images dir")
	}

	for _, imageInfo := range existingImages {
		images = append(images, imageInfo.Name())
	}

	return images, nil
}

func (b *ImageCloner) Create(logger lager.Logger, spec groot.ImageSpec) (groot.ImageInfo, error) {
	logger = logger.Session("making-image", lager.Data{"storePath": b.storePath, "id": spec.ID})
	logger.Info("starting")
	defer logger.Info("ending")

	imagePath := b.imagePath(spec.ID)
	imageRootFSPath := filepath.Join(imagePath, "rootfs")

	var err error
	defer func() {
		if err != nil {
			log := logger.Session("create-failed-cleaning-up", lager.Data{
				"id":    spec.ID,
				"cause": err.Error(),
			})

			log.Info("starting")
			defer log.Info("ending")

			if err = b.imageDriver.DestroyImage(logger, imagePath); err != nil {
				log.Error("destroying-rootfs-image", err)
			}

			if err := os.RemoveAll(imagePath); err != nil {
				log.Error("deleting-image-path", err)
			}
		}
	}()

	if err = os.Mkdir(imagePath, 0700); err != nil {
		return groot.ImageInfo{}, errorspkg.Wrap(err, "making image path")
	}

	imageDriverSpec := ImageDriverSpec{
		BaseVolumeIDs:      spec.BaseVolumeIDs,
		Mount:              spec.Mount,
		ImagePath:          imagePath,
		DiskLimit:          spec.DiskLimit,
		ExclusiveDiskLimit: spec.ExcludeBaseImageFromQuota,
	}

	var mountInfo groot.MountInfo
	if mountInfo, err = b.imageDriver.CreateImage(logger, imageDriverSpec); err != nil {
		logger.Error("creating-image-failed", err, lager.Data{"imageDriverSpec": imageDriverSpec})
		return groot.ImageInfo{}, errorspkg.Wrap(err, "creating image")
	}

	if err := b.setOwnership(spec,
		imagePath,
		imageRootFSPath,
	); err != nil {
		logger.Error("setting-permission-failed", err, lager.Data{"imageDriverSpec": imageDriverSpec})
		return groot.ImageInfo{}, err
	}

	imageInfo, err := b.imageInfo(imageRootFSPath, imagePath, spec.BaseImage, mountInfo, spec.Mount)
	if err != nil {
		logger.Error("creating-image-object", err)
		return groot.ImageInfo{}, errorspkg.Wrap(err, "creating image object")
	}

	if err := b.createVolumesSources(imageInfo.Mounts, spec.OwnerUID, spec.OwnerGID); err != nil {
		return groot.ImageInfo{}, errorspkg.Wrap(err, "creating volume source")
	}

	return imageInfo, nil
}

func (b *ImageCloner) createVolumesSources(mounts []groot.MountInfo, ownerUID, ownerGID int) error {
	for _, mountInfo := range mounts {
		if mountInfo.Type != "bind" {
			continue
		}

		if err := os.Mkdir(mountInfo.Source, 0755); err != nil {
			return err
		}
		if err := os.Chown(mountInfo.Source, ownerUID, ownerGID); err != nil {
			return err
		}
	}
	return nil
}

func (b *ImageCloner) Destroy(logger lager.Logger, id string) error {
	logger = logger.Session("deleting-image", lager.Data{"storePath": b.storePath, "id": id})
	logger.Info("starting")
	defer logger.Info("ending")

	if ok, err := b.Exists(id); !ok {
		logger.Error("checking-image-path-failed", err)
		if err != nil {
			return errorspkg.Wrapf(err, "unable to check image: %s", id)
		}
		return errorspkg.Errorf("image not found: %s", id)
	}

	imagePath := b.imagePath(id)
	var volDriverErr error
	if volDriverErr = b.imageDriver.DestroyImage(logger, imagePath); volDriverErr != nil {
		logger.Error("destroying-image-failed", volDriverErr)
	}

	if _, err := os.Stat(imagePath); err == nil {
		logger.Error("deleting-image-dir-failed", err, lager.Data{"volumeDriverError": volDriverErr})
		return errors.New("deleting image path")
	}

	return nil
}

func (b *ImageCloner) Exists(id string) (bool, error) {
	imagePath := path.Join(b.storePath, store.ImageDirName, id)
	if _, err := os.Stat(imagePath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, errorspkg.Wrapf(err, "checking if image `%s` exists", id)
	}

	return true, nil
}

func (b *ImageCloner) Stats(logger lager.Logger, id string) (groot.VolumeStats, error) {
	logger = logger.Session("fetching-stats", lager.Data{"id": id})
	logger.Debug("starting")
	defer logger.Debug("ending")

	if ok, err := b.Exists(id); !ok {
		logger.Error("checking-image-path-failed", err)
		return groot.VolumeStats{}, errorspkg.Errorf("image not found: %s", id)
	}

	imagePath := b.imagePath(id)

	return b.imageDriver.FetchStats(logger, imagePath)
}

var OpenFile = os.OpenFile

func (b *ImageCloner) imageInfo(rootfsPath, imagePath string, baseImage specsv1.Image, mountJson groot.MountInfo, mount bool) (groot.ImageInfo, error) {
	imageInfo := groot.ImageInfo{
		Path:   imagePath,
		Rootfs: rootfsPath,
		Image:  baseImage,
	}

	if !mount {
		imageInfo.Mounts = []groot.MountInfo{mountJson}
	}

	for volume, _ := range baseImage.Config.Volumes {
		volumeHash := sha256.Sum256([]byte(volume))
		mountSourceName := "vol-" + hex.EncodeToString(volumeHash[:32])

		imageInfo.Mounts = append(imageInfo.Mounts, groot.MountInfo{
			Destination: volume,
			Source:      filepath.Join(imagePath, mountSourceName),
			Type:        "bind",
			Options:     []string{"bind"},
		})
	}

	return imageInfo, nil
}

func (b *ImageCloner) imagePath(id string) string {
	return path.Join(b.storePath, store.ImageDirName, id)
}

func (b *ImageCloner) setOwnership(spec groot.ImageSpec, paths ...string) error {
	if spec.OwnerUID == 0 && spec.OwnerGID == 0 {
		return nil
	}

	for _, path := range paths {
		if err := os.Chown(path, spec.OwnerUID, spec.OwnerGID); err != nil {
			return errorspkg.Wrapf(err, "changing %s ownership to %d:%d", path, spec.OwnerUID, spec.OwnerGID)
		}
	}
	return nil
}
