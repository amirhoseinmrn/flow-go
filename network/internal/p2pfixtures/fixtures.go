package p2pfixtures

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	addrutil "github.com/libp2p/go-addr-util"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	flownet "github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/internal/p2putils"
	"github.com/onflow/flow-go/network/internal/testutils"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/p2p"
	p2pdht "github.com/onflow/flow-go/network/p2p/dht"
	"github.com/onflow/flow-go/network/p2p/keyutils"
	"github.com/onflow/flow-go/network/p2p/p2pbuilder"
	validator "github.com/onflow/flow-go/network/validator/pubsub"

	"github.com/onflow/flow-go/network/p2p/unicast"
	"github.com/onflow/flow-go/network/p2p/utils"
	"github.com/onflow/flow-go/utils/unittest"
)

// NetworkingKeyFixtures is a test helper that generates a ECDSA flow key pair.
func NetworkingKeyFixtures(t *testing.T) crypto.PrivateKey {
	seed := unittest.SeedFixture(48)
	key, err := crypto.GeneratePrivateKey(crypto.ECDSASecp256k1, seed)
	require.NoError(t, err)
	return key
}

// SilentNodeFixture returns a TCP listener and a node which never replies
func SilentNodeFixture(t *testing.T) (net.Listener, flow.Identity) {
	key := NetworkingKeyFixtures(t)

	lst, err := net.Listen("tcp4", unittest.DefaultAddress)
	require.NoError(t, err)

	addr, err := manet.FromNetAddr(lst.Addr())
	require.NoError(t, err)

	addrs := []multiaddr.Multiaddr{addr}
	addrs, err = addrutil.ResolveUnspecifiedAddresses(addrs, nil)
	require.NoError(t, err)

	go acceptAndHang(t, lst)

	ip, port, err := p2putils.IPPortFromMultiAddress(addrs...)
	require.NoError(t, err)

	identity := unittest.IdentityFixture(unittest.WithNetworkingKey(key.PublicKey()), unittest.WithAddress(ip+":"+port))
	return lst, *identity
}

func acceptAndHang(t *testing.T, l net.Listener) {
	conns := make([]net.Conn, 0, 10)
	for {
		c, err := l.Accept()
		if err != nil {
			break
		}
		if c != nil {
			conns = append(conns, c)
		}
	}
	for _, c := range conns {
		require.NoError(t, c.Close())
	}
}

type nodeOpt func(p2pbuilder.NodeBuilder)

func WithSubscriptionFilter(filter pubsub.SubscriptionFilter) nodeOpt {
	return func(builder p2pbuilder.NodeBuilder) {
		builder.SetSubscriptionFilter(filter)
	}
}

func CreateNode(t *testing.T, nodeID flow.Identifier, networkKey crypto.PrivateKey, sporkID flow.Identifier, logger zerolog.Logger, opts ...nodeOpt) p2p.LibP2PNode {
	builder := p2pbuilder.NewNodeBuilder(
		logger,
		metrics.NewNoopCollector(),
		unittest.DefaultAddress,
		networkKey,
		sporkID,
		p2pbuilder.DefaultResourceManagerConfig()).
		SetRoutingSystem(func(c context.Context, h host.Host) (routing.Routing, error) {
			return p2pdht.NewDHT(c, h, unicast.FlowDHTProtocolID(sporkID), zerolog.Nop(), metrics.NewNoopCollector())
		}).
		SetResourceManager(testutils.NewResourceManager(t))

	for _, opt := range opts {
		opt(builder)
	}

	libp2pNode, err := builder.Build()
	require.NoError(t, err)

	return libp2pNode
}

// PeerIdFixture creates a random and unique peer ID (libp2p node ID).
func PeerIdFixture(t *testing.T) peer.ID {
	key, err := generateNetworkingKey(unittest.IdentifierFixture())
	require.NoError(t, err)

	pubKey, err := keyutils.LibP2PPublicKeyFromFlow(key.PublicKey())
	require.NoError(t, err)

	peerID, err := peer.IDFromPublicKey(pubKey)
	require.NoError(t, err)

	return peerID
}

// generateNetworkingKey generates a Flow ECDSA key using the given seed
func generateNetworkingKey(s flow.Identifier) (crypto.PrivateKey, error) {
	seed := make([]byte, crypto.KeyGenSeedMinLenECDSASecp256k1)
	copy(seed, s[:])
	return crypto.GeneratePrivateKey(crypto.ECDSASecp256k1, seed)
}

// PeerIdsFixture creates random and unique peer IDs (libp2p node IDs).
func PeerIdsFixture(t *testing.T, n int) []peer.ID {
	peerIDs := make([]peer.ID, n)
	for i := 0; i < n; i++ {
		peerIDs[i] = PeerIdFixture(t)
	}
	return peerIDs
}

// MustEncodeEvent encodes and returns the given event and fails the test if it faces any issue while encoding.
func MustEncodeEvent(t *testing.T, v interface{}, channel channels.Channel) []byte {
	bz, err := unittest.NetworkCodec().Encode(v)
	require.NoError(t, err)

	msg := message.Message{
		ChannelID: channel.String(),
		Payload:   bz,
	}
	data, err := msg.Marshal()
	require.NoError(t, err)

	return data
}

// SubMustReceiveMessage checks that the subscription have received the given message within the given timeout by the context.
func SubMustReceiveMessage(t *testing.T, ctx context.Context, expectedMessage []byte, sub p2p.Subscription) {
	received := make(chan struct{})
	go func() {
		msg, err := sub.Next(ctx)
		require.NoError(t, err)
		require.Equal(t, expectedMessage, msg.Data)
		close(received)
	}()

	select {
	case <-received:
		return
	case <-ctx.Done():
		require.Fail(t, "timeout on receiving expected pubsub message")
	}
}

// SubsMustReceiveMessage checks that all subscriptions receive the given message within the given timeout by the context.
func SubsMustReceiveMessage(t *testing.T, ctx context.Context, expectedMessage []byte, subs []p2p.Subscription) {
	for _, sub := range subs {
		SubMustReceiveMessage(t, ctx, expectedMessage, sub)
	}
}

// SubMustNeverReceiveAnyMessage checks that the subscription never receives any message within the given timeout by the context.
func SubMustNeverReceiveAnyMessage(t *testing.T, ctx context.Context, sub p2p.Subscription) {
	timeouted := make(chan struct{})
	go func() {
		_, err := sub.Next(ctx)
		require.Error(t, err)
		require.ErrorIs(t, err, context.DeadlineExceeded)
		close(timeouted)
	}()

	// wait for the timeout, we choose the timeout to be long enough to make sure that
	// on a happy path the timeout never happens, and short enough to make sure that
	// the test doesn't take too long in case of a failure.
	unittest.RequireCloseBefore(t, timeouted, 10*time.Second, "timeout did not happen on receiving expected pubsub message")
}

// HasSubReceivedMessage checks that the subscription have received the given message within the given timeout by the context.
// It returns true if the subscription has received the message, false otherwise.
func HasSubReceivedMessage(t *testing.T, ctx context.Context, expectedMessage []byte, sub p2p.Subscription) bool {
	received := make(chan struct{})
	go func() {
		msg, err := sub.Next(ctx)
		if err != nil {
			require.ErrorIs(t, err, context.DeadlineExceeded)
			return
		}
		if !bytes.Equal(expectedMessage, msg.Data) {
			return
		}
		close(received)
	}()

	select {
	case <-received:
		return true
	case <-ctx.Done():
		return false
	}
}

// SubsMustNeverReceiveAnyMessage checks that all subscriptions never receive any message within the given timeout by the context.
func SubsMustNeverReceiveAnyMessage(t *testing.T, ctx context.Context, subs []p2p.Subscription) {
	for _, sub := range subs {
		SubMustNeverReceiveAnyMessage(t, ctx, sub)
	}
}

// AddNodesToEachOthersPeerStore adds the dialing address of all nodes to the peer store of all other nodes.
// However, it does not connect them to each other.
func AddNodesToEachOthersPeerStore(t *testing.T, nodes []p2p.LibP2PNode, ids flow.IdentityList) {
	for _, node := range nodes {
		for i, other := range nodes {
			if node == other {
				continue
			}
			otherPInfo, err := utils.PeerAddressInfo(*ids[i])
			require.NoError(t, err)
			node.Host().Peerstore().AddAddrs(otherPInfo.ID, otherPInfo.Addrs, peerstore.AddressTTL)
		}
	}
}

// EnsureConnected ensures that the given nodes are connected to each other.
// It fails the test if any of the nodes is not connected to any other node.
func EnsureConnected(t *testing.T, ctx context.Context, nodes []p2p.LibP2PNode) {
	for _, node := range nodes {
		for _, other := range nodes {
			if node == other {
				continue
			}
			require.NoError(t, node.Host().Connect(ctx, other.Host().Peerstore().PeerInfo(other.Host().ID())))
		}
	}
}

// EnsureNotConnected ensures that no connection exists from "from" nodes to "to" nodes.
func EnsureNotConnected(t *testing.T, ctx context.Context, from []p2p.LibP2PNode, to []p2p.LibP2PNode) {
	for _, node := range from {
		for _, other := range to {
			if node == other {
				require.Fail(t, "overlapping nodes in from and to lists")
			}
			require.Error(t, node.Host().Connect(ctx, other.Host().Peerstore().PeerInfo(other.Host().ID())))
		}
	}
}

// EnsureNotConnectedBetweenGroups ensures no connection exists between the given groups of nodes.
func EnsureNotConnectedBetweenGroups(t *testing.T, ctx context.Context, groupA []p2p.LibP2PNode, groupB []p2p.LibP2PNode) {
	// ensure no connection from group A to group B
	EnsureNotConnected(t, ctx, groupA, groupB)
	// ensure no connection from group B to group A
	EnsureNotConnected(t, ctx, groupB, groupA)
}

// EnsurePubsubMessageExchange ensures that the given nodes exchange the given message on the given channel through pubsub.
func EnsurePubsubMessageExchange(t *testing.T, ctx context.Context, nodes []p2p.LibP2PNode, messageFactory func() (interface{}, channels.Topic)) {
	_, topic := messageFactory()

	subs := make([]p2p.Subscription, len(nodes))
	slashingViolationsConsumer := unittest.NetworkSlashingViolationsConsumer(unittest.Logger(), metrics.NewNoopCollector())
	for i, node := range nodes {
		ps, err := node.Subscribe(
			topic,
			validator.TopicValidator(
				unittest.Logger(),
				unittest.NetworkCodec(),
				slashingViolationsConsumer,
				unittest.AllowAllPeerFilter()))
		require.NoError(t, err)
		subs[i] = ps
	}

	// let subscriptions propagate
	time.Sleep(1 * time.Second)

	channel, ok := channels.ChannelFromTopic(topic)
	require.True(t, ok)

	for _, node := range nodes {
		// creates a unique message to be published by the node
		msg, _ := messageFactory()
		data := MustEncodeEvent(t, msg, channel)
		require.NoError(t, node.Publish(ctx, topic, data))

		// wait for the message to be received by all nodes
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		SubsMustReceiveMessage(t, ctx, data, subs)
		cancel()
	}
}

// EnsureNoPubsubMessageExchange ensures that the no pubsub message is exchanged "from" the given nodes "to" the given nodes.
func EnsureNoPubsubMessageExchange(t *testing.T, ctx context.Context, from []p2p.LibP2PNode, to []p2p.LibP2PNode, messageFactory func() (interface{}, channels.Topic)) {
	_, topic := messageFactory()

	subs := make([]p2p.Subscription, len(to))
	svc := unittest.NetworkSlashingViolationsConsumer(unittest.Logger(), metrics.NewNoopCollector())
	tv := validator.TopicValidator(
		unittest.Logger(),
		unittest.NetworkCodec(),
		svc,
		unittest.AllowAllPeerFilter())
	var err error
	for _, node := range from {
		_, err = node.Subscribe(topic, tv)
		require.NoError(t, err)
	}

	for i, node := range to {
		s, err := node.Subscribe(topic, tv)
		require.NoError(t, err)
		subs[i] = s
	}

	// let subscriptions propagate
	time.Sleep(1 * time.Second)

	for _, node := range from {
		// creates a unique message to be published by the node.
		msg, _ := messageFactory()
		channel, ok := channels.ChannelFromTopic(topic)
		require.True(t, ok)
		data := MustEncodeEvent(t, msg, channel)

		// ensure the message is NOT received by any of the nodes.
		require.NoError(t, node.Publish(ctx, topic, data))
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		SubsMustNeverReceiveAnyMessage(t, ctx, subs)
		cancel()
	}
}

// EnsureNoPubsubExchangeBetweenGroups ensures that no pubsub message is exchanged between the given groups of nodes.
func EnsureNoPubsubExchangeBetweenGroups(t *testing.T, ctx context.Context, groupA []p2p.LibP2PNode, groupB []p2p.LibP2PNode, messageFactory func() (interface{}, channels.Topic)) {
	// ensure no message exchange from group A to group B
	EnsureNoPubsubMessageExchange(t, ctx, groupA, groupB, messageFactory)
	// ensure no message exchange from group B to group A
	EnsureNoPubsubMessageExchange(t, ctx, groupB, groupA, messageFactory)
}

// EnsureMessageExchangeOverUnicast ensures that the given nodes exchange arbitrary messages on through unicasting (i.e., stream creation).
// It fails the test if any of the nodes does not receive the message from the other nodes.
// The "inbounds" parameter specifies the inbound channel of the nodes on which the messages are received.
// The "messageFactory" parameter specifies the function that creates unique messages to be sent.
func EnsureMessageExchangeOverUnicast(t *testing.T, ctx context.Context, nodes []p2p.LibP2PNode, inbounds []chan string, messageFactory func() string) {
	for _, this := range nodes {
		msg := messageFactory()

		// send the message to all other nodes
		for _, other := range nodes {
			if this == other {
				continue
			}
			s, err := this.CreateStream(ctx, other.Host().ID())
			require.NoError(t, err)
			rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))
			_, err = rw.WriteString(msg)
			require.NoError(t, err)

			// Flush the stream
			require.NoError(t, rw.Flush())
		}

		// wait for the message to be received by all other nodes
		for i, other := range nodes {
			if this == other {
				continue
			}

			select {
			case rcv := <-inbounds[i]:
				require.Equal(t, msg, rcv)
			case <-time.After(3 * time.Second):
				require.Fail(t, fmt.Sprintf("did not receive message from node %d", i))
			}
		}
	}
}

// EnsureNoStreamCreationBetweenGroups ensures that no stream is created between the given groups of nodes.
func EnsureNoStreamCreationBetweenGroups(t *testing.T, ctx context.Context, groupA []p2p.LibP2PNode, groupB []p2p.LibP2PNode, errorCheckers ...func(*testing.T, error)) {
	// no stream from groupA -> groupB
	EnsureNoStreamCreation(t, ctx, groupA, groupB, errorCheckers...)
	// no stream from groupB -> groupA
	EnsureNoStreamCreation(t, ctx, groupB, groupA, errorCheckers...)
}

// EnsureNoStreamCreation ensures that no stream is created "from" the given nodes "to" the given nodes.
func EnsureNoStreamCreation(t *testing.T, ctx context.Context, from []p2p.LibP2PNode, to []p2p.LibP2PNode, errorCheckers ...func(*testing.T, error)) {
	for _, this := range from {
		for _, other := range to {
			if this == other {
				// should not happen, unless the test is misconfigured.
				require.Fail(t, "node is in both from and to lists")
			}
			// stream creation should fail
			_, err := this.CreateStream(ctx, other.Host().ID())
			require.Error(t, err)
			require.True(t, flownet.IsPeerUnreachableError(err))

			// runs the error checkers if any.
			for _, check := range errorCheckers {
				check(t, err)
			}
		}
	}
}

// EnsureStreamCreation ensures that a stream is created between each of the  "from" nodes to each of the "to" nodes.
func EnsureStreamCreation(t *testing.T, ctx context.Context, from []p2p.LibP2PNode, to []p2p.LibP2PNode) {
	for _, this := range from {
		for _, other := range to {
			if this == other {
				// should not happen, unless the test is misconfigured.
				require.Fail(t, "node is in both from and to lists")
			}
			// stream creation should pass without error
			s, err := this.CreateStream(ctx, other.Host().ID())
			require.NoError(t, err)
			require.NotNil(t, s)
		}
	}
}

// EnsureStreamCreationInBothDirections ensure that between each pair of nodes in the given list, a stream is created in both directions.
func EnsureStreamCreationInBothDirections(t *testing.T, ctx context.Context, nodes []p2p.LibP2PNode) {
	for _, this := range nodes {
		for _, other := range nodes {
			if this == other {
				continue
			}
			// stream creation should pass without error
			s, err := this.CreateStream(ctx, other.Host().ID())
			require.NoError(t, err)
			require.NotNil(t, s)
		}
	}
}

// LongStringMessageFactoryFixture returns a function that creates a long unique string message.
func LongStringMessageFactoryFixture(t *testing.T) func() string {
	return func() string {
		msg := "this is an intentionally long MESSAGE to be bigger than buffer size of most of stream compressors"
		require.Greater(t, len(msg), 10, "we must stress test with longer than 10 bytes messages")
		return fmt.Sprintf("%s %d \n", msg, time.Now().UnixNano()) // add timestamp to make sure we don't send the same message twice
	}
}
