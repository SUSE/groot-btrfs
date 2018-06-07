package driver_test

import (
	"code.cloudfoundry.org/lager"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io/ioutil"
	"os"

	grootdriver "github.com/SUSE/groot-btrfs/driver"
)

var _ = Describe("Driver", func() {
	const (
		storeName = "test-store"
	)

	var (
		driver *grootdriver.Driver
	)

	btrfsMountPath := os.Getenv("BTRFS")
	_, err := os.Stat(btrfsMountPath)
	if os.IsNotExist(err) {
		panic("BTRFS environment variable does not match an existing directory")
	}

	BeforeEach(func() {
		driverConfig := &grootdriver.Config{
			VolumesDirName: "volumes",
			DraxBinPath:    "tmp/drax",
			StorePath:      storeName,
		}

		driver = grootdriver.NewDriver(driverConfig)
	})

	Describe("MoveVolume", func() {
		var tmpVolumeDir string
		var err error
		BeforeEach(func() {
			tmpVolumeDir, err = ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			err := os.RemoveAll(tmpVolumeDir)
			Expect(err).NotTo(HaveOccurred())
			err = os.RemoveAll(tmpVolumeDir + "-new")
			Expect(err).NotTo(HaveOccurred())
		})

		It("succcesfully moves the volume directory", func() {
			newDir := tmpVolumeDir + "-new"
			driver.MoveVolume(lager.NewLogger("groot"), tmpVolumeDir, newDir)
			_, err := os.Stat(newDir)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
