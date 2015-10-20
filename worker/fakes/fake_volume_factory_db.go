// This file was generated by counterfeiter
package fakes

import (
	"sync"
	"time"

	"github.com/concourse/atc/worker"
)

type FakeVolumeFactoryDB struct {
	GetVolumeTTLStub        func(volumeHandle string) (time.Duration, error)
	getVolumeTTLMutex       sync.RWMutex
	getVolumeTTLArgsForCall []struct {
		volumeHandle string
	}
	getVolumeTTLReturns struct {
		result1 time.Duration
		result2 error
	}
}

func (fake *FakeVolumeFactoryDB) GetVolumeTTL(volumeHandle string) (time.Duration, error) {
	fake.getVolumeTTLMutex.Lock()
	fake.getVolumeTTLArgsForCall = append(fake.getVolumeTTLArgsForCall, struct {
		volumeHandle string
	}{volumeHandle})
	fake.getVolumeTTLMutex.Unlock()
	if fake.GetVolumeTTLStub != nil {
		return fake.GetVolumeTTLStub(volumeHandle)
	} else {
		return fake.getVolumeTTLReturns.result1, fake.getVolumeTTLReturns.result2
	}
}

func (fake *FakeVolumeFactoryDB) GetVolumeTTLCallCount() int {
	fake.getVolumeTTLMutex.RLock()
	defer fake.getVolumeTTLMutex.RUnlock()
	return len(fake.getVolumeTTLArgsForCall)
}

func (fake *FakeVolumeFactoryDB) GetVolumeTTLArgsForCall(i int) string {
	fake.getVolumeTTLMutex.RLock()
	defer fake.getVolumeTTLMutex.RUnlock()
	return fake.getVolumeTTLArgsForCall[i].volumeHandle
}

func (fake *FakeVolumeFactoryDB) GetVolumeTTLReturns(result1 time.Duration, result2 error) {
	fake.GetVolumeTTLStub = nil
	fake.getVolumeTTLReturns = struct {
		result1 time.Duration
		result2 error
	}{result1, result2}
}

var _ worker.VolumeFactoryDB = new(FakeVolumeFactoryDB)
