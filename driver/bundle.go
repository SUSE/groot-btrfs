package driver

import (
	"bytes"
	"code.cloudfoundry.org/lager"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tscolari/lagregator"

	specs "github.com/opencontainers/runtime-spec/specs-go"
	errorspkg "github.com/pkg/errors"
)

//func (d *Driver) CreateImage(logger lager.Logger, spec image_cloner.ImageDriverSpec) (groot.MountInfo, error) {
func (d *Driver) Bundle(logger lager.Logger, bundleID string, layerIDs []string, diskLimit int64) (specs.Spec, error) {
	logger = logger.Session("btrfs-creating-snapshot", lager.Data{"IDs": layerIDs})
	logger.Info("starting")
	defer logger.Info("ending")

	toPath := filepath.Join(d.storePath, "rootfs")
	baseVolumePath := filepath.Join(d.storePath, d.volumesDirName, layerIDs[len(layerIDs)-1])

	spec := specs.Spec{
		Version: specs.Version,
		Root: &specs.Root{
			Path: baseVolumePath,
		},
		Linux: &specs.Linux{},
	}

	if err := os.Mkdir(toPath, 0755); err != nil {
		logger.Error("creating-rootfs-folder-failed", err, lager.Data{"rootfs": toPath})
		return specs.Spec{}, errorspkg.Wrap(err, "creating rootfs folder")
	}

	if err := os.Chmod(toPath, 0755); err != nil {
		logger.Error("chmoding-rootfs-folder", err)
		return specs.Spec{}, errorspkg.Wrap(err, "chmoding rootfs folder")
	}

	snapshotPath := filepath.Join(d.storePath, "snapshot")
	cmd := exec.Command(d.btrfsBinPath, "subvolume", "snapshot", baseVolumePath,
		snapshotPath)
	logger.Debug("starting-btrfs", lager.Data{"path": cmd.Path, "args": cmd.Args})
	if contents, err := cmd.CombinedOutput(); err != nil {
		return specs.Spec{}, errorspkg.Errorf(
			"creating btrfs snapshot from `%s` to `%s` (%s): %s",
			baseVolumePath, snapshotPath, err, string(contents),
		)
	}

	if err := os.Chmod(snapshotPath, 0755); err != nil {
		logger.Error("chmoding-snapshot", err)
		return specs.Spec{}, errorspkg.Wrap(err, "chmoding snapshot")
	}

	return spec, d.applyDiskLimit(logger, diskLimit)
}

func (d *Driver) applyDiskLimit(logger lager.Logger, diskLimit int64) error {
	logger = logger.Session("applying-quotas", lager.Data{"diskLimit": diskLimit})
	logger.Info("starting")
	defer logger.Info("ending")

	if diskLimit == 0 {
		logger.Debug("no-need-for-quotas")
		return nil
	}

	args := []string{
		"--btrfs-bin", d.btrfsBinPath,
		"limit",
		"--volume-path", filepath.Join(d.storePath, "rootfs"),
		"--disk-limit-bytes", strconv.FormatInt(diskLimit, 10),
	}

	/*
		if spec.ExclusiveDiskLimit {
			args = append(args, "--exclude-image-from-quota")
		}
	*/

	if _, err := d.runDrax(logger, args...); err != nil {
		return err
	}

	return nil
}

func (d *Driver) runDrax(logger lager.Logger, args ...string) (*bytes.Buffer, error) {
	logger = logger.Session("run-drax", lager.Data{"args": args})
	logger.Debug("starting")
	defer logger.Debug("ending")

	if !d.draxInPath() {
		return nil, errorspkg.New("drax was not found in the $PATH")
	}

	if !d.hasSUID() && os.Geteuid() != 0 {
		return nil, errorspkg.New("missing the setuid bit on drax")
	}

	cmd := exec.Command(d.draxBinPath, args...)
	stdoutBuffer := bytes.NewBuffer([]byte{})
	cmd.Stdout = stdoutBuffer
	cmd.Stderr = lagregator.NewRelogger(logger)

	logger.Debug("starting-drax", lager.Data{"path": cmd.Path, "args": cmd.Args})
	err := cmd.Run()

	if err != nil {
		logger.Error("drax-failed", err)
		return nil, errorspkg.Wrapf(err, " %s", strings.TrimSpace(stdoutBuffer.String()))
	}

	return stdoutBuffer, nil
}

func (d *Driver) draxInPath() bool {
	if _, err := exec.LookPath(d.draxBinPath); err != nil {
		return false
	}
	return true
}

func (d *Driver) hasSUID() bool {
	path, err := exec.LookPath(d.draxBinPath)
	if err != nil {
		return false
	}
	// If LookPath succeeds Stat cannot fail
	draxInfo, _ := os.Stat(path)
	if (draxInfo.Mode() & os.ModeSetuid) == 0 {
		return false
	}
	return true
}
