// Code generated by mockery v2.12.1. DO NOT EDIT.

package mockinsecure

import (
	context "context"

	emptypb "google.golang.org/protobuf/types/known/emptypb"

	insecure "github.com/onflow/flow-go/insecure"

	mock "github.com/stretchr/testify/mock"

	testing "testing"
)

// CorruptibleConduitFactoryServer is an autogenerated mock type for the CorruptibleConduitFactoryServer type
type CorruptibleConduitFactoryServer struct {
	mock.Mock
}

// ProcessAttackerMessage provides a mock function with given fields: _a0
func (_m *CorruptibleConduitFactoryServer) ProcessAttackerMessage(_a0 insecure.CorruptibleConduitFactory_ProcessAttackerMessageServer) error {
	ret := _m.Called(_a0)

	var r0 error
	if rf, ok := ret.Get(0).(func(insecure.CorruptibleConduitFactory_ProcessAttackerMessageServer) error); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// RegisterAttacker provides a mock function with given fields: _a0, _a1
func (_m *CorruptibleConduitFactoryServer) RegisterAttacker(_a0 context.Context, _a1 *insecure.AttackerRegisterMessage) (*emptypb.Empty, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *emptypb.Empty
	if rf, ok := ret.Get(0).(func(context.Context, *insecure.AttackerRegisterMessage) *emptypb.Empty); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*emptypb.Empty)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *insecure.AttackerRegisterMessage) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewCorruptibleConduitFactoryServer creates a new instance of CorruptibleConduitFactoryServer. It also registers the testing.TB interface on the mock and a cleanup function to assert the mocks expectations.
func NewCorruptibleConduitFactoryServer(t testing.TB) *CorruptibleConduitFactoryServer {
	mock := &CorruptibleConduitFactoryServer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
