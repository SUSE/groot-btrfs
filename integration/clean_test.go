package integration_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/SUSE/groot-btrfs/groot"
	"github.com/SUSE/groot-btrfs/integration"
	"github.com/SUSE/groot-btrfs/store"
	"github.com/SUSE/groot-btrfs/testhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Clean", func() {
	Context("OCI Images", func() {
		var (
			baseImagePath        string
			anotherBaseImagePath string
		)

		BeforeEach(func() {
			workDir, err := os.Getwd()
			Expect(err).NotTo(HaveOccurred())
			baseImagePath = fmt.Sprintf("oci:///%s/assets/oci-test-image/grootfs-busybox:latest", workDir)
			anotherBaseImagePath = fmt.Sprintf("oci:///%s/assets/oci-test-image/4mb-image:latest", workDir)

			_, err = Runner.Create(groot.CreateSpec{
				ID:           "my-image-1",
				BaseImageURL: integration.String2URL(baseImagePath),
				Mount:        true,
			})
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			Expect(Runner.Delete("my-image-1")).To(Succeed())
		})

		Context("when the store doesn't exist", func() {
			It("logs an error message and exits successfully", func() {
				logBuffer := gbytes.NewBuffer()
				_, err := Runner.WithStore("/invalid-store").WithStderr(logBuffer).Clean(0)
				Expect(err).ToNot(HaveOccurred())
				Expect(logBuffer).To(gbytes.Say(`"error":"no store found at /invalid-store"`))
			})
		})

		Context("when there are unused volumes", func() {
			BeforeEach(func() {
				_, err := Runner.Create(groot.CreateSpec{
					ID:           "my-image-2",
					BaseImageURL: integration.String2URL(anotherBaseImagePath),
					Mount:        true,
					DiskLimit:    10 * 1024 * 1024,
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(Runner.Delete("my-image-2")).To(Succeed())
			})

			It("removes them", func() {
				preContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
				Expect(err).NotTo(HaveOccurred())
				Expect(preContents).To(HaveLen(8))

				_, err = Runner.Clean(0)
				Expect(err).NotTo(HaveOccurred())

				afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
				Expect(err).NotTo(HaveOccurred())
				Expect(afterContents).To(HaveLen(4))
			})

			Context("and a threshold is set", func() {
				var cleanupThresholdInBytes int64

				Context("but it lower than 0", func() {
					BeforeEach(func() {
						cleanupThresholdInBytes = -10
					})
					It("returns an error", func() {
						_, err := Runner.Clean(cleanupThresholdInBytes)
						Expect(err).To(MatchError("invalid argument: clean threshold cannot be negative"))
					})
				})

				Context("and the total is less than the threshold", func() {
					BeforeEach(func() {
						cleanupThresholdInBytes = 500000000
					})

					It("does not remove the unused volumes", func() {
						preContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
						Expect(err).NotTo(HaveOccurred())

						_, err = Runner.Clean(cleanupThresholdInBytes)
						Expect(err).NotTo(HaveOccurred())

						afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
						Expect(err).NotTo(HaveOccurred())
						Expect(afterContents).To(HaveLen(len(preContents)))
					})

					It("reports that it was a no-op", func() {
						output, err := Runner.Clean(cleanupThresholdInBytes)
						Expect(err).NotTo(HaveOccurred())
						Expect(output).To(ContainSubstring("threshold not reached: skipping clean"))
					})
				})

				Context("and the total is more than the threshold", func() {
					BeforeEach(func() {
						cleanupThresholdInBytes = 70000
					})

					It("removes the unused volumes", func() {
						preContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
						Expect(err).NotTo(HaveOccurred())
						Expect(preContents).To(HaveLen(8))

						_, err = Runner.Clean(cleanupThresholdInBytes)
						Expect(err).NotTo(HaveOccurred())

						afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
						Expect(err).NotTo(HaveOccurred())
						Expect(afterContents).To(HaveLen(4))
					})
				})
			})
		})
	})

	Context("Remote Images", func() {
		BeforeEach(func() {
			_, err := Runner.Create(groot.CreateSpec{
				ID:           "my-image-1",
				BaseImageURL: integration.String2URL("docker:///cfgarden/empty:v0.1.1"),
				Mount:        true,
			})
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			Expect(Runner.Delete("my-image-1")).To(Succeed())
		})

		Context("when there are unused layers", func() {
			BeforeEach(func() {
				_, err := Runner.Create(groot.CreateSpec{
					ID:           "my-image-2",
					BaseImageURL: integration.String2URL("docker:///cfgarden/garden-busybox"),
					Mount:        true,
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(Runner.Delete("my-image-2")).To(Succeed())
			})

			It("removes unused volumes", func() {
				preContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
				Expect(err).NotTo(HaveOccurred())
				Expect(preContents).To(HaveLen(3))

				_, err = Runner.Clean(0)
				Expect(err).NotTo(HaveOccurred())

				afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
				Expect(err).NotTo(HaveOccurred())
				Expect(afterContents).To(HaveLen(2))
				for _, layer := range testhelpers.EmptyBaseImageV011.Layers {
					Expect(filepath.Join(StorePath, store.VolumesDirName, layer.ChainID)).To(BeADirectory())
				}
			})
		})
	})
})
