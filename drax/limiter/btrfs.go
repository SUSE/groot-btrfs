package limiter // import "github.com/SUSE/groot-btrfs/drax/limiter"

import (
	"bytes"
	"os/exec"
	"strconv"
	"strings"

	"code.cloudfoundry.org/commandrunner"
	errorspkg "github.com/pkg/errors"

	"code.cloudfoundry.org/lager"
)

type BtrfsLimiter struct {
	commandRunner commandrunner.CommandRunner
	btrfsBin      string
}

func NewBtrfsLimiter(btrfsBin string, commandRunner commandrunner.CommandRunner) *BtrfsLimiter {
	return &BtrfsLimiter{
		commandRunner: commandRunner,
		btrfsBin:      btrfsBin,
	}
}

func (i *BtrfsLimiter) ApplyDiskLimit(logger lager.Logger, path string, diskLimit int64, exclusiveLimit bool) error {
	logger = logger.Session("btrfs-applying-quotas", lager.Data{"path": path, "diskLimit": diskLimit, "exclusiveLimit": exclusiveLimit})
	logger.Info("starting")
	defer logger.Info("ending")

	cmd := exec.Command(i.btrfsBin, i.argsForLimit(path, strconv.FormatInt(diskLimit, 10), exclusiveLimit)...)
	combinedBuffer := bytes.NewBuffer([]byte{})
	cmd.Stdout = combinedBuffer
	cmd.Stderr = combinedBuffer

	logger.Debug("starting-btrfs-command", lager.Data{"cmd": cmd.Path, "args": cmd.Args})
	if err := i.commandRunner.Run(cmd); err != nil {
		logger.Error("command-failed", err, lager.Data{"commandOutput": combinedBuffer.String()})
		return errorspkg.Errorf(strings.TrimSpace(combinedBuffer.String()))
	}

	return nil
}

func (i *BtrfsLimiter) DestroyQuotaGroup(logger lager.Logger, path string) error {
	logger = logger.Session("btrfs-destroying-qgroup", lager.Data{"path": path})
	logger.Info("starting")
	defer logger.Info("ending")

	cmd := exec.Command(i.btrfsBin, "qgroup", "destroy", path, path)
	combinedBuffer := bytes.NewBuffer([]byte{})
	cmd.Stdout = combinedBuffer
	cmd.Stderr = combinedBuffer

	if err := i.commandRunner.Run(cmd); err != nil {
		logger.Error("command-failed", err)
		return errorspkg.Errorf(strings.TrimSpace(combinedBuffer.String()))
	}

	return nil
}

func (i *BtrfsLimiter) argsForLimit(path, diskLimit string, exclusiveLimit bool) []string {
	args := []string{"qgroup", "limit"}
	if exclusiveLimit {
		args = append(args, "-e")
	}

	return append(args, diskLimit, path)
}
