package groot_test

import (
	"errors"
	"os"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/SUSE/groot-btrfs/groot"
	"github.com/SUSE/groot-btrfs/groot/grootfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Deleter", func() {
	var (
		fakeImageCloner       *grootfakes.FakeImageCloner
		fakeDependencyManager *grootfakes.FakeDependencyManager
		fakeMetricsEmitter    *grootfakes.FakeMetricsEmitter
		deleter               *groot.Deleter
		logger                lager.Logger
	)

	BeforeEach(func() {
		fakeImageCloner = new(grootfakes.FakeImageCloner)
		fakeDependencyManager = new(grootfakes.FakeDependencyManager)
		fakeMetricsEmitter = new(grootfakes.FakeMetricsEmitter)

		deleter = groot.IamDeleter(fakeImageCloner, fakeDependencyManager, fakeMetricsEmitter)
		logger = lagertest.NewTestLogger("deleter")
	})

	Describe("Delete", func() {
		It("destroys a image", func() {
			Expect(deleter.Delete(logger, "some-id")).To(Succeed())

			_, imageId := fakeImageCloner.DestroyArgsForCall(0)
			Expect(imageId).To(Equal("some-id"))
		})

		It("deregisters image dependencies", func() {
			Expect(deleter.Delete(logger, "some-id")).To(Succeed())
			Expect(fakeDependencyManager.DeregisterCallCount()).To(Equal(1))
		})

		Context("when destroying a image fails", func() {
			BeforeEach(func() {
				fakeImageCloner.DestroyReturns(errors.New("failed to destroy image"))
			})

			It("returns an error", func() {
				Expect(deleter.Delete(logger, "some-id")).To(MatchError(ContainSubstring("failed to destroy image")))
			})

			It("doesn't deregister the image", func() {
				Expect(deleter.Delete(logger, "some-id")).To(HaveOccurred())
				Expect(fakeDependencyManager.DeregisterCallCount()).To(Equal(0))
			})
		})

		It("emits metrics for deletion", func() {
			Expect(deleter.Delete(logger, "some-id")).To(Succeed())

			Expect(fakeMetricsEmitter.TryEmitDurationFromCallCount()).To(Equal(1))
			_, name, start := fakeMetricsEmitter.TryEmitDurationFromArgsForCall(0)
			Expect(name).To(Equal(groot.MetricImageDeletionTime))
			Expect(start).NotTo(BeZero())
		})

		Context("when it fails to deregister an image", func() {
			BeforeEach(func() {
				fakeDependencyManager.DeregisterReturns(errors.New("failed"))
			})

			It("returns an error", func() {
				Expect(deleter.Delete(logger, "some-id")).To(MatchError(ContainSubstring("failed")))
			})

			Context("when the image metadata doesn't exist", func() {
				BeforeEach(func() {
					fakeDependencyManager.DeregisterReturns(os.ErrNotExist)
				})

				It("doesn't return an error", func() {
					Expect(deleter.Delete(logger, "some-id")).To(Succeed())
				})
			})
		})
	})
})
