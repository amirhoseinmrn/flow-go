package validator

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/message"
)

var _ network.MessageValidator = &OriginValidator{}

// OriginValidator returns true if the sender of the message is among the set of identifiers
// returned by the given IdentifierProvider
type OriginValidator struct {
	idProvider module.IdentifierProvider
}

func NewOriginValidator(provider module.IdentifierProvider) network.MessageValidator {
	return &OriginValidator{provider}
}

func (v OriginValidator) Validate(msg message.Message) bool {
	return v.idProvider.Identifiers().Contains(flow.HashToID(msg.OriginID))
}
