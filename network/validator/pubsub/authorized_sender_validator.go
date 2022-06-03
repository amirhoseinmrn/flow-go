package validator

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/rs/zerolog"

	channels "github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/network"

	"github.com/onflow/flow-go/model/flow"
	cborcodec "github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/network/message"
)

const (
	ErrReceiveOnly         = "rejecting message: sender is flagged as receive only and not authorized to send messages on this channel"
	ErrUnauthorized        = "rejecting message: sender is not authorized to send this message type"
	ErrSenderEjected       = "rejecting message: sender is an ejected node"
	ErrInvalidMsgOnChannel = "invalid message type being sent on channel"
	ErrIdentityUnverified  = "could not verify identity of sender"
)

type getIdentityFunc func(peer.ID) (*flow.Identity, bool)

// AuthorizedSenderValidator using the getIdentity func will check if the role of the sender
// is part of the authorized roles list for the channel being communicated on. A node is considered
// to be authorized to send a message if all of the following are true.
// 1. The node is authorized.
// 2. The message type is a known message type (can be decoded with cbor codec).
// 3. The authorized roles list for the channel contains the senders role.
// 4. The node is not ejected
func AuthorizedSenderValidator(log zerolog.Logger, channel network.Channel, getIdentity getIdentityFunc) MessageValidator {
	log = log.With().
		Str("component", "authorized_sender_validator").
		Str("network_channel", channel.String()).
		Logger()

	// use cbor codec to add explicit dependency on cbor encoded messages adding the message type
	// to the first byte of the message payload, this adds safety against changing codec without updating this validator
	codec := cborcodec.NewCodec()

	return func(ctx context.Context, from peer.ID, msg *message.Message) pubsub.ValidationResult {
		identity, ok := getIdentity(from)
		if !ok {
			log.Warn().Str("peer_id", from.String()).Msg(ErrIdentityUnverified)
			return pubsub.ValidationReject
		}

		if identity.Ejected {
			log.Warn().
				Str("peer_id", from.String()).
				Str("role", identity.Role.String()).
				Str("node_id", identity.NodeID.String()).
				Msg(ErrSenderEjected)
			return pubsub.ValidationReject
		}

		// attempt to decode the flow message type from encoded payload
		code, what, err := codec.DecodeMsgType(msg.Payload)
		if err != nil {
			log.Warn().
				Str("peer_id", from.String()).
				Str("role", identity.Role.String()).
				Str("node_id", identity.NodeID.String()).
				Msg(err.Error())
			return pubsub.ValidationReject
		}

		if err := isAuthorizedSender(identity, channel, code); err != nil {
			log.Warn().
				Str("peer_id", from.String()).
				Str("role", identity.Role.String()).
				Str("node_id", identity.NodeID.String()).
				Str("message_type", what).
				Msg(err.Error())

			return pubsub.ValidationReject
		}

		return pubsub.ValidationAccept
	}
}

// isAuthorizedSender checks if node is an authorized role and is not ejected.
func isAuthorizedSender(identity *flow.Identity, channel network.Channel, code uint8) error {

	// cluster channels have a dynamic channel name
	if code == cborcodec.CodeClusterBlockProposal || code == cborcodec.CodeClusterBlockVote || code == cborcodec.CodeClusterBlockResponse {
		channels.ClusterChannelRoles(channel)
	}

	// get authorized roles list
	authorizedRoles, receiveOnlyRoles, err := getRoles(channel, code)
	if err != nil {
		return err
	}

	if !authorizedRoles.Contains(identity.Role) {
		return fmt.Errorf(ErrUnauthorized)
	}

	if receiveOnlyRoles.Contains(identity.Role) {
		// slashable offense: nodes should not send messages on channels they only need to receive on
		return fmt.Errorf(ErrReceiveOnly)
	}

	return nil
}

// getRoles returns list of authorized roles for the channel associated with the message code provided
func getRoles(channel network.Channel, msgTypeCode uint8) (flow.RoleList, flow.RoleList, error) {
	// echo messages can be sent by anyone
	if msgTypeCode == cborcodec.CodeEcho {
		return flow.Roles(), flow.RoleList{}, nil
	}

	// get message type codes for all messages communicated on the channel
	codes, ok := getCodes(channel)
	if !ok {
		return nil, nil, fmt.Errorf("could not get message codes for unknown channel: %s", channel)
	}

	// check if message type code is in list of codes corresponding to channel
	if !containsCode(codes, msgTypeCode) {
		return nil, nil, fmt.Errorf(ErrInvalidMsgOnChannel)
	}

	// get authorized list of roles for channel
	authorizedRoles, ok := channels.RolesByChannel(channel)
	if !ok {
		return nil, nil, fmt.Errorf("could not get roles for channel")
	}

	// get list of receive only roles for channel
	receiveOnlyRoles := channels.ReceiveOnlyRolesByChannel(channel)

	return authorizedRoles, receiveOnlyRoles, nil
}

// getCodes checks if channel is a cluster prefixed channel before returning msg codes
func getCodes(channel network.Channel) ([]uint8, bool) {
	if prefix, ok := channels.ClusterChannelPrefix(channel); ok {
		codes, ok := cborcodec.ChannelToMsgCodes[network.Channel(prefix)]
		return codes, ok
	}

	codes, ok := cborcodec.ChannelToMsgCodes[channel]
	return codes, ok
}

func containsCode(codes []uint8, code uint8) bool {
	for _, c := range codes {
		if c == code {
			return true
		}
	}

	return false
}
