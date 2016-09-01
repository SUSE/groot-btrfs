package remote

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"code.cloudfoundry.org/grootfs/fetcher"
	"code.cloudfoundry.org/lager"
	"github.com/containers/image/types"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type StreamBlob func(logger lager.Logger) (io.ReadCloser, error)

type ContainersImage struct {
	ref               types.ImageReference
	cacheDriver       fetcher.CacheDriver
	cachedManifest    *specsv1.Manifest
	trustedRegistries []string
}

func NewContainersImage(ref types.ImageReference, cacheDriver fetcher.CacheDriver, trustedRegistries []string) *ContainersImage {
	return &ContainersImage{
		ref:               ref,
		cacheDriver:       cacheDriver,
		trustedRegistries: trustedRegistries,
	}
}

func (i *ContainersImage) Manifest(logger lager.Logger) (specsv1.Manifest, error) {
	imgName := i.ref.DockerReference().RemoteName()
	logger = logger.Session("fetching-image-manifest", lager.Data{"reference": imgName})
	logger.Info("start")
	defer logger.Info("end")

	if i.cachedManifest != nil {
		return *i.cachedManifest, nil
	}

	img, err := i.ref.NewImage("", i.tlsVerify())
	if err != nil {
		return specsv1.Manifest{}, fmt.Errorf("creating image `%s`: %s", imgName, err)
	}

	contents, _, err := img.Manifest()
	if err != nil {
		if strings.Contains(err.Error(), "error fetching manifest: status code:") {
			logger.Error("fetching-manifest-failed", err)
			return specsv1.Manifest{}, fmt.Errorf("fetching manifest `%s`: image does not exist or you do not have permissions to see it", imgName)
		}

		if strings.Contains(err.Error(), "malformed HTTP response") {
			logger.Error("fetching-manifest-failed", err)
			return specsv1.Manifest{}, fmt.Errorf("fetching manifest `%s`: TLS validation of insecure registry failed - %s", imgName, err)
		}
		return specsv1.Manifest{}, fmt.Errorf("fetching manifest `%s`: %s", imgName, err)
	}

	var manifest specsv1.Manifest
	if err := json.Unmarshal(contents, &manifest); err != nil {
		return specsv1.Manifest{}, fmt.Errorf("parsing manifest `%s`: %s", imgName, err)
	}
	i.cachedManifest = &manifest

	return manifest, nil
}

func (i *ContainersImage) Config(logger lager.Logger) (specsv1.Image, error) {
	logger = logger.Session("fetching-image-config")
	logger.Info("start")
	defer logger.Info("end")

	manifest, err := i.Manifest(logger)
	if err != nil {
		return specsv1.Image{}, fmt.Errorf("fetching manifest: %s", err)
	}

	imgSrc, err := i.ref.NewImageSource("", i.tlsVerify())
	if err != nil {
		return specsv1.Image{}, fmt.Errorf("creating image source: %s", err)
	}

	stream, err := i.cacheDriver.Blob(
		logger, manifest.Config.Digest,
		func(logger lager.Logger) (io.ReadCloser, error) {
			stream, _, err := imgSrc.GetBlob(manifest.Config.Digest)
			if err != nil {
				return nil, fmt.Errorf("fetching config blob: %s", err)
			}

			return stream, nil
		},
	)
	if err != nil {
		return specsv1.Image{}, err
	}

	var config specsv1.Image
	if err := json.NewDecoder(stream).Decode(&config); err != nil {
		return specsv1.Image{}, fmt.Errorf("parsing image config: %s", err)
	}

	return config, nil
}

func (i *ContainersImage) tlsVerify() bool {
	for _, trustedRegistry := range i.trustedRegistries {
		if strings.HasPrefix(i.ref.StringWithinTransport(), "//"+trustedRegistry) {
			return false
		}
	}
	return true
}
