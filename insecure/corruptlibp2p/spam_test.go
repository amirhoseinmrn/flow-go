package corruptlibp2p_test

import (
	"context"
	"sync"
	"testing"
	"time"

	pb "github.com/libp2p/go-libp2p-pubsub/pb"

	"github.com/onflow/flow-go/network/p2p/utils"

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
	factory := corruptlibp2p.CorruptGossipSubFactory(func(r *corrupt.GossipSubRouter) {
		require.NotNil(t, r)
		router.setRouter(r)
	})

	spammerNode, _ := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		internal.WithCorruptGossipSub(factory,
			corruptlibp2p.CorruptGossipSubConfigFactoryWithInspector(func(id peer.ID, rpc *corrupt.RPC) error {
				// here we can inspect the incoming RPC message to the spammer node
				return nil
			})),
	)

	receivedAllMsgs := make(chan struct{})

	// keeps track of how many messages victim received from spammer - to know when to stop listening for more messages
	receivedCounter := 0
	var iHaveReceivedCtlMsgs []pb.ControlMessage
	victimNode, victimId := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		internal.WithCorruptGossipSub(factory,
			corruptlibp2p.CorruptGossipSubConfigFactoryWithInspector(func(id peer.ID, rpc *corrupt.RPC) error {
				iHaves := rpc.GetControl().GetIhave()
				if len(iHaves) == 0 {
					// don't inspect control messages with no IHAVE messages
					return nil
				}
				receivedCounter++
				iHaveReceivedCtlMsgs = append(iHaveReceivedCtlMsgs, *rpc.GetControl())

				if receivedCounter == messagesToSpam {
					close(receivedAllMsgs) // acknowledge victim received all of spammer's messages
				}
				return nil
			})),
	)
	victimPeerId, err := unittest.PeerIDFromFlowID(&victimId)
	require.NoError(t, err)

	victimPeerInfo, err := utils.PeerAddressInfo(victimId)
	require.NoError(t, err)

	// starts nodes
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	defer cancel()

	p2ptest.StartNodes(t, signalerCtx, []p2p.LibP2PNode{spammerNode, victimNode}, 100*time.Second)
	defer p2ptest.StopNodes(t, []p2p.LibP2PNode{spammerNode, victimNode}, cancel, 100*time.Second)

	// connect spammer and victim
	err = spammerNode.AddPeer(ctx, victimPeerInfo)
	require.NoError(t, err)
	connected, err := spammerNode.IsConnected(victimPeerInfo.ID)
	require.NoError(t, err)
	require.True(t, connected)

	// create new spammer
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(router.getRouter())
	require.NotNil(t, router)

	// prepare to spam - generate IHAVE control messages
	iHaveSentCtlMsgs := spammer.GenerateIHaveCtlMessages(t, messagesToSpam, 5)

	// start spamming the victim peer
	spammer.SpamIHave(victimPeerId, iHaveSentCtlMsgs)

	// check that victim received all spam messages
	select {
	case <-receivedAllMsgs:
		break
	case <-time.After(1 * time.Second):
		require.Fail(t, "did not receive spam messages")
	}

	// check contents of received messages should match what spammer sent
	require.Equal(t, len(iHaveSentCtlMsgs), len(iHaveReceivedCtlMsgs))
	require.ElementsMatch(t, iHaveReceivedCtlMsgs, iHaveSentCtlMsgs)
}

// atomicRouter is a wrapper around the corrupt.GossipSubRouter that allows atomic access to the router.
// This is done to avoid race conditions when accessing the router from multiple goroutines.
type atomicRouter struct {
	mu     sync.Mutex
	router *corrupt.GossipSubRouter
}

func newAtomicRouter() *atomicRouter {
	return &atomicRouter{
		mu: sync.Mutex{},
	}
}

// SetRouter sets the router if it has never been set.
func (a *atomicRouter) setRouter(router *corrupt.GossipSubRouter) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if router == nil {
		a.router = router
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
