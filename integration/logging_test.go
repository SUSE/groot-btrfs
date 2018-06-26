package integration_test

import (
	"io/ioutil"
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/SUSE/groot-btrfs/groot"
	"github.com/SUSE/groot-btrfs/integration"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Logging", func() {
	var spec groot.CreateSpec

	BeforeEach(func() {
		spec = groot.CreateSpec{
			ID:           "my-image",
			BaseImageURL: integration.String2URL("/non/existent/rootfs"),
			Mount:        true,
		}
	})

	It("forwards human ouput to stdout", func() {
		buffer := gbytes.NewBuffer()

		_, err := Runner.WithStdout(buffer).Create(spec)
		Expect(err).To(HaveOccurred())

		Eventually(buffer).Should(gbytes.Say("no such file or directory"))
	})

	Describe("--log-level and --log-file flags", func() {
		Context("when the --log-file is not set", func() {
			Context("and --log-level is set", func() {
				It("writes logs to stderr", func() {
					buffer := gbytes.NewBuffer()

					_, err := Runner.WithStderr(buffer).WithLogLevel(lager.DEBUG).Create(spec)
					Expect(err).To(HaveOccurred())

					Expect(buffer).To(gbytes.Say(`"error":".*no such file or directory"`))
				})
			})

			Context("and --log-level is not set", func() {
				It("does not write anything to stderr", func() {
					buffer := gbytes.NewBuffer()

					_, err := Runner.WithStderr(buffer).WithoutLogLevel().Create(spec)
					Expect(err).To(HaveOccurred())

					Expect(buffer.Contents()).To(BeEmpty())
				})
			})
		})

		Context("when the --log-file is set", func() {
			var (
				logFilePath string
			)

			BeforeEach(func() {
				logFile, err := ioutil.TempFile("", "log")
				Expect(err).NotTo(HaveOccurred())
				logFilePath = logFile.Name()
				Expect(os.Chmod(logFilePath, 0777)).To(Succeed())
			})

			AfterEach(func() {
				Expect(os.RemoveAll(logFilePath)).To(Succeed())
			})

			Context("and --log-level is set", func() {
				It("forwards logs to the given file", func() {
					_, err := Runner.WithLogFile(logFilePath).WithLogLevel(lager.DEBUG).Create(spec)
					Expect(err).To(HaveOccurred())

					allTheLogs, err := ioutil.ReadFile(logFilePath)
					Expect(err).NotTo(HaveOccurred())
					Expect(string(allTheLogs)).To(ContainSubstring("\"log_level\":0"))
				})
			})

			Context("and --log-level is not set", func() {
				It("forwards logs to the given file with the log level set to INFO", func() {
					_, err := Runner.WithLogFile(logFilePath).WithoutLogLevel().Create(spec)
					Expect(err).To(HaveOccurred())

					allTheLogs, err := ioutil.ReadFile(logFilePath)
					Expect(err).NotTo(HaveOccurred())
					Expect(string(allTheLogs)).NotTo(ContainSubstring("\"log_level\":0"))
					Expect(string(allTheLogs)).To(ContainSubstring("\"log_level\":1"))
				})
			})

			Context("and the log file cannot be created", func() {
				It("returns an error to stderr", func() {
					buffer := gbytes.NewBuffer()

					_, err := Runner.WithStderr(buffer).WithLogFile("/path/to/log_file.log").Create(spec)
					Expect(err).To(HaveOccurred())

					Expect(buffer).To(gbytes.Say("no such file or directory"))
				})
			})
		})
	})
})
