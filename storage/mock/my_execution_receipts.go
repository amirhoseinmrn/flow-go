// Code generated by mockery v2.12.1. DO NOT EDIT.

package mock

import (
	mock "github.com/stretchr/testify/mock"

	flow "github.com/onflow/flow-go/model/flow"

	storage "github.com/onflow/flow-go/storage"

	testing "testing"
)

// MyExecutionReceipts is an autogenerated mock type for the MyExecutionReceipts type
type MyExecutionReceipts struct {
	mock.Mock
}

// BatchStoreMyReceipt provides a mock function with given fields: receipt, batch
func (_m *MyExecutionReceipts) BatchStoreMyReceipt(receipt *flow.ExecutionReceipt, batch storage.BatchStorage) error {
	ret := _m.Called(receipt, batch)

	var r0 error
	if rf, ok := ret.Get(0).(func(*flow.ExecutionReceipt, storage.BatchStorage) error); ok {
		r0 = rf(receipt, batch)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MyReceipt provides a mock function with given fields: blockID
func (_m *MyExecutionReceipts) MyReceipt(blockID flow.Identifier) (*flow.ExecutionReceipt, error) {
	ret := _m.Called(blockID)

	var r0 *flow.ExecutionReceipt
	if rf, ok := ret.Get(0).(func(flow.Identifier) *flow.ExecutionReceipt); ok {
		r0 = rf(blockID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.ExecutionReceipt)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(flow.Identifier) error); ok {
		r1 = rf(blockID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// StoreMyReceipt provides a mock function with given fields: receipt
func (_m *MyExecutionReceipts) StoreMyReceipt(receipt *flow.ExecutionReceipt) error {
	ret := _m.Called(receipt)

	var r0 error
	if rf, ok := ret.Get(0).(func(*flow.ExecutionReceipt) error); ok {
		r0 = rf(receipt)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewMyExecutionReceipts creates a new instance of MyExecutionReceipts. It also registers the testing.TB interface on the mock and a cleanup function to assert the mocks expectations.
func NewMyExecutionReceipts(t testing.TB) *MyExecutionReceipts {
	mock := &MyExecutionReceipts{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
