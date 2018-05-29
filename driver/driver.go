package driver

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	wearegroot "code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/store/filesystems"
	"code.cloudfoundry.org/lager"
	errorspkg "github.com/pkg/errors"
	"github.com/tscolari/lagregator"
)

const (
	// BtrfsType is BTRFS_SUPER_MAGIC f_type constant
	// see `man statfs` for more details
	BtrfsType = 0x9123683E

	utimeOmit         int64 = ((1 << 30) - 2)
	atSymlinkNoFollow int   = 0x100
)

// Driver represents a BTRFS groot Driver
type Driver struct {
	conf *Config
}

// Config contains all the configurations required for the driver
type Config struct {
	VolumesDirName      string
	BtrfsProgsPath      string
	MetronEndpoint      string
	CleanThresholdBytes int64
	DraxBinPath         string
	StorePath           string
}

// BtrfsBinPath calculates the path to the btrfs CLI.
func (c *Config) BtrfsBinPath() string {
	return filepath.Join(c.BtrfsProgsPath, "btrfs")
}

// MkfsBinPath calculates the path to the mkfs.btrfs CLI.
func (c *Config) MkfsBinPath() string {
	return filepath.Join(c.BtrfsProgsPath, "mkfs.btrfs")
}

// NewDriver creates a new Driver
func NewDriver(conf *Config) *Driver {
	return &Driver{conf: conf}
}

func (d *Driver) readMappings() (wearegroot.IDMappings, error) {
	storeNamespacer := wearegroot.NewStoreNamespacer(d.conf.StorePath)
	idMappings, err := storeNamespacer.Read()
	if err != nil {
		return wearegroot.IDMappings{}, err
	}

	return idMappings, nil
}

func (d *Driver) parseOwner(idMappings wearegroot.IDMappings) (int, int) {
	uid := os.Getuid()
	gid := os.Getgid()

	for _, mapping := range idMappings.UIDMappings {
		if mapping.Size == 1 && mapping.NamespaceID == 0 {
			uid = int(mapping.HostID)
			break
		}
	}

	for _, mapping := range idMappings.GIDMappings {
		if mapping.Size == 1 && mapping.NamespaceID == 0 {
			gid = int(mapping.HostID)
			break
		}
	}

	return uid, gid
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
		"--btrfs-bin", d.conf.BtrfsBinPath(),
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

func (d *Driver) createVolume(logger lager.Logger, parentID, id string) (string, error) {
	logger = logger.Session("btrfs-creating-volume", lager.Data{"parentID": parentID, "id": id})
	logger.Info("starting")
	defer logger.Info("ending")

	var cmd *exec.Cmd
	volPath := filepath.Join(d.conf.StorePath, d.conf.VolumesDirName, id)

	if parentID == "" {
		cmd = exec.Command(d.conf.BtrfsBinPath(), "subvolume", "create", volPath)
	} else {
		parentVolPath := filepath.Join(d.conf.StorePath, d.conf.VolumesDirName, parentID)
		cmd = exec.Command(d.conf.BtrfsBinPath(), "subvolume", "snapshot", parentVolPath, volPath)
	}

	logger.Debug("starting-btrfs", lager.Data{"path": cmd.Path, "args": cmd.Args})
	if contents, err := cmd.CombinedOutput(); err != nil {
		return "", errorspkg.Wrapf(err, "creating btrfs volume `%s` %s", volPath, string(contents))
	}

	return volPath, nil
}

// MoveVolume uses os.Rename to rename a volume
func (d *Driver) MoveVolume(logger lager.Logger, from, to string) error {
	logger = logger.Session("btrfs-moving-volume", lager.Data{"from": from, "to": to})
	logger.Debug("starting")
	defer logger.Debug("ending")

	if err := os.Rename(from, to); err != nil {
		if !os.IsExist(err) {
			logger.Error("moving-volume-failed", err)
			return errorspkg.Wrap(err, "moving volume")
		}
	}

	return nil
}

// VolumePath returns the path to a specific volume
func (d *Driver) VolumePath(logger lager.Logger, id string) (string, error) {
	volPath := filepath.Join(d.conf.StorePath, d.conf.VolumesDirName, id)
	_, err := os.Stat(volPath)
	if err == nil {
		return volPath, nil
	}

	return "", errorspkg.Wrapf(err, "volume does not exist `%s`", id)
}

// DestroyVolume destroys a BTRFS volume
func (d *Driver) DestroyVolume(logger lager.Logger, id string) error {
	logger = logger.Session("btrfs-destroying-volume", lager.Data{"volumeID": id})
	logger.Info("starting")
	defer logger.Info("ending")

	volumeMetaFilePath := filesystems.VolumeMetaFilePath(d.conf.StorePath, id)
	if err := os.Remove(volumeMetaFilePath); err != nil {
		logger.Error("deleting-metadata-file-failed", err, lager.Data{"path": volumeMetaFilePath})
	}

	return d.destroyBtrfsVolume(logger, filepath.Join(d.conf.StorePath, "volumes", id))
}

// VolumeSize returns the size of a volume in bytes
func (d *Driver) VolumeSize(logger lager.Logger, id string) (int64, error) {
	logger = logger.Session("btrfs-volume-size", lager.Data{"volumeID": id})
	logger.Debug("starting")
	defer logger.Debug("ending")

	return filesystems.VolumeSize(logger, d.conf.StorePath, id)
}

// Volumes returns the list of existing volume IDs
func (d *Driver) Volumes(logger lager.Logger) ([]string, error) {
	logger = logger.Session("btrfs-listing-volumes")
	logger.Debug("starting")
	defer logger.Debug("ending")

	volumes := []string{}
	existingVolumes, err := ioutil.ReadDir(path.Join(d.conf.StorePath, store.VolumesDirName))
	if err != nil {
		return nil, errorspkg.Wrap(err, "failed to list volumes")
	}

	for _, volumeInfo := range existingVolumes {
		volumes = append(volumes, volumeInfo.Name())
	}
	return volumes, nil
}

// ImageIDs returns the list of existing image IDs
func (d *Driver) ImageIDs(logger lager.Logger) ([]string, error) {
	images := []string{}

	existingImages, err := ioutil.ReadDir(path.Join(d.conf.StorePath, store.ImageDirName))
	if err != nil {
		return nil, errorspkg.Wrap(err, "failed to read images dir")
	}

	for _, imageInfo := range existingImages {
		images = append(images, imageInfo.Name())
	}

	return images, nil
}

func (d *Driver) destroyBtrfsVolume(logger lager.Logger, path string) error {
	logger = logger.Session("destroying-subvolume", lager.Data{"path": path})
	logger.Info("starting")
	defer logger.Info("ending")

	if _, err := os.Stat(path); err != nil {
		return errorspkg.Wrap(err, "image path not found")
	}

	if err := d.destroyQgroup(logger, path); err != nil {
		logger.Error("destroying-quota-groups-failed", err, lager.Data{
			"warning": "could not delete quota group"})
	}

	cmd := exec.Command(d.conf.BtrfsBinPath(), "subvolume", "delete", path)
	logger.Debug("starting-btrfs", lager.Data{"path": cmd.Path, "args": cmd.Args})
	if contents, err := cmd.CombinedOutput(); err != nil {
		logger.Error("btrfs-failed", err)
		return errorspkg.Wrapf(err, "destroying volume %s", strings.TrimSpace(string(contents)))
	}
	return nil
}

func cleanWhiteoutDir(path string) error {
	contents, err := ioutil.ReadDir(path)
	if err != nil {
		return errorspkg.Wrap(err, "reading whiteout directory")
	}

	for _, content := range contents {
		if err := os.RemoveAll(filepath.Join(path, content.Name())); err != nil {
			return errorspkg.Wrap(err, "cleaning up whiteout directory")
		}
	}

	return nil
}

func (d *Driver) destroyQgroup(logger lager.Logger, path string) error {
	_, err := d.runDrax(logger, "--btrfs-bin", d.conf.BtrfsBinPath(), "destroy", "--volume-path", path)

	return err
}

func (d *Driver) handleOpaqueWhiteouts(logger lager.Logger, id string, opaqueWhiteouts []string) error {
	volumePath, err := d.VolumePath(logger, id)
	if err != nil {
		return err
	}

	for _, opaqueWhiteout := range opaqueWhiteouts {
		parentDir := path.Dir(filepath.Join(volumePath, opaqueWhiteout))
		if err := cleanWhiteoutDir(parentDir); err != nil {
			return err
		}
	}

	return nil
}

func (d *Driver) imagePath(id string) string {
	return path.Join(d.conf.StorePath, store.ImageDirName, id)
}
