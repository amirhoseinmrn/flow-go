package environment

import (
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/sema"
	"go.opentelemetry.io/otel/attribute"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

// ContractFunctionSpec specify all the information, except the function's
// address and arguments, needed to invoke the contract function.
type ContractFunctionSpec struct {
	AddressFromChain func(flow.Chain) flow.Address
	LocationName     string
	FunctionName     string
	ArgumentTypes    []sema.Type
}

// SystemContracts provides methods for invoking system contract functions as
// service account.
type SystemContracts struct {
	chain flow.Chain

	tracer  *Tracer
	logger  *ProgramLogger
	runtime *Runtime
}

func NewSystemContracts(
	chain flow.Chain,
	tracer *Tracer,
	logger *ProgramLogger,
	runtime *Runtime,
) *SystemContracts {
	return &SystemContracts{
		chain:   chain,
		tracer:  tracer,
		logger:  logger,
		runtime: runtime,
	}
}

func (sys *SystemContracts) Invoke(
	spec ContractFunctionSpec,
	arguments []cadence.Value,
) (
	cadence.Value,
	error,
) {
	contractLocation := common.AddressLocation{
		Address: common.Address(spec.AddressFromChain(sys.chain)),
		Name:    spec.LocationName,
	}

	span := sys.tracer.StartSpanFromRoot(trace.FVMInvokeContractFunction)
	span.SetAttributes(
		attribute.String(
			"transaction.ContractFunctionCall",
			contractLocation.String()+"."+spec.FunctionName))
	defer span.End()

	runtime := sys.runtime.BorrowCadenceRuntime()
	defer sys.runtime.ReturnCadenceRuntime(runtime)

	value, err := runtime.InvokeContractFunction(
		contractLocation,
		spec.FunctionName,
		arguments,
		spec.ArgumentTypes,
	)
	if err != nil {
		sys.logger.Logger().
			Info().
			Err(err).
			Str("contract", contractLocation.String()).
			Str("function", spec.FunctionName).
			Msg("Contract function call executed with error")
	}
	return value, err
}

func FlowFeesAddress(chain flow.Chain) flow.Address {
	address, _ := chain.AddressAtIndex(FlowFeesAccountIndex)
	return address
}

func ServiceAddress(chain flow.Chain) flow.Address {
	return chain.ServiceAddress()
}

var verifyPayersBalanceForTransactionExecutionSpec = ContractFunctionSpec{
	AddressFromChain: FlowFeesAddress,
	LocationName:     systemcontracts.ContractNameFlowFees,
	FunctionName:     systemcontracts.ContractServiceAccountFunction_verifyPayersBalanceForTransactionExecution,
	ArgumentTypes: []sema.Type{
		sema.AuthAccountType,
		sema.UInt64Type,
		sema.UInt64Type,
	},
}

// CheckPayerBalanceAndGetMaxTxFees executes the verifyPayersBalanceForTransactionExecution
// on the FlowFees account.
// It checks whether the given payer has enough balance to cover inclusion fee and max execution
// fee.
// It returns (maxTransactionFee, ErrCodeInsufficientPayerBalance) if the payer doesn't have enough balance
// It returns (maxTransactionFee, nil) if the payer has enough balance
func (sys *SystemContracts) CheckPayerBalanceAndGetMaxTxFees(
	payer flow.Address,
	inclusionEffort uint64,
	maxExecutionEffort uint64,
) (cadence.Value, error) {
	return sys.Invoke(
		verifyPayersBalanceForTransactionExecutionSpec,
		[]cadence.Value{
			cadence.BytesToAddress(payer.Bytes()),
			cadence.UFix64(inclusionEffort),
			cadence.UFix64(maxExecutionEffort),
		},
	)
}

var deductTransactionFeeSpec = ContractFunctionSpec{
	AddressFromChain: FlowFeesAddress,
	LocationName:     systemcontracts.ContractNameFlowFees,
	FunctionName:     systemcontracts.ContractServiceAccountFunction_deductTransactionFee,
	ArgumentTypes: []sema.Type{
		sema.AuthAccountType,
		sema.UInt64Type,
		sema.UInt64Type,
	},
}

// DeductTransactionFees executes the fee deduction function
// on the FlowFees account.
func (sys *SystemContracts) DeductTransactionFees(
	payer flow.Address,
	inclusionEffort uint64,
	executionEffort uint64,
) (cadence.Value, error) {
	return sys.Invoke(
		deductTransactionFeeSpec,
		[]cadence.Value{
			cadence.BytesToAddress(payer.Bytes()),
			cadence.UFix64(inclusionEffort),
			cadence.UFix64(executionEffort),
		},
	)
}

// uses `FlowServiceAccount.setupNewAccount` from https://github.com/onflow/flow-core-contracts/blob/master/contracts/FlowServiceAccount.cdc
var setupNewAccountSpec = ContractFunctionSpec{
	AddressFromChain: ServiceAddress,
	LocationName:     systemcontracts.ContractServiceAccount,
	FunctionName:     systemcontracts.ContractServiceAccountFunction_setupNewAccount,
	ArgumentTypes: []sema.Type{
		sema.AuthAccountType,
		sema.AuthAccountType,
	},
}

// SetupNewAccount executes the new account setup contract on the service
// account.
func (sys *SystemContracts) SetupNewAccount(
	flowAddress flow.Address,
	payer common.Address,
) (cadence.Value, error) {
	return sys.Invoke(
		setupNewAccountSpec,
		[]cadence.Value{
			cadence.BytesToAddress(flowAddress.Bytes()),
			cadence.BytesToAddress(payer.Bytes()),
		},
	)
}

var accountAvailableBalanceSpec = ContractFunctionSpec{
	AddressFromChain: ServiceAddress,
	LocationName:     systemcontracts.ContractStorageFees,
	FunctionName:     systemcontracts.ContractStorageFeesFunction_defaultTokenAvailableBalance,
	ArgumentTypes: []sema.Type{
		&sema.AddressType{},
	},
}

// AccountAvailableBalance executes the get available balance contract on the
// storage fees contract.
func (sys *SystemContracts) AccountAvailableBalance(
	address common.Address,
) (cadence.Value, error) {
	return sys.Invoke(
		accountAvailableBalanceSpec,
		[]cadence.Value{
			cadence.BytesToAddress(address.Bytes()),
		},
	)
}

var accountBalanceInvocationSpec = ContractFunctionSpec{
	AddressFromChain: ServiceAddress,
	LocationName:     systemcontracts.ContractServiceAccount,
	FunctionName:     systemcontracts.ContractServiceAccountFunction_defaultTokenBalance,
	ArgumentTypes: []sema.Type{
		sema.PublicAccountType,
	},
}

// AccountBalance executes the get available balance contract on the service
// account.
func (sys *SystemContracts) AccountBalance(
	address common.Address,
) (cadence.Value, error) {
	return sys.Invoke(
		accountBalanceInvocationSpec,
		[]cadence.Value{
			cadence.BytesToAddress(address.Bytes()),
		},
	)
}

var accountStorageCapacitySpec = ContractFunctionSpec{
	AddressFromChain: ServiceAddress,
	LocationName:     systemcontracts.ContractStorageFees,
	FunctionName:     systemcontracts.ContractStorageFeesFunction_calculateAccountCapacity,
	ArgumentTypes: []sema.Type{
		&sema.AddressType{},
	},
}

// AccountStorageCapacity executes the get storage capacity contract on the
// service account.
func (sys *SystemContracts) AccountStorageCapacity(
	address common.Address,
) (cadence.Value, error) {
	return sys.Invoke(
		accountStorageCapacitySpec,
		[]cadence.Value{
			cadence.BytesToAddress(address.Bytes()),
		},
	)
}

// AccountsStorageCapacity gets storage capacity for multiple accounts at once.
func (sys *SystemContracts) AccountsStorageCapacity(
	addresses []common.Address,
	payer common.Address,
	maxTxFees uint64,
) (cadence.Value, error) {
	arrayValues := make([]cadence.Value, len(addresses))
	for i, address := range addresses {
		arrayValues[i] = cadence.BytesToAddress(address.Bytes())
	}

	return sys.Invoke(
		ContractFunctionSpec{
			AddressFromChain: ServiceAddress,
			LocationName:     systemcontracts.ContractStorageFees,
			FunctionName:     systemcontracts.ContractStorageFeesFunction_getAccountsCapacityForTransactionStorageCheck,
			ArgumentTypes: []sema.Type{
				sema.NewConstantSizedType(
					nil,
					&sema.AddressType{},
					int64(len(arrayValues)),
				),
				&sema.AddressType{},
				sema.UFix64Type,
			},
		},
		[]cadence.Value{
			cadence.NewArray(arrayValues),
			cadence.BytesToAddress(payer.Bytes()),
			cadence.UFix64(maxTxFees),
		},
	)
}

var useContractAuditVoucherSpec = ContractFunctionSpec{
	AddressFromChain: ServiceAddress,
	LocationName:     systemcontracts.ContractDeploymentAudits,
	FunctionName:     systemcontracts.ContractDeploymentAuditsFunction_useVoucherForDeploy,
	ArgumentTypes: []sema.Type{
		&sema.AddressType{},
		sema.StringType,
	},
}

// UseContractAuditVoucher executes the use a contract deployment audit voucher
// contract.
func (sys *SystemContracts) UseContractAuditVoucher(
	address common.Address,
	code string,
) (bool, error) {
	resultCdc, err := sys.Invoke(
		useContractAuditVoucherSpec,
		[]cadence.Value{
			cadence.BytesToAddress(address.Bytes()),
			cadence.String(code),
		},
	)
	if err != nil {
		return false, err
	}
	result := resultCdc.(cadence.Bool).ToGoValue().(bool)
	return result, nil
}
