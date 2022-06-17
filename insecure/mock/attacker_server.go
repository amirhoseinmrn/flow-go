// Code generated by mockery v2.12.1. DO NOT EDIT.

package mockinsecure

import (
	mock "github.com/stretchr/testify/mock"

	insecure "github.com/onflow/flow-go/insecure"

	testing "testing"
)

// AttackerServer is an autogenerated mock type for the AttackerServer type
type AttackerServer struct {
	mock.Mock
}

// Observe provides a mock function with given fields: _a0
func (_m *AttackerServer) Observe(_a0 insecure.Attacker_ObserveServer) error {
	ret := _m.Called(_a0)

	var r0 error
	if rf, ok := ret.Get(0).(func(insecure.Attacker_ObserveServer) error); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewAttackerServer creates a new instance of AttackerServer. It also registers the testing.TB interface on the mock and a cleanup function to assert the mocks expectations.
func NewAttackerServer(t testing.TB) *AttackerServer {
	mock := &AttackerServer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
