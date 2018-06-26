package groot_test

import (
	"errors"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/SUSE/groot-btrfs/groot"
	"github.com/SUSE/groot-btrfs/groot/grootfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Statser", func() {
	var (
		fakeImageCloner    *grootfakes.FakeImageCloner
		fakeMetricsEmitter *grootfakes.FakeMetricsEmitter
		statser            *groot.Statser
		logger             lager.Logger
	)

	BeforeEach(func() {
		fakeImageCloner = new(grootfakes.FakeImageCloner)
		fakeMetricsEmitter = new(grootfakes.FakeMetricsEmitter)
		statser = groot.IamStatser(fakeImageCloner, fakeMetricsEmitter)
		logger = lagertest.NewTestLogger("statser")
	})

	Describe("Stats", func() {
		It("asks for stats from the imageCloner", func() {
			fakeImageCloner.StatsReturns(groot.VolumeStats{}, nil)
			_, err := statser.Stats(logger, "some-id")
			Expect(err).ToNot(HaveOccurred())

			Expect(fakeImageCloner.StatsCallCount()).To(Equal(1))
			_, id := fakeImageCloner.StatsArgsForCall(0)
			Expect(id).To(Equal("some-id"))
		})

		It("asks for stats from the imageCloner", func() {
			stats := groot.VolumeStats{
				DiskUsage: groot.DiskUsage{
					TotalBytesUsed:     1024,
					ExclusiveBytesUsed: 512,
				},
			}
			fakeImageCloner.StatsReturns(stats, nil)

			returnedStats, err := statser.Stats(logger, "some-id")
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedStats).To(Equal(stats))
		})

		It("emits metrics for stats", func() {
			_, err := statser.Stats(logger, "some-id")
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeMetricsEmitter.TryEmitDurationFromCallCount()).To(Equal(1))
			_, name, start := fakeMetricsEmitter.TryEmitDurationFromArgsForCall(0)
			Expect(name).To(Equal(groot.MetricImageStatsTime))
			Expect(start).NotTo(BeZero())
		})

		Context("when imageCloner fails", func() {
			It("returns an error", func() {
				fakeImageCloner.StatsReturns(groot.VolumeStats{}, errors.New("sorry"))

				_, err := statser.Stats(logger, "some-id")
				Expect(err).To(MatchError(ContainSubstring("sorry")))
			})
		})
	})
})
