// Code generated by mockery v2.13.1. DO NOT EDIT.

package mocks

import (
	flow "github.com/onflow/flow-go/model/flow"

	mock "github.com/stretchr/testify/mock"

	model "github.com/onflow/flow-go/consensus/hotstuff/model"
)

// Consumer is an autogenerated mock type for the Consumer type
type Consumer struct {
	mock.Mock
}

// OnBlockIncorporated provides a mock function with given fields: _a0
func (_m *Consumer) OnBlockIncorporated(_a0 *model.Block) {
	_m.Called(_a0)
}

// OnDoubleProposeDetected provides a mock function with given fields: _a0, _a1
func (_m *Consumer) OnDoubleProposeDetected(_a0 *model.Block, _a1 *model.Block) {
	_m.Called(_a0, _a1)
}

// OnDoubleVotingDetected provides a mock function with given fields: _a0, _a1
func (_m *Consumer) OnDoubleVotingDetected(_a0 *model.Vote, _a1 *model.Vote) {
	_m.Called(_a0, _a1)
}

// OnEnteringView provides a mock function with given fields: viewNumber, leader
func (_m *Consumer) OnEnteringView(viewNumber uint64, leader flow.Identifier) {
	_m.Called(viewNumber, leader)
}

// OnEventProcessed provides a mock function with given fields:
func (_m *Consumer) OnEventProcessed() {
	_m.Called()
}

// OnFinalizedBlock provides a mock function with given fields: _a0
func (_m *Consumer) OnFinalizedBlock(_a0 *model.Block) {
	_m.Called(_a0)
}

// OnForkChoiceGenerated provides a mock function with given fields: _a0, _a1
func (_m *Consumer) OnForkChoiceGenerated(_a0 uint64, _a1 *flow.QuorumCertificate) {
	_m.Called(_a0, _a1)
}

// OnInvalidVoteDetected provides a mock function with given fields: _a0
func (_m *Consumer) OnInvalidVoteDetected(_a0 *model.Vote) {
	_m.Called(_a0)
}

// OnProposingBlock provides a mock function with given fields: proposal
func (_m *Consumer) OnProposingBlock(proposal *model.Proposal) {
	_m.Called(proposal)
}

// OnQcConstructedFromVotes provides a mock function with given fields: curView, qc
func (_m *Consumer) OnQcConstructedFromVotes(curView uint64, qc *flow.QuorumCertificate) {
	_m.Called(curView, qc)
}

// OnQcIncorporated provides a mock function with given fields: _a0
func (_m *Consumer) OnQcIncorporated(_a0 *flow.QuorumCertificate) {
	_m.Called(_a0)
}

// OnQcTriggeredViewChange provides a mock function with given fields: qc, newView
func (_m *Consumer) OnQcTriggeredViewChange(qc *flow.QuorumCertificate, newView uint64) {
	_m.Called(qc, newView)
}

// OnReachedTimeout provides a mock function with given fields: timeout
func (_m *Consumer) OnReachedTimeout(timeout *model.TimerInfo) {
	_m.Called(timeout)
}

// OnReceiveProposal provides a mock function with given fields: currentView, proposal
func (_m *Consumer) OnReceiveProposal(currentView uint64, proposal *model.Proposal) {
	_m.Called(currentView, proposal)
}

// OnReceiveVote provides a mock function with given fields: currentView, vote
func (_m *Consumer) OnReceiveVote(currentView uint64, vote *model.Vote) {
	_m.Called(currentView, vote)
}

// OnStartingTimeout provides a mock function with given fields: _a0
func (_m *Consumer) OnStartingTimeout(_a0 *model.TimerInfo) {
	_m.Called(_a0)
}

// OnVoteForInvalidBlockDetected provides a mock function with given fields: vote, invalidProposal
func (_m *Consumer) OnVoteForInvalidBlockDetected(vote *model.Vote, invalidProposal *model.Proposal) {
	_m.Called(vote, invalidProposal)
}

// OnVoting provides a mock function with given fields: vote
func (_m *Consumer) OnVoting(vote *model.Vote) {
	_m.Called(vote)
}

type mockConstructorTestingTNewConsumer interface {
	mock.TestingT
	Cleanup(func())
}

// NewConsumer creates a new instance of Consumer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewConsumer(t mockConstructorTestingTNewConsumer) *Consumer {
	mock := &Consumer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
