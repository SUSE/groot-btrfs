package driver

import (
	"bytes"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"code.cloudfoundry.org/lager"
	errorspkg "github.com/pkg/errors"
	"github.com/tscolari/lagregator"
)

func (d *Driver) applyDiskLimit(logger lager.Logger, diskLimit int64, imageRootfsPath string) error {
	logger = logger.Session("applying-quotas", lager.Data{"diskLimit": diskLimit})
	logger.Info("starting")
	defer logger.Info("ending")

	if diskLimit == 0 {
		logger.Debug("no-need-for-quotas")
		return nil
	}

	args := []string{
		"--btrfs-bin", d.conf.BtrfsBinPath(),
		"limit",
		"--volume-path", imageRootfsPath,
		"--disk-limit-bytes", strconv.FormatInt(diskLimit, 10),
	}

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

	cmd := exec.Command(d.conf.DraxBinPath, args...)
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
	if _, err := exec.LookPath(d.conf.DraxBinPath); err != nil {
		return false
	}
	return true
}

func (d *Driver) hasSUID() bool {
	path, err := exec.LookPath(d.conf.DraxBinPath)
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
