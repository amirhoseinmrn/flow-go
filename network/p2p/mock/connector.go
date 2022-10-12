// Code generated by mockery v2.13.1. DO NOT EDIT.

package mockp2p

import (
	context "context"

	mock "github.com/stretchr/testify/mock"

	peer "github.com/libp2p/go-libp2p/core/peer"
)

// Connector is an autogenerated mock type for the Connector type
type Connector struct {
	mock.Mock
}

// UpdatePeers provides a mock function with given fields: ctx, peerIDs
func (_m *Connector) UpdatePeers(ctx context.Context, peerIDs peer.IDSlice) {
	_m.Called(ctx, peerIDs)
}

type mockConstructorTestingTNewConnector interface {
	mock.TestingT
	Cleanup(func())
}

// NewConnector creates a new instance of Connector. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewConnector(t mockConstructorTestingTNewConnector) *Connector {
	mock := &Connector{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
