package driver

import (
	"bytes"
	"code.cloudfoundry.org/lager"
	errorspkg "github.com/pkg/errors"
	"github.com/tscolari/lagregator"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	BtrfsType               = 0x9123683E
	utimeOmit         int64 = ((1 << 30) - 2)
	atSymlinkNoFollow int   = 0x100
)

type Driver struct {
	conf *DriverConfig
}

type DriverConfig struct {
	VolumesDirName string
	BtrfsBinPath   string
	MkfsBinPath    string
	DraxBinPath    string
	StorePath      string
	UIDMapping     []string
	GIDMapping     []string
}

func NewDriver(conf *DriverConfig) *Driver {
	return &Driver{conf: conf}
}

func (d *Driver) UIDMappings() (MappingList, error) {
	UIDMapping, err := NewMappingList(d.conf.UIDMapping)
	if err != nil {
		return nil, err
	}

	return UIDMapping, nil
}

func (d *Driver) GIDMappings() (MappingList, error) {
	GIDMapping, err := NewMappingList(d.conf.GIDMapping)
	if err != nil {
		return nil, err
	}

	return GIDMapping, nil
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
		"--btrfs-bin", d.conf.BtrfsBinPath,
		"limit",
		"--volume-path", filepath.Join(d.conf.StorePath, "rootfs"),
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

func changeModTime(path string, modTime time.Time) error {
	var _path *byte
	_path, err := syscall.BytePtrFromString(path)
	if err != nil {
		return err
	}

	ts := []syscall.Timespec{
		syscall.Timespec{Sec: 0, Nsec: utimeOmit},
		syscall.NsecToTimespec(modTime.UnixNano()),
	}

	atFdCwd := -100
	_, _, errno := syscall.Syscall6(
		syscall.SYS_UTIMENSAT,
		uintptr(atFdCwd),
		uintptr(unsafe.Pointer(_path)),
		uintptr(unsafe.Pointer(&ts[0])),
		uintptr(atSymlinkNoFollow),
		0, 0,
	)
	if errno == syscall.ENOSYS {
		return os.Chtimes(path, time.Now(), modTime)
	}

	if errno != 0 {
		return errno
	}

	return nil
}
