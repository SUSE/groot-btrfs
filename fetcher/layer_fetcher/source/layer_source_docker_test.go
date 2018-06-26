package source_test

import (
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/SUSE/groot-btrfs/fetcher/layer_fetcher"
	"github.com/SUSE/groot-btrfs/fetcher/layer_fetcher/source"
	"github.com/SUSE/groot-btrfs/groot"
	"github.com/SUSE/groot-btrfs/integration"
	"github.com/SUSE/groot-btrfs/testhelpers"
	"github.com/containers/image/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Layer source: Docker", func() {
	var (
		layerSource source.LayerSource

		logger       *lagertest.TestLogger
		baseImageURL *url.URL

		configBlob    string
		layerInfos    []groot.LayerInfo
		systemContext types.SystemContext

		skipOCIChecksumValidation bool
	)

	BeforeEach(func() {
		systemContext = types.SystemContext{
			DockerAuthConfig: &types.DockerAuthConfig{
				Username: RegistryUsername,
				Password: RegistryPassword,
			},
		}

		skipOCIChecksumValidation = false

		configBlob = "sha256:217f3b4afdf698d639f854d9c6d640903a011413bc7e7bffeabe63c7ca7e4a7d"
		layerInfos = []groot.LayerInfo{
			{
				BlobID:    "sha256:47e3dd80d678c83c50cb133f4cf20e94d088f890679716c8b763418f55827a58",
				DiffID:    "afe200c63655576eaa5cabe036a2c09920d6aee67653ae75a9d35e0ec27205a5",
				MediaType: "application/vnd.oci.image.layer.v1.tar+gzip",
				Size:      90,
			},
			{
				BlobID:    "sha256:7f2760e7451ce455121932b178501d60e651f000c3ab3bc12ae5d1f57614cc76",
				DiffID:    "d7c6a5f0d9a15779521094fa5eaf026b719984fb4bfe8e0012bd1da1b62615b0",
				MediaType: "application/vnd.oci.image.layer.v1.tar+gzip",
				Size:      88,
			},
		}

		logger = lagertest.NewTestLogger("test-layer-source")
		var err error
		baseImageURL, err = url.Parse("docker:///cfgarden/empty:v0.1.1")
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		layerSource = source.NewLayerSource(systemContext, skipOCIChecksumValidation)
	})

	Describe("Manifest", func() {
		It("fetches the manifest", func() {
			manifest, err := layerSource.Manifest(logger, baseImageURL)
			Expect(err).NotTo(HaveOccurred())

			Expect(manifest.ConfigInfo().Digest.String()).To(Equal(configBlob))

			Expect(manifest.LayerInfos()).To(HaveLen(2))
			Expect(manifest.LayerInfos()[0].Digest.String()).To(Equal(layerInfos[0].BlobID))
			Expect(manifest.LayerInfos()[0].Size).To(Equal(layerInfos[0].Size))
			Expect(manifest.LayerInfos()[1].Digest.String()).To(Equal(layerInfos[1].BlobID))
			Expect(manifest.LayerInfos()[1].Size).To(Equal(layerInfos[1].Size))
		})

		Context("when the image schema version is 1", func() {
			BeforeEach(func() {
				var err error
				baseImageURL, err = url.Parse("docker://cfgarden/empty:schemaV1")
				Expect(err).NotTo(HaveOccurred())
			})

			It("fetches the manifest", func() {
				manifest, err := layerSource.Manifest(logger, baseImageURL)
				Expect(err).NotTo(HaveOccurred())

				Expect(manifest.ConfigInfo().Digest.String()).To(Equal(testhelpers.SchemaV1EmptyBaseImage.ConfigBlobID))

				Expect(manifest.LayerInfos()).To(HaveLen(3))
				Expect(manifest.LayerInfos()[0].Digest.String()).To(Equal(testhelpers.SchemaV1EmptyBaseImage.Layers[0].BlobID))
				Expect(manifest.LayerInfos()[1].Digest.String()).To(Equal(testhelpers.SchemaV1EmptyBaseImage.Layers[1].BlobID))
				Expect(manifest.LayerInfos()[2].Digest.String()).To(Equal(testhelpers.SchemaV1EmptyBaseImage.Layers[2].BlobID))
			})
		})

		Context("when the image is private", func() {
			BeforeEach(func() {
				var err error
				baseImageURL, err = url.Parse("docker:///viovanov/test")
				Expect(err).NotTo(HaveOccurred())

				configBlob = "sha256:d167ceb40f570b1463fa685d5c85ab8389bfc1f8c8ae6398721a826eba0d1117"
				layerInfos[0].BlobID = "sha256:ff646178418ec68dc4a2b1063ee0fac247f39d6e9f1a67578e0e25df3fc5a69e"
				layerInfos[0].DiffID = "afe200c63655576eaa5cabe036a2c09920d6aee67653ae75a9d35e0ec27205a5"
				layerInfos[1].BlobID = "sha256:5b9380a86827051584700d2cdb646eabe19fd6bca05b3c331e9cf88af575a43f"
			})

			Context("when the correct credentials are provided", func() {
				It("fetches the manifest", func() {
					manifest, err := layerSource.Manifest(logger, baseImageURL)
					Expect(err).NotTo(HaveOccurred())

					Expect(manifest.ConfigInfo().Digest.String()).To(Equal(configBlob))

					Expect(manifest.LayerInfos()).To(HaveLen(2))
					Expect(manifest.LayerInfos()[0].Digest.String()).To(Equal(layerInfos[0].BlobID))
					Expect(manifest.LayerInfos()[0].Size).To(Equal(layerInfos[0].Size))
					Expect(manifest.LayerInfos()[1].Digest.String()).To(Equal(layerInfos[1].BlobID))
					Expect(manifest.LayerInfos()[1].Size).To(Equal(layerInfos[1].Size))
				})
			})

			Context("when the registry returns a 401 when trying to get the auth token", func() {
				// We need a fake registry here because Dockerhub was rate limiting on multiple bad credential auth attempts
				var fakeRegistry *testhelpers.FakeRegistry

				BeforeEach(func() {
					dockerHubUrl, err := url.Parse("https://registry-1.docker.io")
					Expect(err).NotTo(HaveOccurred())
					fakeRegistry = testhelpers.NewFakeRegistry(dockerHubUrl)
					fakeRegistry.Start()
					fakeRegistry.ForceTokenAuthError()
					baseImageURL = integration.String2URL(fmt.Sprintf("docker://%s/doesnt-matter-because-fake-registry", fakeRegistry.Addr()))

					systemContext.DockerInsecureSkipTLSVerify = true
				})

				AfterEach(func() {
					fakeRegistry.Stop()
				})

				It("returns an informative error", func() {
					_, err := layerSource.Manifest(logger, baseImageURL)
					Expect(err).To(MatchError(ContainSubstring("unable to retrieve auth token")))
				})
			})
		})

		Context("when the image url is invalid", func() {
			It("returns an error", func() {
				baseImageURL, err := url.Parse("docker:cfgarden/empty:v0.1.0")
				Expect(err).NotTo(HaveOccurred())

				_, err = layerSource.Manifest(logger, baseImageURL)
				Expect(err).To(MatchError(ContainSubstring("parsing url failed")))
			})
		})

		Context("when the image does not exist", func() {
			BeforeEach(func() {
				var err error
				baseImageURL, err = url.Parse("docker:///cfgarden/non-existing-image")
				Expect(err).NotTo(HaveOccurred())

				systemContext.DockerAuthConfig.Username = ""
				systemContext.DockerAuthConfig.Password = ""
			})

			It("wraps the containers/image with a useful error", func() {
				_, err := layerSource.Manifest(logger, baseImageURL)
				Expect(err.Error()).To(MatchRegexp("^fetching image reference"))
			})

			It("logs the original error message", func() {
				_, err := layerSource.Manifest(logger, baseImageURL)
				Expect(err).To(HaveOccurred())

				Expect(logger).To(gbytes.Say("fetching-image-reference-failed"))
				Expect(logger).To(gbytes.Say("unauthorized: authentication required"))
			})
		})
	})

	Describe("Config", func() {
		It("fetches the config", func() {
			manifest, err := layerSource.Manifest(logger, baseImageURL)
			Expect(err).NotTo(HaveOccurred())
			config, err := manifest.OCIConfig(context.TODO())
			Expect(err).NotTo(HaveOccurred())

			Expect(config.RootFS.DiffIDs).To(HaveLen(2))
			Expect(config.RootFS.DiffIDs[0].Hex()).To(Equal(layerInfos[0].DiffID))
			Expect(config.RootFS.DiffIDs[1].Hex()).To(Equal(layerInfos[1].DiffID))
		})

		Context("when the image is private", func() {
			var manifest layer_fetcher.Manifest

			BeforeEach(func() {
				var err error
				baseImageURL, err = url.Parse("docker:///viovanov/test")
				Expect(err).NotTo(HaveOccurred())
			})

			JustBeforeEach(func() {
				layerSource = source.NewLayerSource(systemContext, skipOCIChecksumValidation)
				var err error
				manifest, err = layerSource.Manifest(logger, baseImageURL)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when the correct credentials are provided", func() {
				It("fetches the config", func() {
					config, err := manifest.OCIConfig(context.TODO())
					Expect(err).NotTo(HaveOccurred())

					Expect(config.RootFS.DiffIDs).To(HaveLen(2))
					Expect(config.RootFS.DiffIDs[0].Hex()).To(Equal("34266fca74f9c9ec860d83d880c095b455c140fd89b5c787bb7ae2865a7d12a7"))
					Expect(config.RootFS.DiffIDs[1].Hex()).To(Equal("0fbd3797562c7254af54436193f37a5b89c4fb8c7c18fad5b88b6f86d4664439"))
				})
			})
		})

		Context("when the image url is invalid", func() {
			It("returns an error", func() {
				baseImageURL, err := url.Parse("docker:cfgarden/empty:v0.1.0")
				Expect(err).NotTo(HaveOccurred())

				_, err = layerSource.Manifest(logger, baseImageURL)
				Expect(err).To(MatchError(ContainSubstring("parsing url failed")))
			})
		})

		Context("when the image schema version is 1", func() {
			BeforeEach(func() {
				var err error
				baseImageURL, err = url.Parse("docker://cfgarden/empty:schemaV1")
				Expect(err).NotTo(HaveOccurred())
			})

			It("fetches the config", func() {
				manifest, err := layerSource.Manifest(logger, baseImageURL)
				Expect(err).NotTo(HaveOccurred())
				config, err := manifest.OCIConfig(context.TODO())
				Expect(err).NotTo(HaveOccurred())

				Expect(config.RootFS.DiffIDs).To(HaveLen(3))
				Expect(config.RootFS.DiffIDs[0].String()).To(Equal(testhelpers.SchemaV1EmptyBaseImage.Layers[0].DiffID))
				Expect(config.RootFS.DiffIDs[1].String()).To(Equal(testhelpers.SchemaV1EmptyBaseImage.Layers[1].DiffID))
				Expect(config.RootFS.DiffIDs[2].String()).To(Equal(testhelpers.SchemaV1EmptyBaseImage.Layers[2].DiffID))
			})
		})
	})

	Context("when registry communication fails temporarily", func() {
		var fakeRegistry *testhelpers.FakeRegistry

		BeforeEach(func() {
			dockerHubUrl, err := url.Parse("https://registry-1.docker.io")
			Expect(err).NotTo(HaveOccurred())
			fakeRegistry = testhelpers.NewFakeRegistry(dockerHubUrl)
			fakeRegistry.Start()

			systemContext.DockerInsecureSkipTLSVerify = true
			baseImageURL, err = url.Parse(fmt.Sprintf("docker://%s/cfgarden/empty:v0.1.1", fakeRegistry.Addr()))
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			fakeRegistry.Stop()
		})

		It("retries fetching the manifest twice", func() {
			fakeRegistry.FailNextRequests(2)

			_, err := layerSource.Manifest(logger, baseImageURL)
			Expect(err).NotTo(HaveOccurred())

			Expect(logger.TestSink.LogMessages()).To(ContainElement("test-layer-source.fetching-image-manifest.attempt-get-image-1"))
			Expect(logger.TestSink.LogMessages()).To(ContainElement("test-layer-source.fetching-image-manifest.attempt-get-image-2"))
			Expect(logger.TestSink.LogMessages()).To(ContainElement("test-layer-source.fetching-image-manifest.attempt-get-image-3"))
			Expect(logger.TestSink.LogMessages()).To(ContainElement("test-layer-source.fetching-image-manifest.attempt-get-image-success"))
		})

		It("retries fetching a blob twice", func() {
			fakeRegistry.FailNextRequests(2)

			_, _, err := layerSource.Blob(logger, baseImageURL, layerInfos[0])
			Expect(err).NotTo(HaveOccurred())

			Expect(logger.TestSink.LogMessages()).To(
				ContainElement("test-layer-source.streaming-blob.attempt-get-blob-failed"))
		})

		It("retries fetching the config blob twice", func() {
			fakeRegistry.WhenGettingBlob(configBlob, 1, func(resp http.ResponseWriter, req *http.Request) {
				resp.WriteHeader(http.StatusTeapot)
				_, _ = resp.Write([]byte("null"))
				return
			})

			_, err := layerSource.Manifest(logger, baseImageURL)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeRegistry.RequestedBlobs()).To(Equal([]string{configBlob}), "config blob was not prefetched within the retry")

			Expect(logger.TestSink.LogMessages()).To(
				ContainElement("test-layer-source.fetching-image-manifest.fetching-image-config-failed"))
		})
	})

	Context("when a private registry is used", func() {
		var fakeRegistry *testhelpers.FakeRegistry

		BeforeEach(func() {
			dockerHubUrl, err := url.Parse("https://registry-1.docker.io")
			Expect(err).NotTo(HaveOccurred())
			fakeRegistry = testhelpers.NewFakeRegistry(dockerHubUrl)
			fakeRegistry.Start()

			baseImageURL, err = url.Parse(fmt.Sprintf("docker://%s/cfgarden/empty:v0.1.1", fakeRegistry.Addr()))
			Expect(err).NotTo(HaveOccurred())

		})

		AfterEach(func() {
			fakeRegistry.Stop()
		})

		It("fails to fetch the manifest", func() {
			_, err := layerSource.Manifest(logger, baseImageURL)
			Expect(err).To(HaveOccurred())
		})

		Context("when the private registry is whitelisted", func() {
			BeforeEach(func() {
				systemContext.DockerInsecureSkipTLSVerify = true
			})

			It("fetches the manifest", func() {
				manifest, err := layerSource.Manifest(logger, baseImageURL)
				Expect(err).NotTo(HaveOccurred())

				Expect(manifest.LayerInfos()).To(HaveLen(2))
				Expect(manifest.LayerInfos()[0].Digest.String()).To(Equal(layerInfos[0].BlobID))
				Expect(manifest.LayerInfos()[0].Size).To(Equal(layerInfos[0].Size))
				Expect(manifest.LayerInfos()[1].Digest.String()).To(Equal(layerInfos[1].BlobID))
				Expect(manifest.LayerInfos()[1].Size).To(Equal(layerInfos[1].Size))
			})

			It("fetches the config", func() {
				manifest, err := layerSource.Manifest(logger, baseImageURL)
				Expect(err).NotTo(HaveOccurred())

				config, err := manifest.OCIConfig(context.TODO())
				Expect(err).NotTo(HaveOccurred())

				Expect(config.RootFS.DiffIDs).To(HaveLen(2))
				Expect(config.RootFS.DiffIDs[0].Hex()).To(Equal(layerInfos[0].DiffID))
				Expect(config.RootFS.DiffIDs[1].Hex()).To(Equal(layerInfos[1].DiffID))
			})

			It("downloads and uncompresses the blob", func() {
				blobPath, size, err := layerSource.Blob(logger, baseImageURL, layerInfos[0])
				Expect(err).NotTo(HaveOccurred())

				blobReader, err := os.Open(blobPath)
				Expect(err).NotTo(HaveOccurred())

				buffer := gbytes.NewBuffer()
				cmd := exec.Command("tar", "tv")
				cmd.Stdin = blobReader
				sess, err := gexec.Start(cmd, buffer, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Expect(size).To(Equal(int64(90)))

				Eventually(buffer, "2s").Should(gbytes.Say("hello"))
				Eventually(sess).Should(gexec.Exit(0))
			})
		})

		Context("when using private images", func() {
			BeforeEach(func() {
				var err error
				baseImageURL, err = url.Parse("docker:///viovanov/test")
				Expect(err).NotTo(HaveOccurred())

				layerInfos[0].BlobID = "sha256:ff646178418ec68dc4a2b1063ee0fac247f39d6e9f1a67578e0e25df3fc5a69e"
				layerInfos[0].DiffID = "34266fca74f9c9ec860d83d880c095b455c140fd89b5c787bb7ae2865a7d12a7"
				layerInfos[1].BlobID = "sha256:5b9380a86827051584700d2cdb646eabe19fd6bca05b3c331e9cf88af575a43f"
				layerInfos[1].DiffID = "0fbd3797562c7254af54436193f37a5b89c4fb8c7c18fad5b88b6f86d4664439"
			})

			JustBeforeEach(func() {
				layerSource = source.NewLayerSource(systemContext, skipOCIChecksumValidation)
			})

			It("fetches the manifest", func() {
				manifest, err := layerSource.Manifest(logger, baseImageURL)
				Expect(err).NotTo(HaveOccurred())

				Expect(manifest.LayerInfos()).To(HaveLen(2))
				Expect(manifest.LayerInfos()[0].Digest.String()).To(Equal(layerInfos[0].BlobID))
				Expect(manifest.LayerInfos()[0].Size).To(Equal(layerInfos[0].Size))
				Expect(manifest.LayerInfos()[1].Digest.String()).To(Equal(layerInfos[1].BlobID))
				Expect(manifest.LayerInfos()[1].Size).To(Equal(layerInfos[1].Size))
			})

			It("fetches the config", func() {
				manifest, err := layerSource.Manifest(logger, baseImageURL)
				Expect(err).NotTo(HaveOccurred())

				config, err := manifest.OCIConfig(context.TODO())
				Expect(err).NotTo(HaveOccurred())

				Expect(config.RootFS.DiffIDs).To(HaveLen(2))
				Expect(config.RootFS.DiffIDs[0].Hex()).To(Equal(layerInfos[0].DiffID))
				Expect(config.RootFS.DiffIDs[1].Hex()).To(Equal(layerInfos[1].DiffID))
			})

			It("downloads and uncompresses the blob", func() {
				blobPath, size, err := layerSource.Blob(logger, baseImageURL, layerInfos[0])
				Expect(err).NotTo(HaveOccurred())

				blobReader, err := os.Open(blobPath)
				Expect(err).NotTo(HaveOccurred())

				buffer := gbytes.NewBuffer()
				cmd := exec.Command("tar", "tv")
				cmd.Stdin = blobReader
				sess, err := gexec.Start(cmd, buffer, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Expect(size).To(Equal(int64(90)))

				Eventually(buffer).Should(gbytes.Say("hello"))
				Eventually(sess).Should(gexec.Exit(0))
			})
		})
	})

	Describe("Blob", func() {
		It("downloads and uncompresses the blob", func() {
			blobPath, size, err := layerSource.Blob(logger, baseImageURL, layerInfos[0])
			Expect(err).NotTo(HaveOccurred())

			blobReader, err := os.Open(blobPath)
			Expect(err).NotTo(HaveOccurred())

			buffer := gbytes.NewBuffer()
			cmd := exec.Command("tar", "tv")
			cmd.Stdin = blobReader
			sess, err := gexec.Start(cmd, buffer, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Expect(size).To(Equal(int64(90)))

			Eventually(buffer).Should(gbytes.Say("hello"))
			Eventually(sess).Should(gexec.Exit(0))
		})

		Context("when the media type doesn't match the blob", func() {
			var fakeRegistry *testhelpers.FakeRegistry

			BeforeEach(func() {
				dockerHubUrl, err := url.Parse("https://registry-1.docker.io")
				Expect(err).NotTo(HaveOccurred())
				fakeRegistry = testhelpers.NewFakeRegistry(dockerHubUrl)

				fakeRegistry.WhenGettingBlob(layerInfos[0].BlobID, 1, func(rw http.ResponseWriter, req *http.Request) {
					_, _ = rw.Write([]byte("bad-blob"))
				})

				fakeRegistry.Start()

				baseImageURL, err = url.Parse(fmt.Sprintf("docker://%s/cfgarden/empty:v0.1.1", fakeRegistry.Addr()))
				Expect(err).NotTo(HaveOccurred())

				systemContext.DockerInsecureSkipTLSVerify = true
			})

			AfterEach(func() {
				fakeRegistry.Stop()
			})

			It("returns an error", func() {
				layerInfos[0].MediaType = "gzip"
				_, _, err := layerSource.Blob(logger, baseImageURL, layerInfos[0])
				Expect(err).To(MatchError(ContainSubstring("expected blob to be of type")))
			})
		})

		Context("when the image is private", func() {
			BeforeEach(func() {
				var err error
				baseImageURL, err = url.Parse("docker:///viovanov/test")
				Expect(err).NotTo(HaveOccurred())

				layerInfos = []groot.LayerInfo{
					{
						BlobID:    "sha256:ff646178418ec68dc4a2b1063ee0fac247f39d6e9f1a67578e0e25df3fc5a69e",
						DiffID:    "34266fca74f9c9ec860d83d880c095b455c140fd89b5c787bb7ae2865a7d12a7",
						MediaType: "application/vnd.oci.image.layer.v1.tar+gzip",
					},
				}
			})

			Context("when the correct credentials are provided", func() {
				It("fetches the config", func() {
					blobPath, size, err := layerSource.Blob(logger, baseImageURL, layerInfos[0])
					Expect(err).NotTo(HaveOccurred())

					blobReader, err := os.Open(blobPath)
					Expect(err).NotTo(HaveOccurred())

					buffer := gbytes.NewBuffer()
					cmd := exec.Command("tar", "tv")
					cmd.Stdin = blobReader
					sess, err := gexec.Start(cmd, buffer, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())
					Expect(size).To(Equal(int64(90)))

					Eventually(buffer, 5*time.Second).Should(gbytes.Say("hello"))
					Eventually(sess).Should(gexec.Exit(0))
				})
			})

			Context("when invalid credentials are provided", func() {
				// We need a fake registry here because Dockerhub was rate limiting on multiple bad credential auth attempts
				var fakeRegistry *testhelpers.FakeRegistry

				BeforeEach(func() {
					dockerHubUrl, err := url.Parse("https://registry-1.docker.io")
					Expect(err).NotTo(HaveOccurred())
					fakeRegistry = testhelpers.NewFakeRegistry(dockerHubUrl)
					fakeRegistry.Start()
					fakeRegistry.ForceTokenAuthError()
					baseImageURL = integration.String2URL(fmt.Sprintf("docker://%s/doesnt-matter-because-fake-registry", fakeRegistry.Addr()))

					systemContext.DockerInsecureSkipTLSVerify = true
				})

				AfterEach(func() {
					fakeRegistry.Stop()
				})

				It("retuns an error", func() {
					_, _, err := layerSource.Blob(logger, baseImageURL, layerInfos[0])
					Expect(err).To(MatchError(ContainSubstring("unable to retrieve auth token")))
				})
			})
		})

		Context("when the image url is invalid", func() {
			It("returns an error", func() {
				baseImageURL, err := url.Parse("docker:cfgarden/empty:v0.1.0")
				Expect(err).NotTo(HaveOccurred())

				_, _, err = layerSource.Blob(logger, baseImageURL, layerInfos[0])
				Expect(err).To(MatchError(ContainSubstring("parsing url failed")))
			})
		})

		Context("when the blob does not exist", func() {
			It("returns an error", func() {
				_, _, err := layerSource.Blob(logger, baseImageURL, groot.LayerInfo{BlobID: "sha256:steamed-blob"})
				Expect(err.Error()).To(ContainSubstring("fetching blob 400"))
			})
		})

		Context("when the blob is corrupted", func() {
			var fakeRegistry *testhelpers.FakeRegistry

			BeforeEach(func() {
				dockerHubUrl, err := url.Parse("https://registry-1.docker.io")
				Expect(err).NotTo(HaveOccurred())
				fakeRegistry = testhelpers.NewFakeRegistry(dockerHubUrl)
				fakeRegistry.WhenGettingBlob(layerInfos[1].BlobID, 1, func(rw http.ResponseWriter, req *http.Request) {
					gzipWriter := gzip.NewWriter(rw)
					_, _ = gzipWriter.Write([]byte("bad-blob"))
					gzipWriter.Close()
				})
				fakeRegistry.Start()

				baseImageURL, err = url.Parse(fmt.Sprintf("docker://%s/cfgarden/empty:v0.1.1", fakeRegistry.Addr()))
				Expect(err).NotTo(HaveOccurred())

				systemContext.DockerInsecureSkipTLSVerify = true
			})

			AfterEach(func() {
				fakeRegistry.Stop()
			})

			It("returns an error", func() {
				_, _, err := layerSource.Blob(logger, baseImageURL, layerInfos[1])
				Expect(err).To(MatchError(ContainSubstring("layerID digest mismatch")))
			})

			Context("when a devious hacker tries to set skipOCIChecksumValidation to true", func() {
				BeforeEach(func() {
					skipOCIChecksumValidation = true
				})

				It("returns an error", func() {
					_, _, err := layerSource.Blob(logger, baseImageURL, layerInfos[1])
					Expect(err).To(MatchError(ContainSubstring("layerID digest mismatch")))
				})
			})
		})

		Context("when the blob doesn't match the diffID", func() {
			BeforeEach(func() {
				layerInfos[1].DiffID = "0000000000000000000000000000000000000000000000000000000000000000"
			})

			It("returns an error", func() {
				_, _, err := layerSource.Blob(logger, baseImageURL, layerInfos[1])
				Expect(err).To(MatchError(ContainSubstring("diffID digest mismatch")))
			})
		})
	})
})
