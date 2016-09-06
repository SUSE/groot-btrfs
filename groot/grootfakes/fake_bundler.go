// This file was generated by counterfeiter
package grootfakes

import (
	"sync"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"
)

type FakeBundler struct {
	ExistsStub        func(id string) (bool, error)
	existsMutex       sync.RWMutex
	existsArgsForCall []struct {
		id string
	}
	existsReturns struct {
		result1 bool
		result2 error
	}
	CreateStub        func(logger lager.Logger, id string, spec groot.BundleSpec) (groot.Bundle, error)
	createMutex       sync.RWMutex
	createArgsForCall []struct {
		logger lager.Logger
		id     string
		spec   groot.BundleSpec
	}
	createReturns struct {
		result1 groot.Bundle
		result2 error
	}
	DestroyStub        func(logger lager.Logger, id string) error
	destroyMutex       sync.RWMutex
	destroyArgsForCall []struct {
		logger lager.Logger
		id     string
	}
	destroyReturns struct {
		result1 error
	}
	MetricsStub        func(logger lager.Logger, id string, forceSync bool) (groot.VolumeMetrics, error)
	metricsMutex       sync.RWMutex
	metricsArgsForCall []struct {
		logger    lager.Logger
		id        string
		forceSync bool
	}
	metricsReturns struct {
		result1 groot.VolumeMetrics
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeBundler) Exists(id string) (bool, error) {
	fake.existsMutex.Lock()
	fake.existsArgsForCall = append(fake.existsArgsForCall, struct {
		id string
	}{id})
	fake.recordInvocation("Exists", []interface{}{id})
	fake.existsMutex.Unlock()
	if fake.ExistsStub != nil {
		return fake.ExistsStub(id)
	} else {
		return fake.existsReturns.result1, fake.existsReturns.result2
	}
}

func (fake *FakeBundler) ExistsCallCount() int {
	fake.existsMutex.RLock()
	defer fake.existsMutex.RUnlock()
	return len(fake.existsArgsForCall)
}

func (fake *FakeBundler) ExistsArgsForCall(i int) string {
	fake.existsMutex.RLock()
	defer fake.existsMutex.RUnlock()
	return fake.existsArgsForCall[i].id
}

func (fake *FakeBundler) ExistsReturns(result1 bool, result2 error) {
	fake.ExistsStub = nil
	fake.existsReturns = struct {
		result1 bool
		result2 error
	}{result1, result2}
}

func (fake *FakeBundler) Create(logger lager.Logger, id string, spec groot.BundleSpec) (groot.Bundle, error) {
	fake.createMutex.Lock()
	fake.createArgsForCall = append(fake.createArgsForCall, struct {
		logger lager.Logger
		id     string
		spec   groot.BundleSpec
	}{logger, id, spec})
	fake.recordInvocation("Create", []interface{}{logger, id, spec})
	fake.createMutex.Unlock()
	if fake.CreateStub != nil {
		return fake.CreateStub(logger, id, spec)
	} else {
		return fake.createReturns.result1, fake.createReturns.result2
	}
}

func (fake *FakeBundler) CreateCallCount() int {
	fake.createMutex.RLock()
	defer fake.createMutex.RUnlock()
	return len(fake.createArgsForCall)
}

func (fake *FakeBundler) CreateArgsForCall(i int) (lager.Logger, string, groot.BundleSpec) {
	fake.createMutex.RLock()
	defer fake.createMutex.RUnlock()
	return fake.createArgsForCall[i].logger, fake.createArgsForCall[i].id, fake.createArgsForCall[i].spec
}

func (fake *FakeBundler) CreateReturns(result1 groot.Bundle, result2 error) {
	fake.CreateStub = nil
	fake.createReturns = struct {
		result1 groot.Bundle
		result2 error
	}{result1, result2}
}

func (fake *FakeBundler) Destroy(logger lager.Logger, id string) error {
	fake.destroyMutex.Lock()
	fake.destroyArgsForCall = append(fake.destroyArgsForCall, struct {
		logger lager.Logger
		id     string
	}{logger, id})
	fake.recordInvocation("Destroy", []interface{}{logger, id})
	fake.destroyMutex.Unlock()
	if fake.DestroyStub != nil {
		return fake.DestroyStub(logger, id)
	} else {
		return fake.destroyReturns.result1
	}
}

func (fake *FakeBundler) DestroyCallCount() int {
	fake.destroyMutex.RLock()
	defer fake.destroyMutex.RUnlock()
	return len(fake.destroyArgsForCall)
}

func (fake *FakeBundler) DestroyArgsForCall(i int) (lager.Logger, string) {
	fake.destroyMutex.RLock()
	defer fake.destroyMutex.RUnlock()
	return fake.destroyArgsForCall[i].logger, fake.destroyArgsForCall[i].id
}

func (fake *FakeBundler) DestroyReturns(result1 error) {
	fake.DestroyStub = nil
	fake.destroyReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeBundler) Metrics(logger lager.Logger, id string, forceSync bool) (groot.VolumeMetrics, error) {
	fake.metricsMutex.Lock()
	fake.metricsArgsForCall = append(fake.metricsArgsForCall, struct {
		logger    lager.Logger
		id        string
		forceSync bool
	}{logger, id, forceSync})
	fake.recordInvocation("Metrics", []interface{}{logger, id, forceSync})
	fake.metricsMutex.Unlock()
	if fake.MetricsStub != nil {
		return fake.MetricsStub(logger, id, forceSync)
	} else {
		return fake.metricsReturns.result1, fake.metricsReturns.result2
	}
}

func (fake *FakeBundler) MetricsCallCount() int {
	fake.metricsMutex.RLock()
	defer fake.metricsMutex.RUnlock()
	return len(fake.metricsArgsForCall)
}

func (fake *FakeBundler) MetricsArgsForCall(i int) (lager.Logger, string, bool) {
	fake.metricsMutex.RLock()
	defer fake.metricsMutex.RUnlock()
	return fake.metricsArgsForCall[i].logger, fake.metricsArgsForCall[i].id, fake.metricsArgsForCall[i].forceSync
}

func (fake *FakeBundler) MetricsReturns(result1 groot.VolumeMetrics, result2 error) {
	fake.MetricsStub = nil
	fake.metricsReturns = struct {
		result1 groot.VolumeMetrics
		result2 error
	}{result1, result2}
}

func (fake *FakeBundler) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.existsMutex.RLock()
	defer fake.existsMutex.RUnlock()
	fake.createMutex.RLock()
	defer fake.createMutex.RUnlock()
	fake.destroyMutex.RLock()
	defer fake.destroyMutex.RUnlock()
	fake.metricsMutex.RLock()
	defer fake.metricsMutex.RUnlock()
	return fake.invocations
}

func (fake *FakeBundler) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}

var _ groot.Bundler = new(FakeBundler)
