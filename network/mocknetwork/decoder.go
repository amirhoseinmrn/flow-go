// Code generated by mockery v2.12.1. DO NOT EDIT.

package mocknetwork

import (
	mock "github.com/stretchr/testify/mock"

	testing "testing"
)

// Decoder is an autogenerated mock type for the Decoder type
type Decoder struct {
	mock.Mock
}

// Decode provides a mock function with given fields:
func (_m *Decoder) Decode() (interface{}, error) {
	ret := _m.Called()

	var r0 interface{}
	if rf, ok := ret.Get(0).(func() interface{}); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(interface{})
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

// NewDecoder creates a new instance of Decoder. It also registers the testing.TB interface on the mock and a cleanup function to assert the mocks expectations.
func NewDecoder(t testing.TB) *Decoder {
	mock := &Decoder{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
