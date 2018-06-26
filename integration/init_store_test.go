package integration_test

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/SUSE/groot-btrfs/groot"
	"github.com/SUSE/groot-btrfs/integration"
	grootfsRunner "github.com/SUSE/groot-btrfs/integration/runner"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Init Store", func() {
	const (
		rootUID = 0
		rootGID = 0
	)

	var (
		runner    grootfsRunner.Runner
		spec      grootfsRunner.InitSpec
		storePath string
	)

	BeforeEach(func() {
		integration.SkipIfNonRoot(GrootfsTestUid)

		storePath = filepath.Join(StorePath, strconv.Itoa(rand.Int()))
		Expect(os.MkdirAll(storePath, 0777)).To(Succeed())
		runner = Runner.WithStore(storePath).SkipInitStore()
		spec = grootfsRunner.InitSpec{}
	})

	It("returns a newly created store path", func() {
		Expect(runner.InitStore(spec)).To(Succeed())

		Expect(runner.StorePath).To(BeADirectory())

		stat, err := os.Stat(runner.StorePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(stat.Mode().Perm()).To(Equal(os.FileMode(0700)))
	})

	It("sets the ownership to the caller user", func() {
		Expect(runner.InitStore(spec)).To(Succeed())

		Expect(runner.StorePath).To(BeADirectory())

		stat, err := os.Stat(runner.StorePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(stat.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(rootUID)))
		Expect(stat.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(rootGID)))
	})

	Context("when --store-size-bytes is passed", func() {
		var backingStoreFile string

		BeforeEach(func() {
			tmpDir, err := ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())

			storePath = filepath.Join(tmpDir, "store")
			spec.StoreSizeBytes = 500 * 1024 * 1024
			backingStoreFile = fmt.Sprintf("%s.backing-store", storePath)

			runner = runner.WithStore(storePath)
		})

		AfterEach(func() {
			_ = syscall.Unmount(storePath, 0)
			Expect(os.RemoveAll(backingStoreFile)).To(Succeed())
		})

		It("creates the backing file with the correct size", func() {
			err := runner.InitStore(spec)
			Expect(err).NotTo(HaveOccurred())

			stat, err := os.Stat(backingStoreFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(stat.Size()).To(Equal(int64(500 * 1024 * 1024)))
		})

		It("initialises a filesystem in the backing file", func() {
			Expect(runner.InitStore(spec)).To(Succeed())

			buffer := gbytes.NewBuffer()
			cmd := exec.Command("file", backingStoreFile)
			cmd.Stdout = buffer
			Expect(cmd.Run()).To(Succeed())

			fsType := map[string]string{"btrfs": "BTRFS"}
			Expect(buffer).To(gbytes.Say(fsType[Driver]))
		})

		Context("when the given store path is already initialized", func() {
			BeforeEach(func() {
				Expect(runner.InitStore(spec)).To(Succeed())
			})

			It("logs the event", func() {
				logs := gbytes.NewBuffer()
				Expect(runner.WithStderr(logs).InitStore(spec)).To(Succeed())

				Expect(logs).To(gbytes.Say("store-already-initialized"))
			})
		})

		Context("when the given backing store size is too small", func() {
			BeforeEach(func() {
				spec.StoreSizeBytes = 199 * 1024 * 1024
			})

			It("returns an error", func() {
				err := runner.InitStore(spec)
				Expect(err).To(MatchError(ContainSubstring("store size must be at least 200Mb")))
			})
		})
	})

	Context("when id mappings are provided", func() {
		BeforeEach(func() {
			spec.UIDMappings = []groot.IDMappingSpec{
				groot.IDMappingSpec{HostID: GrootUID, NamespaceID: 0, Size: 1},
				groot.IDMappingSpec{HostID: 100000, NamespaceID: 1, Size: 65000},
			}
			spec.GIDMappings = []groot.IDMappingSpec{
				groot.IDMappingSpec{HostID: GrootGID, NamespaceID: 0, Size: 1},
				groot.IDMappingSpec{HostID: 100000, NamespaceID: 1, Size: 65000},
			}
		})

		It("sets the ownership to the user mapped as root", func() {
			Expect(runner.InitStore(spec)).To(Succeed())

			Expect(runner.StorePath).To(BeADirectory())

			stat, err := os.Stat(runner.StorePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(stat.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(GrootUID)))
			Expect(stat.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(GrootGID)))
		})
	})

	Context("when the given driver does not match the mounted path", func() {
		BeforeEach(func() {
			runner = Runner.WithStore("/mnt/ext4/grootfs")
		})

		It("returns an error", func() {
			err := runner.InitStore(spec)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("Store path filesystem (/mnt/ext4) is incompatible with requested driver"))
			Expect(runner.StorePath).ToNot(BeADirectory())
		})
	})

	Context("when the given store path is already initialized", func() {
		BeforeEach(func() {
			Expect(runner.InitStore(spec)).To(Succeed())
		})

		It("does not return an error", func() {
			Expect(runner.InitStore(spec)).To(Succeed())
		})
	})

	Context("when the user is not root", func() {
		It("returns an error", func() {
			err := runner.RunningAsUser(GrootUID, GrootUID).InitStore(spec)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(fmt.Sprintf("store %s can only be initialized by Root user", runner.StorePath)))
		})
	})
})
