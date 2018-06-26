package namespaced_test

import (
	"errors"
	"os"
	"os/exec"

	"code.cloudfoundry.org/commandrunner/fake_command_runner"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/SUSE/groot-btrfs/base_image_puller"
	"github.com/SUSE/groot-btrfs/base_image_puller/unpacker/unpackerfakes"
	"github.com/SUSE/groot-btrfs/groot"
	"github.com/SUSE/groot-btrfs/integration"
	"github.com/SUSE/groot-btrfs/store/filesystems/namespaced"
	"github.com/SUSE/groot-btrfs/store/filesystems/namespaced/namespacedfakes"
	"github.com/SUSE/groot-btrfs/store/image_cloner"

	"github.com/containers/storage/pkg/reexec"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func init() {
	if reexec.Init() {
		os.Exit(0)
	}
}

var _ = Describe("Driver", func() {
	var (
		internalDriver    *namespacedfakes.FakeInternalDriver
		idMapper          *unpackerfakes.FakeIDMapper
		driver            *namespaced.Driver
		logger            lager.Logger
		idMappings        groot.IDMappings
		fakeCommandRunner *fake_command_runner.FakeCommandRunner
	)

	BeforeEach(func() {
		fakeCommandRunner = fake_command_runner.New()
		idMappings = groot.IDMappings{
			UIDMappings: []groot.IDMappingSpec{
				{HostID: 100, NamespaceID: 1000, Size: 10},
			},
			GIDMappings: []groot.IDMappingSpec{
				{HostID: 200, NamespaceID: 2000, Size: 20},
			},
		}
	})

	JustBeforeEach(func() {
		internalDriver = new(namespacedfakes.FakeInternalDriver)
		idMapper = new(unpackerfakes.FakeIDMapper)
		driver = namespaced.New(internalDriver, idMappings, idMapper, fakeCommandRunner)
		logger = lagertest.NewTestLogger("driver")
	})

	Describe("VolumePath", func() {
		JustBeforeEach(func() {
			internalDriver.VolumePathReturns("abc", errors.New("error"))
		})

		It("decorates the internal driver function", func() {
			path, err := driver.VolumePath(logger, "123")
			Expect(path).To(Equal("abc"))
			Expect(err).To(MatchError("error"))
			Expect(internalDriver.VolumePathCallCount()).To(Equal(1))
			loggerArg, id := internalDriver.VolumePathArgsForCall(0)
			Expect(loggerArg).To(Equal(logger))
			Expect(id).To(Equal("123"))
		})
	})

	Describe("CreateVolume", func() {
		JustBeforeEach(func() {
			internalDriver.CreateVolumeReturns("abc", errors.New("error"))
		})

		It("decorates the internal driver function", func() {
			path, err := driver.CreateVolume(logger, "123", "456")
			Expect(path).To(Equal("abc"))
			Expect(err).To(MatchError("error"))
			Expect(internalDriver.CreateVolumeCallCount()).To(Equal(1))
			loggerArg, parentId, id := internalDriver.CreateVolumeArgsForCall(0)
			Expect(loggerArg).To(Equal(logger))
			Expect(parentId).To(Equal("123"))
			Expect(id).To(Equal("456"))
		})
	})

	Describe("DestroyVolume", func() {
		var (
			commandError error
			reexecOutput string
		)

		BeforeEach(func() {
			commandError = nil
			reexecOutput = ""
		})

		JustBeforeEach(func() {
			fakeCommandRunner.WhenRunning(fake_command_runner.CommandSpec{
				Path: "/proc/self/exe",
			}, func(cmd *exec.Cmd) error {
				cmd.Process = &os.Process{
					Pid: 12, // don't panic
				}

				return nil
			})

			fakeCommandRunner.WhenWaitingFor(fake_command_runner.CommandSpec{
				Path: "/proc/self/exe",
			}, func(cmd *exec.Cmd) error {
				if reexecOutput != "" {
					_, err := cmd.Stdout.Write([]byte(reexecOutput))
					Expect(err).NotTo(HaveOccurred())
				}

				return commandError
			})

			internalDriver.MarshalReturns([]byte(`{"super-cool":"json"}`), nil)
		})

		Context("when the running user is root", func() {
			BeforeEach(func() {
				integration.SkipIfNonRoot(os.Getuid())
			})

			JustBeforeEach(func() {
				internalDriver.DestroyVolumeReturns(errors.New("error"))
			})

			It("doesn't call the IDMapper", func() {
				_ = driver.DestroyVolume(logger, "123")
				Expect(idMapper.MapGIDsCallCount()).To(BeZero())
				Expect(idMapper.MapUIDsCallCount()).To(BeZero())
			})

			It("decorates the internal driver function", func() {
				err := driver.DestroyVolume(logger, "123")
				Expect(err).To(MatchError("error"))
				Expect(internalDriver.DestroyVolumeCallCount()).To(Equal(1))
				loggerArg, id := internalDriver.DestroyVolumeArgsForCall(0)
				Expect(loggerArg).To(Equal(logger))
				Expect(id).To(Equal("123"))
			})
		})

		Context("when the running user is not root", func() {
			BeforeEach(func() {
				integration.SkipIfRoot(os.Getuid())
			})

			It("reexecs with the correct arguments", func() {
				Expect(driver.DestroyVolume(logger, "123")).To(Succeed())

				cmds := fakeCommandRunner.StartedCommands()
				Expect(cmds).To(HaveLen(1))

				Expect(cmds[0].Args).To(ConsistOf([]string{"with-caps-in-userns", "destroy-volume", `{"super-cool":"json"}`, "123"}))
			})

			It("uses idMapper to map the all the ids of the reexec process", func() {
				Expect(driver.DestroyVolume(logger, "123")).To(Succeed())

				Expect(idMapper.MapUIDsCallCount()).To(Equal(1))
				_, pid, uidMappings := idMapper.MapUIDsArgsForCall(0)
				Expect(pid).To(Equal(12))
				Expect(uidMappings).To(Equal([]groot.IDMappingSpec{
					{HostID: 100, NamespaceID: 1000, Size: 10},
				}))

				Expect(idMapper.MapGIDsCallCount()).To(Equal(1))
				_, pid, gidMappings := idMapper.MapGIDsArgsForCall(0)
				Expect(pid).To(Equal(12))
				Expect(gidMappings).To(Equal([]groot.IDMappingSpec{
					{HostID: 200, NamespaceID: 2000, Size: 20},
				}))
			})

		})

		Context("when the idmappings are empty", func() {
			BeforeEach(func() {
				idMappings = groot.IDMappings{}
			})

			JustBeforeEach(func() {
				internalDriver.DestroyVolumeReturns(errors.New("error"))
			})

			It("doesn't call the IDMapper", func() {
				_ = driver.DestroyVolume(logger, "123")
				Expect(idMapper.MapGIDsCallCount()).To(BeZero())
				Expect(idMapper.MapUIDsCallCount()).To(BeZero())
			})

			It("decorates the internal driver function", func() {
				err := driver.DestroyVolume(logger, "123")
				Expect(err).To(MatchError("error"))
				Expect(internalDriver.DestroyVolumeCallCount()).To(Equal(1))
				loggerArg, id := internalDriver.DestroyVolumeArgsForCall(0)
				Expect(loggerArg).To(Equal(logger))
				Expect(id).To(Equal("123"))
			})
		})
	})

	Describe("Volumes", func() {
		JustBeforeEach(func() {
			internalDriver.VolumesReturns([]string{"abc"}, errors.New("error"))
		})

		It("decorates the internal driver function", func() {
			paths, err := driver.Volumes(logger)
			Expect(paths).To(Equal([]string{"abc"}))
			Expect(err).To(MatchError("error"))
			Expect(internalDriver.VolumesCallCount()).To(Equal(1))
			loggerArg := internalDriver.VolumesArgsForCall(0)
			Expect(loggerArg).To(Equal(logger))
		})
	})

	Describe("MoveVolume", func() {
		JustBeforeEach(func() {
			internalDriver.MoveVolumeReturns(errors.New("error"))
		})

		It("decorates the internal driver function", func() {
			err := driver.MoveVolume(logger, "123", "456")
			Expect(err).To(MatchError("error"))
			Expect(internalDriver.MoveVolumeCallCount()).To(Equal(1))
			loggerArg, from, to := internalDriver.MoveVolumeArgsForCall(0)
			Expect(loggerArg).To(Equal(logger))
			Expect(from).To(Equal("123"))
			Expect(to).To(Equal("456"))
		})
	})

	Describe("WriteVolumeMeta", func() {
		JustBeforeEach(func() {
			internalDriver.WriteVolumeMetaReturns(errors.New("error"))
		})

		It("decorates the internal driver function", func() {
			volMeta := base_image_puller.VolumeMeta{Size: 1000}
			err := driver.WriteVolumeMeta(logger, "123", volMeta)
			Expect(err).To(MatchError("error"))
			Expect(internalDriver.WriteVolumeMetaCallCount()).To(Equal(1))
			loggerArg, id, volMetaArgs := internalDriver.WriteVolumeMetaArgsForCall(0)
			Expect(loggerArg).To(Equal(logger))
			Expect(id).To(Equal("123"))
			Expect(volMetaArgs).To(Equal(volMeta))
		})
	})

	Describe("HandleOpaqueWhiteouts", func() {
		JustBeforeEach(func() {
			internalDriver.HandleOpaqueWhiteoutsReturns(errors.New("error"))
		})

		It("decorates the internal driver function", func() {
			err := driver.HandleOpaqueWhiteouts(logger, "123", []string{"456"})
			Expect(err).To(MatchError("error"))
			Expect(internalDriver.HandleOpaqueWhiteoutsCallCount()).To(Equal(1))
			loggerArg, id, opaques := internalDriver.HandleOpaqueWhiteoutsArgsForCall(0)
			Expect(loggerArg).To(Equal(logger))
			Expect(id).To(Equal("123"))
			Expect(opaques).To(Equal([]string{"456"}))
		})
	})

	Describe("CreateImage", func() {
		JustBeforeEach(func() {
			internalDriver.CreateImageReturns(groot.MountInfo{Destination: "Dimension 31-C"}, errors.New("error"))
		})

		It("decorates the internal driver function", func() {
			mountInfo, err := driver.CreateImage(logger, image_cloner.ImageDriverSpec{Mount: true})
			Expect(mountInfo).To(Equal(groot.MountInfo{Destination: "Dimension 31-C"}))
			Expect(err).To(MatchError("error"))
			Expect(internalDriver.CreateImageCallCount()).To(Equal(1))
			loggerArg, specArg := internalDriver.CreateImageArgsForCall(0)
			Expect(loggerArg).To(Equal(logger))
			Expect(specArg).To(Equal(image_cloner.ImageDriverSpec{Mount: true}))
		})
	})

	Describe("DestroyImage", func() {
		var (
			commandError error
			reexecOutput string
		)

		BeforeEach(func() {
			commandError = nil
			reexecOutput = ""
		})

		JustBeforeEach(func() {
			fakeCommandRunner.WhenRunning(fake_command_runner.CommandSpec{
				Path: "/proc/self/exe",
			}, func(cmd *exec.Cmd) error {
				cmd.Process = &os.Process{
					Pid: 12, // don't panic
				}

				return nil
			})

			fakeCommandRunner.WhenWaitingFor(fake_command_runner.CommandSpec{
				Path: "/proc/self/exe",
			}, func(cmd *exec.Cmd) error {
				if reexecOutput != "" {
					_, err := cmd.Stdout.Write([]byte(reexecOutput))
					Expect(err).NotTo(HaveOccurred())
				}

				return commandError
			})

			internalDriver.MarshalReturns([]byte(`{"super-cool":"json"}`), nil)
		})

		Context("when the running user is root", func() {
			BeforeEach(func() {
				integration.SkipIfNonRoot(os.Getuid())
			})

			JustBeforeEach(func() {
				internalDriver.DestroyImageReturns(errors.New("error"))
			})

			It("doesn't call the IDMapper", func() {
				_ = driver.DestroyImage(logger, "123")
				Expect(idMapper.MapGIDsCallCount()).To(BeZero())
				Expect(idMapper.MapUIDsCallCount()).To(BeZero())
			})

			It("decorates the internal driver function", func() {
				err := driver.DestroyImage(logger, "123")
				Expect(err).To(MatchError("error"))
				Expect(internalDriver.DestroyImageCallCount()).To(Equal(1))
				loggerArg, id := internalDriver.DestroyImageArgsForCall(0)
				Expect(loggerArg).To(Equal(logger))
				Expect(id).To(Equal("123"))
			})
		})

		Context("when the running user is not root", func() {
			BeforeEach(func() {
				integration.SkipIfRoot(os.Getuid())
			})

			It("reexecs with the correct arguments", func() {
				Expect(driver.DestroyImage(logger, "id-1")).To(Succeed())

				cmds := fakeCommandRunner.StartedCommands()
				Expect(cmds).To(HaveLen(1))

				Expect(cmds[0].Args).To(ConsistOf([]string{"with-caps-in-userns", "destroy-image", `{"super-cool":"json"}`, "id-1"}))
			})

			It("uses idMapper to map the all the ids of the reexec process", func() {
				Expect(driver.DestroyImage(logger, "123")).To(Succeed())

				Expect(idMapper.MapUIDsCallCount()).To(Equal(1))
				_, pid, uidMappings := idMapper.MapUIDsArgsForCall(0)
				Expect(pid).To(Equal(12))
				Expect(uidMappings).To(Equal([]groot.IDMappingSpec{
					{HostID: 100, NamespaceID: 1000, Size: 10},
				}))

				Expect(idMapper.MapGIDsCallCount()).To(Equal(1))
				_, pid, gidMappings := idMapper.MapGIDsArgsForCall(0)
				Expect(pid).To(Equal(12))
				Expect(gidMappings).To(Equal([]groot.IDMappingSpec{
					{HostID: 200, NamespaceID: 2000, Size: 20},
				}))
			})
		})

		Context("when the idmappings are empty", func() {
			BeforeEach(func() {
				idMappings = groot.IDMappings{}
			})

			JustBeforeEach(func() {
				internalDriver.DestroyImageReturns(errors.New("error"))
			})

			It("doesn't call the IDMapper", func() {
				_ = driver.DestroyImage(logger, "123")
				Expect(idMapper.MapGIDsCallCount()).To(BeZero())
				Expect(idMapper.MapUIDsCallCount()).To(BeZero())
			})

			It("decorates the internal driver function", func() {
				err := driver.DestroyImage(logger, "123")
				Expect(err).To(MatchError("error"))
				Expect(internalDriver.DestroyImageCallCount()).To(Equal(1))
				loggerArg, id := internalDriver.DestroyImageArgsForCall(0)
				Expect(loggerArg).To(Equal(logger))
				Expect(id).To(Equal("123"))
			})
		})
	})

	Describe("FetchStats", func() {
		JustBeforeEach(func() {
			internalDriver.FetchStatsReturns(groot.VolumeStats{DiskUsage: groot.DiskUsage{TotalBytesUsed: 100}}, errors.New("error"))
		})

		It("decorates the internal driver function", func() {
			stats, err := driver.FetchStats(logger, "id-1")
			Expect(stats).To(Equal(groot.VolumeStats{DiskUsage: groot.DiskUsage{TotalBytesUsed: 100}}))
			Expect(err).To(MatchError("error"))
			Expect(internalDriver.FetchStatsCallCount()).To(Equal(1))
			loggerArg, imageIdArg := internalDriver.FetchStatsArgsForCall(0)
			Expect(loggerArg).To(Equal(logger))
			Expect(imageIdArg).To(Equal("id-1"))
		})
	})
})
