package tar_fetcher // import "github.com/SUSE/groot-btrfs/fetcher/tar_fetcher"

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/SUSE/groot-btrfs/groot"
	errorspkg "github.com/pkg/errors"
)

type TarFetcher struct {
}

func NewTarFetcher() *TarFetcher {
	return &TarFetcher{}
}

func (l *TarFetcher) StreamBlob(logger lager.Logger, baseImageURL *url.URL,
	layerInfo groot.LayerInfo) (io.ReadCloser, int64, error) {
	logger = logger.Session("stream-blob", lager.Data{
		"baseImageURL": baseImageURL.String(),
		"source":       layerInfo.BlobID,
	})
	logger.Info("starting")
	defer logger.Info("ending")

	baseImagePath := baseImageURL.String()
	if _, err := os.Stat(baseImagePath); err != nil {
		return nil, 0, errorspkg.Wrapf(err, "local image not found in `%s`", baseImagePath)
	}

	if err := l.validateBaseImage(baseImagePath); err != nil {
		return nil, 0, errorspkg.Wrap(err, "invalid base image")
	}

	logger.Debug("opening-tar", lager.Data{"baseImagePath": baseImagePath})
	stream, err := os.Open(baseImagePath)
	if err != nil {
		return nil, 0, errorspkg.Wrap(err, "reading local image")
	}

	return stream, 0, nil
}

func (l *TarFetcher) BaseImageInfo(logger lager.Logger, baseImageURL *url.URL) (groot.BaseImageInfo, error) {
	logger = logger.Session("layers-digest", lager.Data{"baseImageURL": baseImageURL.String()})
	logger.Info("starting")
	defer logger.Info("ending")

	stat, err := os.Stat(baseImageURL.String())
	if err != nil {
		return groot.BaseImageInfo{},
			errorspkg.Wrap(err, "fetching image timestamp")
	}

	return groot.BaseImageInfo{
		LayerInfos: []groot.LayerInfo{
			groot.LayerInfo{
				BlobID:        baseImageURL.String(),
				ParentChainID: "",
				ChainID:       l.generateChainID(baseImageURL.String(), stat.ModTime().UnixNano()),
			},
		},
	}, nil
}

func (l *TarFetcher) generateChainID(baseImagePath string, timestamp int64) string {
	baseImagePathSha := sha256.Sum256([]byte(baseImagePath))
	return fmt.Sprintf("%s-%d", hex.EncodeToString(baseImagePathSha[:32]), timestamp)
}

func (l *TarFetcher) validateBaseImage(baseImagePath string) error {
	stat, err := os.Stat(baseImagePath)
	if err != nil {
		return err
	}

	if stat.IsDir() {
		return errorspkg.New("directory provided instead of a tar file")
	}

	return nil
}
