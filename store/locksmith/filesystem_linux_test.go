package locksmith_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/SUSE/groot-btrfs/groot/grootfakes"
	"github.com/SUSE/groot-btrfs/store"
	"github.com/SUSE/groot-btrfs/store/locksmith"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Filesystem", func() {
	var (
		metricsEmitter *grootfakes.FakeMetricsEmitter
		storePath      string
	)

	BeforeEach(func() {
		var err error
		storePath, err = ioutil.TempDir("", "store")
		Expect(err).ToNot(HaveOccurred())
		Expect(os.Mkdir(filepath.Join(storePath, "locks"), 0755)).To(Succeed())
		metricsEmitter = new(grootfakes.FakeMetricsEmitter)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(storePath)).To(Succeed())
		locksmith.FlockSyscall = syscall.Flock
	})

	Context("ExclusiveLocksmith", func() {
		var exclusiveLocksmith *locksmith.FileSystem

		JustBeforeEach(func() {
			exclusiveLocksmith = locksmith.NewExclusiveFileSystem(storePath, metricsEmitter)
		})

		It("blocks when locking the same key twice", func() {
			lockFd, err := exclusiveLocksmith.Lock("key")
			Expect(err).NotTo(HaveOccurred())

			wentThrough := make(chan struct{})
			go func() {
				defer GinkgoRecover()

				_, err := exclusiveLocksmith.Lock("key")
				Expect(err).NotTo(HaveOccurred())

				close(wentThrough)
			}()

			Consistently(wentThrough).ShouldNot(BeClosed())
			Expect(exclusiveLocksmith.Unlock(lockFd)).To(Succeed())
			Eventually(wentThrough).Should(BeClosed())
		})

		Describe("Lock", func() {
			It("creates the lock file in the lock path when it does not exist", func() {
				lockFile := filepath.Join(storePath, store.LocksDirName, "key.lock")

				Expect(lockFile).ToNot(BeAnExistingFile())
				_, err := exclusiveLocksmith.Lock("key")
				Expect(err).NotTo(HaveOccurred())
				Expect(lockFile).To(BeAnExistingFile())
			})

			It("removes slashes(/) from key name", func() {
				lockFile := filepath.Join(storePath, store.LocksDirName, "/tmpkey.lock")

				Expect(lockFile).ToNot(BeAnExistingFile())
				_, err := exclusiveLocksmith.Lock("/tmp/key")
				Expect(err).NotTo(HaveOccurred())
				Expect(lockFile).To(BeAnExistingFile())
			})

			It("emits the locking time metric", func() {
				startTime := time.Now()
				_ = filepath.Join(storePath, store.LocksDirName, "key.lock")
				_, err := exclusiveLocksmith.Lock("key")
				Expect(err).NotTo(HaveOccurred())

				Expect(metricsEmitter.TryEmitDurationFromCallCount()).To(Equal(1))
				_, metricName, fromTime := metricsEmitter.TryEmitDurationFromArgsForCall(0)

				Expect(metricName).To(Equal(locksmith.ExclusiveMetricsLockingTime))
				Expect(fromTime.Unix()).To(BeNumerically("~", startTime.Unix(), 1))
			})

			Context("when creating the lock file fails", func() {
				BeforeEach(func() {
					storePath = "/not/real"
					exclusiveLocksmith = locksmith.NewExclusiveFileSystem(storePath, metricsEmitter)
				})

				It("returns an error", func() {
					lockFile := filepath.Join(storePath, "key")

					_, err := exclusiveLocksmith.Lock("key")
					Expect(err).To(MatchError(ContainSubstring("creating lock file for key `key`:")))
					Expect(lockFile).ToNot(BeAnExistingFile())
				})
			})

			Context("when locking the file fails", func() {
				BeforeEach(func() {
					locksmith.FlockSyscall = func(_ int, _ int) error {
						return errors.New("failed to lock file")
					}
				})

				It("returns an error", func() {
					_, err := exclusiveLocksmith.Lock("key")
					Expect(err).To(MatchError(ContainSubstring("failed to lock file")))
				})
			})
		})

		Context("Unlock", func() {
			Context("when unlocking a file descriptor fails", func() {
				var lockFile *os.File

				BeforeEach(func() {
					lockFile = os.NewFile(uintptr(12), "lockFile")
					locksmith.FlockSyscall = func(_ int, _ int) error {
						return errors.New("failed to unlock file")
					}
				})

				It("returns an error", func() {
					Expect(exclusiveLocksmith.Unlock(lockFile)).To(
						MatchError(ContainSubstring("failed to unlock file")),
					)
				})
			})
		})
	})

	Context("SharedLocksmith", func() {
		var sharedLocksmith *locksmith.FileSystem
		JustBeforeEach(func() {
			sharedLocksmith = locksmith.NewSharedFileSystem(storePath, metricsEmitter)
		})

		It("blocks when locking the same key twice", func() {
			lockFd, err := sharedLocksmith.Lock("key")
			Expect(err).NotTo(HaveOccurred())

			wentThrough := make(chan struct{})
			go func() {
				defer GinkgoRecover()

				_, err := sharedLocksmith.Lock("key")
				Expect(err).NotTo(HaveOccurred())

				close(wentThrough)
			}()

			Eventually(wentThrough).Should(BeClosed())
			Expect(sharedLocksmith.Unlock(lockFd)).To(Succeed())
		})

		Describe("Lock", func() {
			It("creates the lock file in the lock path when it does not exist", func() {
				lockFile := filepath.Join(storePath, store.LocksDirName, "key.lock")

				Expect(lockFile).ToNot(BeAnExistingFile())

				_, err := sharedLocksmith.Lock("key")
				Expect(err).NotTo(HaveOccurred())
				Expect(lockFile).To(BeAnExistingFile())
			})

			It("removes slashes(/) from key name", func() {
				lockFile := filepath.Join(storePath, store.LocksDirName, "/tmpkey.lock")

				Expect(lockFile).ToNot(BeAnExistingFile())
				_, err := sharedLocksmith.Lock("/tmp/key")
				Expect(err).NotTo(HaveOccurred())
				Expect(lockFile).To(BeAnExistingFile())
			})

			It("emits the locking time metric", func() {
				startTime := time.Now()
				_ = filepath.Join(storePath, store.LocksDirName, "key.lock")
				_, err := sharedLocksmith.Lock("key")
				Expect(err).NotTo(HaveOccurred())

				Expect(metricsEmitter.TryEmitDurationFromCallCount()).To(Equal(1))
				_, metricName, fromTime := metricsEmitter.TryEmitDurationFromArgsForCall(0)

				Expect(metricName).To(Equal(locksmith.SharedMetricsLockingTime))
				Expect(fromTime.Unix()).To(BeNumerically("~", startTime.Unix(), 1))
			})

			Context("when creating the lock file fails", func() {
				BeforeEach(func() {
					storePath = "/not/real"
					sharedLocksmith = locksmith.NewSharedFileSystem(storePath, metricsEmitter)
				})

				It("returns an error", func() {
					lockFile := filepath.Join(storePath, "key")

					_, err := sharedLocksmith.Lock("key")
					Expect(err).To(MatchError(ContainSubstring("creating lock file for key `key`:")))
					Expect(lockFile).ToNot(BeAnExistingFile())
				})
			})

			Context("when locking the file fails", func() {
				BeforeEach(func() {
					locksmith.FlockSyscall = func(_ int, _ int) error {
						return errors.New("failed to lock file")
					}
				})

				It("returns an error", func() {
					_, err := sharedLocksmith.Lock("key")
					Expect(err).To(MatchError(ContainSubstring("failed to lock file")))
				})
			})
		})

		Context("Unlock", func() {
			Context("when unlocking a file descriptor fails", func() {
				var lockFile *os.File

				BeforeEach(func() {
					lockFile = os.NewFile(uintptr(12), "lockFile")
					locksmith.FlockSyscall = func(_ int, _ int) error {
						return errors.New("failed to unlock file")
					}
				})

				It("returns an error", func() {
					Expect(sharedLocksmith.Unlock(lockFile)).To(
						MatchError(ContainSubstring("failed to unlock file")),
					)
				})
			})
		})
	})
})
