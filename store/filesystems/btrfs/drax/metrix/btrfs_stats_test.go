package metrix_test

import (
	"errors"
	"os/exec"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/SUSE/groot-btrfs/store/filesystems/btrfs/drax/metrix"

	"code.cloudfoundry.org/commandrunner/fake_command_runner"
	. "code.cloudfoundry.org/commandrunner/fake_command_runner/matchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Stats", func() {
	var (
		stats []byte

		fakeCommandRunner *fake_command_runner.FakeCommandRunner
		btrfsStats        *metrix.BtrfsStats
		logger            lager.Logger
	)

	BeforeEach(func() {
		stats = []byte(`qgroupid         rfer         excl\n--------         ----         ----\n0/259         2113536      1064960`)

		fakeCommandRunner = fake_command_runner.New()
		btrfsStats = metrix.NewBtrfsStats("custom-btrfs-bin", fakeCommandRunner)
		logger = lagertest.NewTestLogger("drax-limiter")
	})

	Describe("VolumeStats", func() {
		BeforeEach(func() {
			fakeCommandRunner.WhenRunning(fake_command_runner.CommandSpec{
				Path: "custom-btrfs-bin",
				Args: []string{"qgroup", "show", "--raw", "-F", "/full/path/to/volume"},
			}, func(cmd *exec.Cmd) error {
				_, err := cmd.Stdout.Write(stats)
				Expect(err).NotTo(HaveOccurred())
				return nil
			})
		})

		It("returns the parsed output", func() {
			m, err := btrfsStats.VolumeStats(logger, "/full/path/to/volume", false)
			Expect(err).ToNot(HaveOccurred())

			Expect(m).To(Equal(stats))
		})

		It("runs the correct btrfs command", func() {
			_, err := btrfsStats.VolumeStats(logger, "/full/path/to/volume", false)
			Expect(err).ToNot(HaveOccurred())

			Expect(fakeCommandRunner).Should(HaveExecutedSerially(fake_command_runner.CommandSpec{
				Path: "custom-btrfs-bin",
				Args: []string{"qgroup", "show", "--raw", "-F", "/full/path/to/volume"},
			}))
		})

		Context("when force-sync is not given", func() {
			It("does not sync the filesystem", func() {
				_, err := btrfsStats.VolumeStats(logger, "/full/path/to/volume", false)
				Expect(err).ToNot(HaveOccurred())

				Expect(fakeCommandRunner).ShouldNot(HaveExecutedSerially(
					fake_command_runner.CommandSpec{
						Path: "custom-btrfs-bin",
						Args: []string{"filesystem", "sync", "/full/path/to/volume"},
					},
				))
			})
		})

		Context("when force-sync is given", func() {
			It("forces the filesystem to sync", func() {
				_, err := btrfsStats.VolumeStats(logger, "/full/path/to/volume", true)
				Expect(err).ToNot(HaveOccurred())

				Expect(fakeCommandRunner).Should(HaveExecutedSerially(
					fake_command_runner.CommandSpec{
						Path: "custom-btrfs-bin",
						Args: []string{"filesystem", "sync", "/full/path/to/volume"},
					},
					fake_command_runner.CommandSpec{
						Path: "custom-btrfs-bin",
						Args: []string{"qgroup", "show", "--raw", "-F", "/full/path/to/volume"},
					},
				))
			})

			Context("when filesystem sync fails", func() {
				BeforeEach(func() {
					fakeCommandRunner.WhenRunning(fake_command_runner.CommandSpec{
						Path: "custom-btrfs-bin",
						Args: []string{"filesystem", "sync", "/full/path/to/volume"},
					}, func(cmd *exec.Cmd) error {
						_, err := cmd.Stdout.Write([]byte("failed to sync stuff"))
						Expect(err).NotTo(HaveOccurred())
						_, err = cmd.Stderr.Write([]byte("some stderr text"))
						Expect(err).NotTo(HaveOccurred())
						return errors.New("super error")
					})
				})

				It("returns an error", func() {
					_, err := btrfsStats.VolumeStats(logger, "/full/path/to/volume", true)
					Expect(err).To(MatchError(ContainSubstring("failed to sync stuff")))
					Expect(err).To(MatchError(ContainSubstring("some stderr text")))
				})
			})
		})

		Context("when checking the path fails", func() {
			BeforeEach(func() {
				fakeCommandRunner.WhenRunning(fake_command_runner.CommandSpec{
					Path: "custom-btrfs-bin",
					Args: []string{"subvolume", "show", "/full/path/to/volume"},
				}, func(cmd *exec.Cmd) error {
					_, err := cmd.Stdout.Write([]byte("failed to show stuff"))
					Expect(err).NotTo(HaveOccurred())
					_, err = cmd.Stderr.Write([]byte("some stderr text"))
					Expect(err).NotTo(HaveOccurred())
					return errors.New("super error")
				})
			})

			It("returns an error", func() {
				_, err := btrfsStats.VolumeStats(logger, "/full/path/to/volume", true)
				Expect(err).To(MatchError(ContainSubstring("failed to show stuff")))
				Expect(err).To(MatchError(ContainSubstring("some stderr text")))
			})
		})

		Context("when the path is not a volume", func() {
			BeforeEach(func() {
				fakeCommandRunner.WhenRunning(fake_command_runner.CommandSpec{
					Path: "custom-btrfs-bin",
					Args: []string{"subvolume", "show", "/full/path/to/volume"},
				}, func(cmd *exec.Cmd) error {
					_, err := cmd.Stdout.Write([]byte("failed to show stuff"))
					Expect(err).NotTo(HaveOccurred())
					_, err = cmd.Stderr.Write([]byte("not a subvolume"))
					Expect(err).NotTo(HaveOccurred())
					return errors.New("super error")
				})
			})

			It("returns an error", func() {
				_, err := btrfsStats.VolumeStats(logger, "/full/path/to/volume", true)
				Expect(err).To(MatchError(ContainSubstring("is not a btrfs volume")))
			})
		})

		Context("when qgroup fails", func() {
			BeforeEach(func() {
				fakeCommandRunner = fake_command_runner.New()
				btrfsStats = metrix.NewBtrfsStats("custom-btrfs-bin", fakeCommandRunner)
				fakeCommandRunner.WhenRunning(fake_command_runner.CommandSpec{
					Path: "custom-btrfs-bin",
					Args: []string{"qgroup", "show", "--raw", "-F", "/full/path/to/volume"},
				}, func(cmd *exec.Cmd) error {
					_, err := cmd.Stdout.Write([]byte("failed to sync stuff"))
					Expect(err).NotTo(HaveOccurred())
					_, err = cmd.Stderr.Write([]byte("some stderr text"))
					Expect(err).NotTo(HaveOccurred())
					return errors.New("super error")
				})
			})

			It("returns an error", func() {
				_, err := btrfsStats.VolumeStats(logger, "/full/path/to/volume", true)
				Expect(err).To(MatchError(ContainSubstring("some stderr text")))
				Expect(err).To(MatchError(ContainSubstring("failed to sync stuff")))
			})
		})
	})
})
