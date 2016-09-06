package unpacker_test

import (
	"errors"
	"os/exec"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/image_puller/unpacker"
	"github.com/cloudfoundry/gunk/command_runner/fake_command_runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/st3v/glager"
)

var _ = Describe("IDMapper", func() {
	var (
		fakeCmdRunner *fake_command_runner.FakeCommandRunner
		idMapper      *unpacker.CommandIDMapper
		logger        *TestLogger
	)

	BeforeEach(func() {
		fakeCmdRunner = fake_command_runner.New()
		idMapper = unpacker.NewIDMapper(fakeCmdRunner)
		logger = NewLogger("idmapper")
	})

	Describe("MapUIDs", func() {

		Context("when mapping is successful", func() {
			BeforeEach(func() {
				fakeCmdRunner.WhenRunning(fake_command_runner.CommandSpec{
					Path: "newuidmap",
				}, func(cmd *exec.Cmd) error {
					// Avoid calling the OS command
					return nil
				})
			})

			It("logs when running newuidmap", func() {
				Expect(idMapper.MapUIDs(logger, 1000, []groot.IDMappingSpec{
					groot.IDMappingSpec{HostID: 10, NamespaceID: 20, Size: 30},
					groot.IDMappingSpec{HostID: 100, NamespaceID: 200, Size: 300},
				})).To(Succeed())

				Expect(logger).To(ContainSequence(
					Debug(
						Message("idmapper.mapUID.starting-id-map"),
						Data("path", "/usr/bin/newuidmap"),
						Data("args", []string{"newuidmap", "1000", "20", "10", "30", "200", "100", "300"}),
					),
				))
			})

			It("uses the newuidmap correctly", func() {
				Expect(idMapper.MapUIDs(logger, 1000, []groot.IDMappingSpec{
					groot.IDMappingSpec{HostID: 10, NamespaceID: 20, Size: 30},
					groot.IDMappingSpec{HostID: 100, NamespaceID: 200, Size: 300},
				})).To(Succeed())

				cmds := fakeCmdRunner.ExecutedCommands()
				newuidCmd := cmds[0]

				Expect(newuidCmd.Args[0]).To(Equal("newuidmap"))
				Expect(newuidCmd.Args[1]).To(Equal("1000"))
				Expect(newuidCmd.Args[2:]).To(Equal([]string{"20", "10", "30", "200", "100", "300"}))
			})
		})

		Context("when mapping the uids fail", func() {
			BeforeEach(func() {
				fakeCmdRunner.WhenRunning(fake_command_runner.CommandSpec{
					Path: "newuidmap",
				}, func(cmd *exec.Cmd) error {
					cmd.Stdout.Write([]byte("invalid mapping"))
					return errors.New("exit code 1")
				})
			})

			It("fails", func() {
				Expect(idMapper.MapUIDs(logger, 1000, []groot.IDMappingSpec{
					groot.IDMappingSpec{HostID: 10, NamespaceID: 20, Size: 30},
				})).To(MatchError(ContainSubstring("exit code 1")))
			})

			It("includes the command output in the error", func() {
				Expect(idMapper.MapUIDs(logger, 1000, []groot.IDMappingSpec{
					groot.IDMappingSpec{HostID: 10, NamespaceID: 20, Size: 30},
				})).To(MatchError(ContainSubstring("invalid mapping")))
			})
		})
	})

	Describe("MapGIDs", func() {
		Context("when mapping is successful", func() {
			BeforeEach(func() {
				fakeCmdRunner.WhenRunning(fake_command_runner.CommandSpec{
					Path: "newuidmap",
				}, func(cmd *exec.Cmd) error {
					// Avoid calling the OS command
					return nil
				})
			})

			It("logs when running newuidmap", func() {
				Expect(idMapper.MapGIDs(logger, 1000, []groot.IDMappingSpec{
					groot.IDMappingSpec{HostID: 10, NamespaceID: 20, Size: 30},
					groot.IDMappingSpec{HostID: 100, NamespaceID: 200, Size: 300},
				})).To(Succeed())

				Expect(logger).To(ContainSequence(
					Debug(
						Message("idmapper.mapGID.starting-id-map"),
						Data("path", "/usr/bin/newgidmap"),
						Data("args", []string{"newgidmap", "1000", "20", "10", "30", "200", "100", "300"}),
					),
				))
			})

			It("uses the newgidmap correctly", func() {
				Expect(idMapper.MapGIDs(logger, 1000, []groot.IDMappingSpec{
					groot.IDMappingSpec{HostID: 50, NamespaceID: 60, Size: 70},
					groot.IDMappingSpec{HostID: 400, NamespaceID: 500, Size: 600},
				})).To(Succeed())

				cmds := fakeCmdRunner.ExecutedCommands()
				newgidCmd := cmds[0]

				Expect(newgidCmd.Args[0]).To(Equal("newgidmap"))
				Expect(newgidCmd.Args[1]).To(Equal("1000"))
				Expect(newgidCmd.Args[2:]).To(Equal([]string{"60", "50", "70", "500", "400", "600"}))
			})
		})

		Context("when mapping the gids fail", func() {
			BeforeEach(func() {
				fakeCmdRunner.WhenRunning(fake_command_runner.CommandSpec{
					Path: "newgidmap",
				}, func(cmd *exec.Cmd) error {
					cmd.Stdout.Write([]byte("invalid mapping"))
					return errors.New("exit code 1")
				})
			})

			It("fails", func() {
				Expect(idMapper.MapGIDs(logger, 1000, []groot.IDMappingSpec{
					groot.IDMappingSpec{HostID: 10, NamespaceID: 20, Size: 30},
				})).To(MatchError(ContainSubstring("exit code 1")))
			})

			It("includes the command output in the error", func() {
				Expect(idMapper.MapGIDs(logger, 1000, []groot.IDMappingSpec{
					groot.IDMappingSpec{HostID: 10, NamespaceID: 20, Size: 30},
				})).To(MatchError(ContainSubstring("invalid mapping")))
			})
		})
	})
})
