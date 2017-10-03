// Code generated by counterfeiter. DO NOT EDIT.
package layer_fetcherfakes

import (
	"net/url"
	"sync"

	"code.cloudfoundry.org/grootfs/fetcher/layer_fetcher"
	"code.cloudfoundry.org/lager"
	"github.com/containers/image/types"
)

type FakeSource struct {
	ManifestStub        func(logger lager.Logger, baseImageURL *url.URL) (types.Image, error)
	manifestMutex       sync.RWMutex
	manifestArgsForCall []struct {
		logger       lager.Logger
		baseImageURL *url.URL
	}
	manifestReturns struct {
		result1 types.Image
		result2 error
	}
	manifestReturnsOnCall map[int]struct {
		result1 types.Image
		result2 error
	}
	BlobStub        func(logger lager.Logger, baseImageURL *url.URL, digest string, layersURLs []string) (string, int64, error)
	blobMutex       sync.RWMutex
	blobArgsForCall []struct {
		logger       lager.Logger
		baseImageURL *url.URL
		digest       string
		layersURLs   []string
	}
	blobReturns struct {
		result1 string
		result2 int64
		result3 error
	}
	blobReturnsOnCall map[int]struct {
		result1 string
		result2 int64
		result3 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeSource) Manifest(logger lager.Logger, baseImageURL *url.URL) (types.Image, error) {
	fake.manifestMutex.Lock()
	ret, specificReturn := fake.manifestReturnsOnCall[len(fake.manifestArgsForCall)]
	fake.manifestArgsForCall = append(fake.manifestArgsForCall, struct {
		logger       lager.Logger
		baseImageURL *url.URL
	}{logger, baseImageURL})
	fake.recordInvocation("Manifest", []interface{}{logger, baseImageURL})
	fake.manifestMutex.Unlock()
	if fake.ManifestStub != nil {
		return fake.ManifestStub(logger, baseImageURL)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fake.manifestReturns.result1, fake.manifestReturns.result2
}

func (fake *FakeSource) ManifestCallCount() int {
	fake.manifestMutex.RLock()
	defer fake.manifestMutex.RUnlock()
	return len(fake.manifestArgsForCall)
}

func (fake *FakeSource) ManifestArgsForCall(i int) (lager.Logger, *url.URL) {
	fake.manifestMutex.RLock()
	defer fake.manifestMutex.RUnlock()
	return fake.manifestArgsForCall[i].logger, fake.manifestArgsForCall[i].baseImageURL
}

func (fake *FakeSource) ManifestReturns(result1 types.Image, result2 error) {
	fake.ManifestStub = nil
	fake.manifestReturns = struct {
		result1 types.Image
		result2 error
	}{result1, result2}
}

func (fake *FakeSource) ManifestReturnsOnCall(i int, result1 types.Image, result2 error) {
	fake.ManifestStub = nil
	if fake.manifestReturnsOnCall == nil {
		fake.manifestReturnsOnCall = make(map[int]struct {
			result1 types.Image
			result2 error
		})
	}
	fake.manifestReturnsOnCall[i] = struct {
		result1 types.Image
		result2 error
	}{result1, result2}
}

func (fake *FakeSource) Blob(logger lager.Logger, baseImageURL *url.URL, digest string, layersURLs []string) (string, int64, error) {
	var layersURLsCopy []string
	if layersURLs != nil {
		layersURLsCopy = make([]string, len(layersURLs))
		copy(layersURLsCopy, layersURLs)
	}
	fake.blobMutex.Lock()
	ret, specificReturn := fake.blobReturnsOnCall[len(fake.blobArgsForCall)]
	fake.blobArgsForCall = append(fake.blobArgsForCall, struct {
		logger       lager.Logger
		baseImageURL *url.URL
		digest       string
		layersURLs   []string
	}{logger, baseImageURL, digest, layersURLsCopy})
	fake.recordInvocation("Blob", []interface{}{logger, baseImageURL, digest, layersURLsCopy})
	fake.blobMutex.Unlock()
	if fake.BlobStub != nil {
		return fake.BlobStub(logger, baseImageURL, digest, layersURLs)
	}
	if specificReturn {
		return ret.result1, ret.result2, ret.result3
	}
	return fake.blobReturns.result1, fake.blobReturns.result2, fake.blobReturns.result3
}

func (fake *FakeSource) BlobCallCount() int {
	fake.blobMutex.RLock()
	defer fake.blobMutex.RUnlock()
	return len(fake.blobArgsForCall)
}

func (fake *FakeSource) BlobArgsForCall(i int) (lager.Logger, *url.URL, string, []string) {
	fake.blobMutex.RLock()
	defer fake.blobMutex.RUnlock()
	return fake.blobArgsForCall[i].logger, fake.blobArgsForCall[i].baseImageURL, fake.blobArgsForCall[i].digest, fake.blobArgsForCall[i].layersURLs
}

func (fake *FakeSource) BlobReturns(result1 string, result2 int64, result3 error) {
	fake.BlobStub = nil
	fake.blobReturns = struct {
		result1 string
		result2 int64
		result3 error
	}{result1, result2, result3}
}

func (fake *FakeSource) BlobReturnsOnCall(i int, result1 string, result2 int64, result3 error) {
	fake.BlobStub = nil
	if fake.blobReturnsOnCall == nil {
		fake.blobReturnsOnCall = make(map[int]struct {
			result1 string
			result2 int64
			result3 error
		})
	}
	fake.blobReturnsOnCall[i] = struct {
		result1 string
		result2 int64
		result3 error
	}{result1, result2, result3}
}

func (fake *FakeSource) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.manifestMutex.RLock()
	defer fake.manifestMutex.RUnlock()
	fake.blobMutex.RLock()
	defer fake.blobMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeSource) recordInvocation(key string, args []interface{}) {
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

var _ layer_fetcher.Source = new(FakeSource)
