package fvm

import (
	"context"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/handler"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
)

var _ runtime.Interface = &ScriptEnv{}
var _ Environment = &ScriptEnv{}

// ScriptEnv is a read-only mostly used for executing scripts.
type ScriptEnv struct {
	commonEnv
}

func NewScriptEnvironment(
	reqContext context.Context,
	fvmContext Context,
	vm *VirtualMachine,
	sth *state.StateHolder,
	txnProgs *programs.TransactionPrograms,
) *ScriptEnv {

	tracer := environment.NewTracer(fvmContext.Tracer, nil, fvmContext.ExtensiveTracing)
	meter := environment.NewCancellableMeter(reqContext, sth)

	env := &ScriptEnv{
		commonEnv: newCommonEnv(
			fvmContext,
			vm,
			sth,
			txnProgs,
			tracer,
			meter,
		),
	}

	env.TransactionInfo = environment.NoTransactionInfo{}
	env.EventEmitter = environment.NoEventEmitter{}
	env.SystemContracts.SetEnvironment(env)

	// TODO(patrick): remove this hack
	env.accountKeys = handler.NewAccountKeyHandler(env.accounts)
	env.fullEnv = env

	return env
}

// Block Environment Functions

func (e *ScriptEnv) CreateAccount(_ runtime.Address) (address runtime.Address, err error) {
	return runtime.Address{}, errors.NewOperationNotSupportedError("CreateAccount")
}

func (e *ScriptEnv) AddEncodedAccountKey(_ runtime.Address, _ []byte) error {
	return errors.NewOperationNotSupportedError("AddEncodedAccountKey")
}

func (e *ScriptEnv) RevokeEncodedAccountKey(_ runtime.Address, _ int) (publicKey []byte, err error) {
	return nil, errors.NewOperationNotSupportedError("RevokeEncodedAccountKey")
}

func (e *ScriptEnv) AddAccountKey(_ runtime.Address, _ *runtime.PublicKey, _ runtime.HashAlgorithm, _ int) (*runtime.AccountKey, error) {
	return nil, errors.NewOperationNotSupportedError("AddAccountKey")
}

func (e *ScriptEnv) RevokeAccountKey(_ runtime.Address, _ int) (*runtime.AccountKey, error) {
	return nil, errors.NewOperationNotSupportedError("RevokeAccountKey")
}

func (e *ScriptEnv) UpdateAccountContractCode(_ runtime.Address, _ string, _ []byte) (err error) {
	return errors.NewOperationNotSupportedError("UpdateAccountContractCode")
}

func (e *ScriptEnv) RemoveAccountContractCode(_ runtime.Address, _ string) (err error) {
	return errors.NewOperationNotSupportedError("RemoveAccountContractCode")
}

func (e *ScriptEnv) SetAccountFrozen(address common.Address, frozen bool) error {
	return errors.NewOperationNotSupportedError("SetAccountFrozen")
}
