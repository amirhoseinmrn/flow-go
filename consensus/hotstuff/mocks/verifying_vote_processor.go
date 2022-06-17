// Code generated by mockery v2.12.1. DO NOT EDIT.

package mocks

import (
	mock "github.com/stretchr/testify/mock"

	hotstuff "github.com/onflow/flow-go/consensus/hotstuff"

	model "github.com/onflow/flow-go/consensus/hotstuff/model"

	testing "testing"
)

// VerifyingVoteProcessor is an autogenerated mock type for the VerifyingVoteProcessor type
type VerifyingVoteProcessor struct {
	mock.Mock
}

// Block provides a mock function with given fields:
func (_m *VerifyingVoteProcessor) Block() *model.Block {
	ret := _m.Called()

	var r0 *model.Block
	if rf, ok := ret.Get(0).(func() *model.Block); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.Block)
		}
	}

	return r0
}

// Process provides a mock function with given fields: vote
func (_m *VerifyingVoteProcessor) Process(vote *model.Vote) error {
	ret := _m.Called(vote)

	var r0 error
	if rf, ok := ret.Get(0).(func(*model.Vote) error); ok {
		r0 = rf(vote)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Status provides a mock function with given fields:
func (_m *VerifyingVoteProcessor) Status() hotstuff.VoteCollectorStatus {
	ret := _m.Called()

	var r0 hotstuff.VoteCollectorStatus
	if rf, ok := ret.Get(0).(func() hotstuff.VoteCollectorStatus); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(hotstuff.VoteCollectorStatus)
	}

	return r0
}

// NewVerifyingVoteProcessor creates a new instance of VerifyingVoteProcessor. It also registers the testing.TB interface on the mock and a cleanup function to assert the mocks expectations.
func NewVerifyingVoteProcessor(t testing.TB) *VerifyingVoteProcessor {
	mock := &VerifyingVoteProcessor{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
