package p2pnode

import (
	"context"
	"fmt"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/utils/logging"
)

// GossipSubAdapter is a wrapper around the libp2p GossipSub implementation
// that implements the PubSubAdapter interface for the Flow network.
type GossipSubAdapter struct {
	gossipSub *pubsub.PubSub
	logger    zerolog.Logger
}

var _ p2p.PubSubAdapter = (*GossipSubAdapter)(nil)

func NewGossipSubAdapter(ctx context.Context, logger zerolog.Logger, h host.Host, cfg p2p.PubSubAdapterConfig) (p2p.PubSubAdapter, error) {
	gossipSubConfig, ok := cfg.(*GossipSubAdapterConfig)
	if !ok {
		return nil, fmt.Errorf("invalid gossipsub config type: %T", cfg)
	}

	gossipSub, err := pubsub.NewGossipSub(ctx, h, gossipSubConfig.Build()...)
	if err != nil {
		return nil, err
	}
	return &GossipSubAdapter{
		gossipSub: gossipSub,
		logger:    logger,
	}, nil
}

func (g *GossipSubAdapter) RegisterTopicValidator(topic string, topicValidator p2p.TopicValidatorFunc) error {
	// wrap the topic validator function into a libp2p topic validator function.
	var v pubsub.ValidatorEx = func(ctx context.Context, from peer.ID, message *pubsub.Message) pubsub.ValidationResult {
		switch result := topicValidator(ctx, from, message); result {
		case p2p.ValidationAccept:
			return pubsub.ValidationAccept
		case p2p.ValidationIgnore:
			return pubsub.ValidationIgnore
		case p2p.ValidationReject:
			return pubsub.ValidationReject
		default:
			// should never happen, indicates a bug in the topic validator
			g.logger.Fatal().Msgf("invalid validation result: %v", result)
		}
		// should never happen, indicates a bug in the topic validator, but we need to return something
		g.logger.Warn().
			Bool(logging.KeySuspicious, true).
			Msg("invalid validation result, returning reject")
		return pubsub.ValidationReject
	}

	return g.gossipSub.RegisterTopicValidator(topic, v, pubsub.WithValidatorInline(true))
}

func (g *GossipSubAdapter) UnregisterTopicValidator(topic string) error {
	return g.gossipSub.UnregisterTopicValidator(topic)
}

func (g *GossipSubAdapter) Join(topic string) (p2p.Topic, error) {
	t, err := g.gossipSub.Join(topic)
	if err != nil {
		return nil, fmt.Errorf("could not join topic %s: %w", topic, err)
	}
	return NewGossipSubTopic(t), nil
}

func (g *GossipSubAdapter) GetTopics() []string {
	return g.gossipSub.GetTopics()
}

func (g *GossipSubAdapter) ListPeers(topic string) []peer.ID {
	return g.gossipSub.ListPeers(topic)
}
