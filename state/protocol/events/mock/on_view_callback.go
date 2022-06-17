// Code generated by mockery v2.12.1. DO NOT EDIT.

package mock

import (
	mock "github.com/stretchr/testify/mock"

	flow "github.com/onflow/flow-go/model/flow"

	testing "testing"
)

// OnViewCallback is an autogenerated mock type for the OnViewCallback type
type OnViewCallback struct {
	mock.Mock
}

// Execute provides a mock function with given fields: _a0
func (_m *OnViewCallback) Execute(_a0 *flow.Header) {
	_m.Called(_a0)
}

// NewOnViewCallback creates a new instance of OnViewCallback. It also registers the testing.TB interface on the mock and a cleanup function to assert the mocks expectations.
func NewOnViewCallback(t testing.TB) *OnViewCallback {
	mock := &OnViewCallback{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
