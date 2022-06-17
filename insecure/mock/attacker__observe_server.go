// Code generated by mockery v2.12.1. DO NOT EDIT.

package mockinsecure

import (
	context "context"

	emptypb "google.golang.org/protobuf/types/known/emptypb"

	insecure "github.com/onflow/flow-go/insecure"

	metadata "google.golang.org/grpc/metadata"

	mock "github.com/stretchr/testify/mock"

	testing "testing"
)

// Attacker_ObserveServer is an autogenerated mock type for the Attacker_ObserveServer type
type Attacker_ObserveServer struct {
	mock.Mock
}

// Context provides a mock function with given fields:
func (_m *Attacker_ObserveServer) Context() context.Context {
	ret := _m.Called()

	var r0 context.Context
	if rf, ok := ret.Get(0).(func() context.Context); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(context.Context)
		}
	}

	return r0
}

// Recv provides a mock function with given fields:
func (_m *Attacker_ObserveServer) Recv() (*insecure.Message, error) {
	ret := _m.Called()

	var r0 *insecure.Message
	if rf, ok := ret.Get(0).(func() *insecure.Message); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*insecure.Message)
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

// RecvMsg provides a mock function with given fields: m
func (_m *Attacker_ObserveServer) RecvMsg(m interface{}) error {
	ret := _m.Called(m)

	var r0 error
	if rf, ok := ret.Get(0).(func(interface{}) error); ok {
		r0 = rf(m)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SendAndClose provides a mock function with given fields: _a0
func (_m *Attacker_ObserveServer) SendAndClose(_a0 *emptypb.Empty) error {
	ret := _m.Called(_a0)

	var r0 error
	if rf, ok := ret.Get(0).(func(*emptypb.Empty) error); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SendHeader provides a mock function with given fields: _a0
func (_m *Attacker_ObserveServer) SendHeader(_a0 metadata.MD) error {
	ret := _m.Called(_a0)

	var r0 error
	if rf, ok := ret.Get(0).(func(metadata.MD) error); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SendMsg provides a mock function with given fields: m
func (_m *Attacker_ObserveServer) SendMsg(m interface{}) error {
	ret := _m.Called(m)

	var r0 error
	if rf, ok := ret.Get(0).(func(interface{}) error); ok {
		r0 = rf(m)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SetHeader provides a mock function with given fields: _a0
func (_m *Attacker_ObserveServer) SetHeader(_a0 metadata.MD) error {
	ret := _m.Called(_a0)

	var r0 error
	if rf, ok := ret.Get(0).(func(metadata.MD) error); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SetTrailer provides a mock function with given fields: _a0
func (_m *Attacker_ObserveServer) SetTrailer(_a0 metadata.MD) {
	_m.Called(_a0)
}

// NewAttacker_ObserveServer creates a new instance of Attacker_ObserveServer. It also registers the testing.TB interface on the mock and a cleanup function to assert the mocks expectations.
func NewAttacker_ObserveServer(t testing.TB) *Attacker_ObserveServer {
	mock := &Attacker_ObserveServer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
