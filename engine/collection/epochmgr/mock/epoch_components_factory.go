// Code generated by mockery v2.13.0. DO NOT EDIT.

package mock

import (
	cluster "github.com/onflow/flow-go/state/cluster"

	hotstuff "github.com/onflow/flow-go/consensus/hotstuff"

	mock "github.com/stretchr/testify/mock"

	module "github.com/onflow/flow-go/module"

	network "github.com/onflow/flow-go/network"

	protocol "github.com/onflow/flow-go/state/protocol"
)

// EpochComponentsFactory is an autogenerated mock type for the EpochComponentsFactory type
type EpochComponentsFactory struct {
	mock.Mock
}

// Create provides a mock function with given fields: epoch
func (_m *EpochComponentsFactory) Create(epoch protocol.Epoch) (cluster.State, network.Engine, network.Engine, module.HotStuff, hotstuff.VoteAggregator, hotstuff.TimeoutAggregator, error) {
	ret := _m.Called(epoch)

	var r0 cluster.State
	if rf, ok := ret.Get(0).(func(protocol.Epoch) cluster.State); ok {
		r0 = rf(epoch)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(cluster.State)
		}
	}

	var r1 network.Engine
	if rf, ok := ret.Get(1).(func(protocol.Epoch) network.Engine); ok {
		r1 = rf(epoch)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(network.Engine)
		}
	}

	var r2 network.Engine
	if rf, ok := ret.Get(2).(func(protocol.Epoch) network.Engine); ok {
		r2 = rf(epoch)
	} else {
		if ret.Get(2) != nil {
			r2 = ret.Get(2).(network.Engine)
		}
	}

	var r3 module.HotStuff
	if rf, ok := ret.Get(3).(func(protocol.Epoch) module.HotStuff); ok {
		r3 = rf(epoch)
	} else {
		if ret.Get(3) != nil {
			r3 = ret.Get(3).(module.HotStuff)
		}
	}

	var r4 hotstuff.VoteAggregator
	if rf, ok := ret.Get(4).(func(protocol.Epoch) hotstuff.VoteAggregator); ok {
		r4 = rf(epoch)
	} else {
		if ret.Get(4) != nil {
			r4 = ret.Get(4).(hotstuff.VoteAggregator)
		}
	}

	var r5 hotstuff.TimeoutAggregator
	if rf, ok := ret.Get(5).(func(protocol.Epoch) hotstuff.TimeoutAggregator); ok {
		r5 = rf(epoch)
	} else {
		if ret.Get(5) != nil {
			r5 = ret.Get(5).(hotstuff.TimeoutAggregator)
		}
	}

	var r6 error
	if rf, ok := ret.Get(6).(func(protocol.Epoch) error); ok {
		r6 = rf(epoch)
	} else {
		r6 = ret.Error(6)
	}

	return r0, r1, r2, r3, r4, r5, r6
}

type NewEpochComponentsFactoryT interface {
	mock.TestingT
	Cleanup(func())
}

// NewEpochComponentsFactory creates a new instance of EpochComponentsFactory. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewEpochComponentsFactory(t NewEpochComponentsFactoryT) *EpochComponentsFactory {
	mock := &EpochComponentsFactory{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
