package driver

// import "code.cloudfoundry.org/grootfs/store/filesystems/btrfs"

import (
//	"bytes"
//	"encoding/json"
//	"fmt"
//	"io/ioutil"
//	"os"
//	"os/exec"
//	"path"
//	"path/filepath"
//	"regexp"
//	"strconv"
//	"strings"
//
//	"github.com/tscolari/lagregator"
//
//	"code.cloudfoundry.org/groot"
//	"code.cloudfoundry.org/lager"
//	errorspkg "github.com/pkg/errors"
)

const (
	BtrfsType = 0x9123683E
)

type Driver struct {
	volumesDirName string
	draxBinPath    string
	btrfsBinPath   string
	mkfsBinPath    string
	storePath      string
}

type DriverConfig struct {
	VolumesDirName string
	BtrfsBinPath   string
	MkfsBinPath    string
	DraxBinPath    string
	StorePath      string
}

func NewDriver(conf *DriverConfig) *Driver {
	return &Driver{
		volumesDirName: conf.VolumesDirName,
		btrfsBinPath:   conf.BtrfsBinPath,
		mkfsBinPath:    conf.MkfsBinPath,
		draxBinPath:    conf.DraxBinPath,
		storePath:      conf.StorePath,
	}
}

/*func (d *Driver) applyDiskLimit(logger lager.Logger, spec image_cloner.ImageDriverSpec) error {
	logger = logger.Session("applying-quotas", lager.Data{"spec": spec})
	logger.Info("starting")
	defer logger.Info("ending")

	if spec.DiskLimit == 0 {
		logger.Debug("no-need-for-quotas")
		return nil
	}

	args := []string{
		"--btrfs-bin", d.btrfsBinPath,
		"limit",
		"--volume-path", filepath.Join(spec.ImagePath, "rootfs"),
		"--disk-limit-bytes", strconv.FormatInt(spec.DiskLimit, 10),
	}

	if spec.ExclusiveDiskLimit {
		args = append(args, "--exclude-image-from-quota")
	}

	if _, err := d.runDrax(logger, args...); err != nil {
		return err
	}

	return nil
}*/
