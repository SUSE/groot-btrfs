package lister_test

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	listerpkg "github.com/SUSE/groot-btrfs/drax/lister"

	"code.cloudfoundry.org/commandrunner/fake_command_runner"
	. "code.cloudfoundry.org/commandrunner/fake_command_runner/matchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var btrfs = os.Getenv("BTRFS")

var _ = Describe("Btrfs", func() {
	var (
		fakeCommandRunner *fake_command_runner.FakeCommandRunner
		lister            *listerpkg.BtrfsLister
		logger            lager.Logger
		list              []byte
		returnError       error
	)

	BeforeEach(func() {
		returnError = nil
		fakeCommandRunner = fake_command_runner.New()

		paths := []string{
			"privileged/images/this/rootfs/snapshot",
			"privileged/images/this/rootfs/subvolume",
			"privileged/images/this/rootfs/subvolume/subvolume",
		}

		list = generateListOutput(paths)
		lister = listerpkg.NewBtrfsLister("custom-btrfs-bin", fakeCommandRunner)
		logger = lagertest.NewTestLogger("drax-lister")
	})

	JustBeforeEach(func() {
		fakeCommandRunner.WhenRunning(fake_command_runner.CommandSpec{
			Path: "custom-btrfs-bin",
			Args: []string{"subvolume", "list", btrfs},
		}, func(cmd *exec.Cmd) error {

			_, err := cmd.Stdout.Write(list)
			Expect(err).NotTo(HaveOccurred())
			return returnError
		})
	})

	Describe("List", func() {
		It("calls the command runner with the correct arguments", func() {
			_, err := lister.List(logger, btrfs)
			Expect(err).ToNot(HaveOccurred())

			Expect(fakeCommandRunner).Should(HaveExecutedSerially(
				fake_command_runner.CommandSpec{
					Path: "custom-btrfs-bin",
					Args: []string{"subvolume", "list", btrfs},
				},
			))
		})

		It("returns the existing subvolumes", func() {
			list, err := lister.List(logger, btrfs)
			Expect(err).ToNot(HaveOccurred())
			Expect(list).To(HaveLen(3))
			Expect(list[0]).To(Equal(btrfs + "/privileged/images/this/rootfs/subvolume/subvolume"))
			Expect(list[1]).To(Equal(btrfs + "/privileged/images/this/rootfs/subvolume"))
			Expect(list[2]).To(Equal(btrfs + "/privileged/images/this/rootfs/snapshot"))
		})
	})

	Context("when the command fails", func() {
		BeforeEach(func() {
			returnError = errors.New("failed")
		})

		It("returns an error", func() {
			_, err := lister.List(logger, btrfs)
			Expect(err).To(MatchError(ContainSubstring("list subvolumes")))
		})
	})

	Context("when the output is not valid", func() {
		BeforeEach(func() {
			list = []byte("super invalid")
		})

		It("returns an error", func() {
			_, err := lister.List(logger, btrfs)
			Expect(err).To(MatchError(ContainSubstring("invalid output")))
		})
	})
})

func generateListOutput(paths []string) []byte {
	var st string
	for _, path := range paths {
		st += fmt.Sprintf("ID 256 gen 11 top level 5 path %s\n", path)
	}
	return []byte(st)
}
