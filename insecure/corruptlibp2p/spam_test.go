package corruptlibp2p_test

import (
	"context"
	"github.com/onflow/flow-go/model/flow"
	"sync"
	"testing"
	"time"

	pb "github.com/libp2p/go-libp2p-pubsub/pb"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
	corrupt "github.com/yhassanzadeh13/go-libp2p-pubsub"

	"github.com/onflow/flow-go/insecure/corruptlibp2p"
	"github.com/onflow/flow-go/insecure/internal"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/p2p"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestSpam_IHave sets up a 2 node test between a victim node and a spammer. The spammer sends a few IHAVE control messages
// to the victim node without being subscribed to any of the same topics.
// The test then checks that the victim node received all the messages from the spammer.
func TestSpam_IHave(t *testing.T) {
	const messagesToSpam = 3
	sporkId := unittest.IdentifierFixture()

	router := newAtomicRouter()
	spammerNode, spammerId := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		p2ptest.WithRole(flow.RoleConsensus),
		internal.WithCorruptGossipSub(corruptlibp2p.CorruptGossipSubFactory(func(r *corrupt.GossipSubRouter, ps *corrupt.PubSub) {
			require.NotNil(t, r)
			router.set(r, ps)
		}),
			corruptlibp2p.CorruptGossipSubConfigFactoryWithInspector(func(id peer.ID, rpc *corrupt.RPC) error {
				// here we can inspect the incoming RPC message to the spammer node
				return nil
			})),
	)

	allSpamIHavesReceived := sync.WaitGroup{}
	allSpamIHavesReceived.Add(messagesToSpam)

	// keeps track of how many messages victim received from spammer - to know when to stop listening for more messages
	receivedCounter := 0
	var iHaveReceivedCtlMsgs []pb.ControlMessage
	victimNode, victimId := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		p2ptest.WithRole(flow.RoleConsensus),
		internal.WithCorruptGossipSub(corruptlibp2p.CorruptGossipSubFactory(),
			corruptlibp2p.CorruptGossipSubConfigFactoryWithInspector(func(id peer.ID, rpc *corrupt.RPC) error {
				iHaves := rpc.GetControl().GetIhave()
				if len(iHaves) == 0 {
					// don't inspect control messages with no IHAVE messages
					return nil
				}
				receivedCounter++
				iHaveReceivedCtlMsgs = append(iHaveReceivedCtlMsgs, *rpc.GetControl())
				allSpamIHavesReceived.Done() // acknowledge that victim received a message.
				return nil
			})),
	)

	// starts nodes
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	defer cancel()
	nodes := []p2p.LibP2PNode{spammerNode, victimNode}
	p2ptest.StartNodes(t, signalerCtx, nodes, 5*time.Second)
	defer p2ptest.StopNodes(t, nodes, cancel, 5*time.Second)

	require.Eventuallyf(t, func() bool {
		// ensuring the spammer router has been initialized.
		// this is needed because the router is initialized asynchronously.
		return router.getRouter() != nil
	}, 1*time.Second, 100*time.Millisecond, "spammer router not set")

	// prior to the test we ensure that
	p2ptest.EnsureConnected(t, ctx, nodes)
	p2ptest.LetNodesDiscoverEachOther(t, ctx, nodes, flow.IdentityList{&spammerId, &victimId})
	p2ptest.EnsureConnected(t, ctx, nodes)
	p2ptest.EnsureStreamCreationInBothDirections(t, ctx, nodes)
	//p2ptest.EnsurePubsubMessageExchange(t, ctx, nodes, func() (interface{}, channels.Topic) {
	//	blockTopic := channels.TopicFromChannel(channels.PushBlocks, sporkId)
	//	return unittest.ProposalFixture(), blockTopic
	//})

	// create new spammer
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(router.getRouter())
	require.NotNil(t, router)

	// prepare to spam - generate IHAVE control messages
	iHaveSentCtlMsgs := spammer.GenerateIHaveCtlMessages(t, messagesToSpam, 5)

	// start spamming the victim peer
	spammer.SpamIHave(t, victimNode.Host().ID(), iHaveSentCtlMsgs)

	// check that victim received all spam messages
	unittest.RequireReturnsBefore(t, allSpamIHavesReceived.Wait, 1*time.Second, "victim did not receive all spam messages")

	// check contents of received messages should match what spammer sent
	require.Equal(t, len(iHaveSentCtlMsgs), len(iHaveReceivedCtlMsgs))
	require.ElementsMatch(t, iHaveReceivedCtlMsgs, iHaveSentCtlMsgs)
}

// atomicRouter is a wrapper around the corrupt.GossipSubRouter that allows atomic access to the router.
// This is done to avoid race conditions when accessing the router from multiple goroutines.
type atomicRouter struct {
	mu     sync.Mutex
	router *corrupt.GossipSubRouter
	ps     *corrupt.PubSub
}

func newAtomicRouter() *atomicRouter {
	return &atomicRouter{
		mu: sync.Mutex{},
	}
}

// SetRouter sets the router if it has never been set.
func (a *atomicRouter) set(router *corrupt.GossipSubRouter, ps *corrupt.PubSub) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.router == nil && a.ps == nil {
		a.router = router
		a.ps = ps
		return true
	}
	return false
}

// GetRouter returns the router.
func (a *atomicRouter) getRouter() *corrupt.GossipSubRouter {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.router
}

func (a *atomicRouter) getPubSub() *corrupt.PubSub {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.ps
}
