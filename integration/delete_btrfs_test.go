package integration_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"syscall"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"
	"code.cloudfoundry.org/lager"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Delete (btrfs only)", func() {
	var (
		sourceImagePath string
		baseImagePath   string
		image           groot.Image
	)

	BeforeEach(func() {
		integration.SkipIfNotBTRFS(Driver)

		var err error
		sourceImagePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(ioutil.WriteFile(path.Join(sourceImagePath, "foo"), []byte("hello-world"), 0644)).To(Succeed())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(sourceImagePath)).To(Succeed())
		Expect(os.RemoveAll(baseImagePath)).To(Succeed())
	})

	JustBeforeEach(func() {
		baseImageFile := integration.CreateBaseImageTar(sourceImagePath)
		baseImagePath = baseImageFile.Name()
		var err error
		image, err = Runner.Create(groot.CreateSpec{
			BaseImage: baseImagePath,
			ID:        "random-id",
			Mount:     true,
		})
		Expect(err).ToNot(HaveOccurred())
	})

	It("destroys the quota group associated with the volume", func() {
		rootIDBuffer := gbytes.NewBuffer()
		sess, err := gexec.Start(exec.Command("sudo", "btrfs", "inspect-internal", "rootid", image.RootFSPath), rootIDBuffer, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(sess).Should(gexec.Exit(0))
		rootID := strings.TrimSpace(string(rootIDBuffer.Contents()))

		sess, err = gexec.Start(exec.Command("sudo", "btrfs", "qgroup", "show", StorePath), GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(sess).Should(gexec.Exit(0))
		Expect(sess).To(gbytes.Say(rootID))

		Expect(Runner.Delete("random-id")).To(Succeed())

		sess, err = gexec.Start(exec.Command("sudo", "btrfs", "qgroup", "show", StorePath), GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(sess).Should(gexec.Exit(0))
		Expect(sess).ToNot(gbytes.Say(rootID))
	})

	Context("when the rootfs folder has subvolumes inside", func() {
		JustBeforeEach(func() {
			cmd := exec.Command("btrfs", "sub", "create", filepath.Join(image.RootFSPath, "subvolume"))
			cmd.SysProcAttr = &syscall.SysProcAttr{
				Credential: &syscall.Credential{
					Uid: uint32(GrootfsTestUid),
					Gid: uint32(GrootfsTestGid),
				},
			}
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(0))

			cmd = exec.Command("btrfs", "sub", "snapshot", filepath.Join(image.RootFSPath, "subvolume"), filepath.Join(image.RootFSPath, "snapshot"))
			cmd.SysProcAttr = &syscall.SysProcAttr{
				Credential: &syscall.Credential{
					Uid: uint32(GrootfsTestUid),
					Gid: uint32(GrootfsTestGid),
				},
			}
			sess, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(0))
		})

		It("removes the rootfs completely", func() {
			Expect(image.RootFSPath).To(BeAnExistingFile())
			Expect(Runner.Delete("random-id")).To(Succeed())
			Expect(image.RootFSPath).ToNot(BeAnExistingFile())
		})
	})

	Context("when drax is not in PATH", func() {
		It("returns a warning", func() {
			errBuffer := gbytes.NewBuffer()
			outBuffer := gbytes.NewBuffer()
			err := Runner.WithoutDraxBin().
				WithLogLevel(lager.INFO).
				WithEnvVar("PATH=/usr/sbin:/usr/bin:/sbin:/bin").
				WithStdout(outBuffer).
				WithStderr(errBuffer).
				Delete("random-id")
			Expect(err).NotTo(HaveOccurred())
			Eventually(errBuffer).Should(gbytes.Say("could not delete quota group"))
			Eventually(outBuffer).Should(gbytes.Say("Image random-id deleted"))
		})
	})
})
