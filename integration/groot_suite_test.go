package integration_test

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"os/user"
	"path"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/SUSE/groot-btrfs/integration"
	"github.com/SUSE/groot-btrfs/integration/runner"
	"github.com/SUSE/groot-btrfs/testhelpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

var (
	GrootFSBin    string
	Driver        string
	Runner        runner.Runner
	StorePath     string
	NamespacerBin string
	mountPath     string

	GrootUser        *user.User
	GrootUID         int
	GrootGID         int
	RegistryUsername string
	RegistryPassword string
	GrootfsTestUid   int
	GrootfsTestGid   int
)

const btrfsMountPath = "/mnt/btrfs-%d"

func TestGroot(t *testing.T) {
	var (
		DraxBin   string
		TardisBin string
	)

	RegisterFailHandler(Fail)

	SynchronizedBeforeSuite(func() []byte {
		grootFSBin, err := gexec.Build("github.com/SUSE/groot-btrfs")
		Expect(err).NotTo(HaveOccurred())
		grootFSBin = integration.MakeBinaryAccessibleToEveryone(grootFSBin)

		draxBin, err := gexec.Build("github.com/SUSE/groot-btrfs/store/filesystems/btrfs/drax")
		Expect(err).NotTo(HaveOccurred())
		draxBin = integration.MakeBinaryAccessibleToEveryone(draxBin)
		testhelpers.SuidBinary(draxBin)

		namespacerBin, err := gexec.Build("github.com/SUSE/groot-btrfs/integration/namespacer")
		Expect(err).NotTo(HaveOccurred())

		return []byte(grootFSBin + ":" + draxBin + ":" + ":" + namespacerBin)
	}, func(data []byte) {
		var err error
		binaries := strings.Split(string(data), ":")
		GrootFSBin = string(binaries[0])
		DraxBin = string(binaries[1])
		TardisBin = string(binaries[2])
		tmpNamespacerBin := string(binaries[3])

		GrootUser, err = user.Lookup("groot")
		Expect(err).NotTo(HaveOccurred())

		rand.Seed(time.Now().UnixNano())
		NamespacerBin = fmt.Sprintf("/tmp/namespacer-%d", rand.Int())
		cpNamespacerBin := exec.Command("cp", tmpNamespacerBin, NamespacerBin)
		sess, err := gexec.Start(cpNamespacerBin, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(sess).Should(gexec.Exit(0))

		GrootUID, err = strconv.Atoi(GrootUser.Uid)
		Expect(err).NotTo(HaveOccurred())

		GrootGID, err = strconv.Atoi(GrootUser.Gid)
		Expect(err).NotTo(HaveOccurred())

		Driver = os.Getenv("VOLUME_DRIVER")

		GrootfsTestUid, _ = strconv.Atoi(os.Getenv("GROOTFS_TEST_UID"))
		GrootfsTestGid, _ = strconv.Atoi(os.Getenv("GROOTFS_TEST_GID"))

		fmt.Fprintf(os.Stderr, "============> RUNNING %s TESTS (%d:%d) <=============", Driver, GrootfsTestUid, GrootfsTestGid)
	})

	SynchronizedAfterSuite(func() {
	}, func() {
		gexec.CleanupBuildArtifacts()
	})

	BeforeEach(func() {
		testhelpers.ReseedRandomNumberGenerator()

		Driver = "btrfs"
		mountPath = fmt.Sprintf(btrfsMountPath, GinkgoParallelNode())

		StorePath = path.Join(mountPath, "store")

		RegistryUsername = os.Getenv("DOCKER_REGISTRY_USERNAME")
		RegistryPassword = os.Getenv("DOCKER_REGISTRY_PASSWORD")

		Runner = runner.Runner{
			GrootFSBin: GrootFSBin,
			StorePath:  StorePath,
			DraxBin:    DraxBin,
		}.WithLogLevel(lager.DEBUG).WithStderr(GinkgoWriter).RunningAsUser(GrootfsTestUid, GrootfsTestGid)
	})

	AfterEach(func() {
		testhelpers.CleanUpBtrfsSubvolumes(mountPath)

		Expect(os.RemoveAll(StorePath)).To(Succeed())
	})

	RunSpecs(t, "Integration Suite")
}

func writeMegabytes(outputPath string, mb int) error {
	cmd := exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", outputPath), "bs=1048576", fmt.Sprintf("count=%d", mb))
	sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	if err != nil {
		return err
	}
	Eventually(sess, 10*time.Second).Should(gexec.Exit())
	if sess.ExitCode() > 0 {
		return errors.New(string(sess.Err.Contents()))
	}
	return nil
}
