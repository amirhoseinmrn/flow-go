package middleware

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/p2p"
	validator "github.com/onflow/flow-go/network/validator/pubsub"
	"github.com/onflow/flow-go/utils/logging"
)

// readSubscriptionCB the callback called when a new message is received on the read subscription
type readSubscriptionCB func(msg *message.Message, decodedMsgPayload interface{}, peerID peer.ID)

// readSubscription reads the messages coming in on the subscription and calls the given callback until
// the context of the subscription is cancelled. Additionally, it reports metrics
type readSubscription struct {
	ctx      context.Context
	log      zerolog.Logger
	sub      p2p.Subscription
	metrics  module.NetworkMetrics
	callback readSubscriptionCB
}

// newReadSubscription reads the messages coming in on the subscription
func newReadSubscription(ctx context.Context, sub p2p.Subscription, callback readSubscriptionCB, log zerolog.Logger, metrics module.NetworkMetrics) *readSubscription {

	log = log.With().
		Str("channel", sub.Topic()).
		Logger()

	r := readSubscription{
		ctx:      ctx,
		log:      log,
		sub:      sub,
		callback: callback,
		metrics:  metrics,
	}

	return &r
}

// receiveLoop must be run in a goroutine. It continuously receives
// messages for the topic and calls the callback synchronously
func (r *readSubscription) receiveLoop(wg *sync.WaitGroup) {

	defer wg.Done()
	defer r.log.Debug().Msg("exiting receive routine")

	for {

		// read the next message from libp2p's subscription (blocking call)
		rawMsg, err := r.sub.Next(r.ctx)

		if err != nil {

			// middleware may have cancelled the context
			if err == context.Canceled {
				return
			}

			// subscription may have just been cancelled if node is being stopped or the topic has been unsubscribed,
			// don't log error in that case
			// (https://github.com/ipsn/go-ipfs/blob/master/gxlibs/github.com/libp2p/go-libp2p-pubsub/pubsub.go#L435)
			if strings.Contains(err.Error(), "subscription cancelled") {
				return
			}

			// log any other error
			r.log.Err(err).Msg("failed to read subscription message")

			return
		}

		validatorData, ok := rawMsg.ValidatorData.(validator.TopicValidatorData)
		if !ok {
			r.log.Error().
				Str("raw_msg", rawMsg.String()).
				Bool(logging.KeySuspicious, true).
				Str("received_validator_data_type", fmt.Sprintf("%T", rawMsg.ValidatorData)).
				Msg("[BUG] validator data missing!")
			return
		}

		msg := validatorData.Message

		// log metrics
		r.metrics.NetworkMessageReceived(msg.Size(), msg.ChannelID, msg.Type)

		// call the callback
		r.callback(msg, validatorData.DecodedMsgPayload, validatorData.From)
	}
}
