package driver

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"code.cloudfoundry.org/groot"
	"code.cloudfoundry.org/lager"
	errorspkg "github.com/pkg/errors"
)

// Stats returns volume stats for a specific bundle
func (d *Driver) Stats(logger lager.Logger, bundleID string) (returnStats groot.VolumeStats, returnError error) {
	imagePath := d.imagePath(bundleID)

	logger = logger.Session("btrfs-fetching-stats", lager.Data{"imagePath": imagePath})
	logger.Debug("starting")
	defer logger.Debug("ending")

	var stats groot.VolumeStats

	lockFile, err := d.sharedLock.Lock(LockKey)
	if err != nil {
		return stats, errorspkg.Wrap(err, "obtaining a lock")
	}
	defer func() {
		if err = d.sharedLock.Unlock(lockFile); err != nil {
			logger.Error("failed-to-unlock", err)
		}
	}()

	args := []string{
		"--btrfs-bin", d.conf.BtrfsBinPath(),
		"stats",
		"--volume-path", filepath.Join(imagePath, "rootfs"),
		"--force-sync",
	}

	stdoutBuffer, err := d.runDrax(logger, args...)
	if err != nil {
		return groot.VolumeStats{}, err
	}

	usageRegexp := regexp.MustCompile(`.*\s+(\d+)\s+(\d+)$`)
	usage := usageRegexp.FindStringSubmatch(strings.TrimSpace(stdoutBuffer.String()))

	if len(usage) != 3 {
		logger.Error("parsing-stats-failed", errorspkg.Errorf("raw stats: %s", stdoutBuffer.String()))
		return stats, errorspkg.New("could not parse stats")
	}

	fmt.Sscanf(usage[1], "%d", &stats.DiskUsage.TotalBytesUsed)
	fmt.Sscanf(usage[2], "%d", &stats.DiskUsage.ExclusiveBytesUsed)

	return stats, nil
}
