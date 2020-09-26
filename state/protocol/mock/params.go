// Code generated by mockery v1.0.0. DO NOT EDIT.

package mock

import (
	flow "github.com/onflow/flow-go/model/flow"
	mock "github.com/stretchr/testify/mock"
)

// Params is an autogenerated mock type for the Params type
type Params struct {
	mock.Mock
}

// ChainID provides a mock function with given fields:
func (_m *Params) ChainID() (flow.ChainID, error) {
	ret := _m.Called()

	var r0 flow.ChainID
	if rf, ok := ret.Get(0).(func() flow.ChainID); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(flow.ChainID)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Root provides a mock function with given fields:
func (_m *Params) Root() (*flow.Header, error) {
	ret := _m.Called()

	var r0 *flow.Header
	if rf, ok := ret.Get(0).(func() *flow.Header); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.Header)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Seal provides a mock function with given fields:
func (_m *Params) Seal() (*flow.Seal, error) {
	ret := _m.Called()

	var r0 *flow.Seal
	if rf, ok := ret.Get(0).(func() *flow.Seal); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.Seal)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
