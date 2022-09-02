// Code generated by mockery v2.13.1. DO NOT EDIT.

package mocknetwork

import mock "github.com/stretchr/testify/mock"

// Encoder is an autogenerated mock type for the Encoder type
type Encoder struct {
	mock.Mock
}

// Encode provides a mock function with given fields: v
func (_m *Encoder) Encode(v interface{}) error {
	ret := _m.Called(v)

	var r0 error
	if rf, ok := ret.Get(0).(func(interface{}) error); ok {
		r0 = rf(v)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type mockConstructorTestingTNewEncoder interface {
	mock.TestingT
	Cleanup(func())
}

// NewEncoder creates a new instance of Encoder. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewEncoder(t mockConstructorTestingTNewEncoder) *Encoder {
	mock := &Encoder{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
