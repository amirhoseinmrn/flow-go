package meter

import (
	"math"

	"github.com/onflow/cadence/runtime/common"
)

type MeterControl struct {
	enforceLimits bool
}

// TODO(patrick): expose this to meter api
func (control *MeterControl) EnableAllLimitEnforcements() {
	control.enforceLimits = true
}

// TODO(patrick): expose this to meter api
func (control *MeterControl) DisableAllLimitEnforcements() {
	control.enforceLimits = false
}

type ControllableMeter struct {
	// NOTE: Once a child meter is creating, changing enforcement behavior in
	// the parent meter wouldn't impact the child's enforcement behavior, and
	// vice versa.
	MeterControl

	// Observer is an unlimited meter used to keep track of all, including
	// un-enforced, usage.
	observer *WeightedMeter

	// Enforcer handles limit enforcement nad only keeps track of enforcable
	// usage.
	enforcer *WeightedMeter
}

// TODO(patrick):
// 1. rename this to NewMeter and change return type to Meter
// 2. rename weighted meter's NewMeter to NewWeightedMeter
// 3. make effort weights, memory weights, and payerIsServiceAccount
//    non-optional arguments
func NewControllableMeter(
	computationLimit uint,
	memoryLimit uint,
	options ...WeightedMeterOptions,
) *ControllableMeter {
	return &ControllableMeter{
		MeterControl: MeterControl{
			enforceLimits:         true,
			payerIsServiceAccount: false,
		},
		observer: newWeightedMeter(
			math.MaxUint,
			math.MaxUint,
			options...,
		),
		enforcer: newWeightedMeter(
			computationLimit,
			memoryLimit,
			options...,
		),
	}
}

func (meter *ControllableMeter) NewChild() Meter {
	return &ControllableMeter{
		MeterControl: meter.MeterControl,
		observer:     meter.observer.newChild(),
		enforcer:     meter.enforcer.newChild(),
	}
}

func (meter *ControllableMeter) MergeMeter(
	child Meter,
	enforceLimits bool,
) error {
	err := meter.observer.MergeMeter(child.Observer(), false)
	if err != nil {
		return err
	}

	if meter.enforceLimits {
		return meter.enforcer.MergeMeter(child.Enforcer(), enforceLimits)
	}
	return nil
}

func (meter *ControllableMeter) Observer() *WeightedMeter {
	return meter.observer
}

func (meter *ControllableMeter) Enforcer() *WeightedMeter {
	return meter.enforcer
}

func (meter *ControllableMeter) MeterComputation(
	kind common.ComputationKind,
	intensity uint,
) error {
	err := meter.observer.MeterComputation(kind, intensity)
	if err != nil {
		return err
	}

	if meter.enforceLimits {
		return meter.enforcer.MeterComputation(kind, intensity)
	}

	return nil
}

func (meter *ControllableMeter) ObservedComputationIntensities() MeteredComputationIntensities {
	return meter.observer.ObservedComputationIntensities()
}

func (meter *ControllableMeter) EnforcedComputationIntensities() MeteredComputationIntensities {
	return meter.enforcer.EnforcedComputationIntensities()
}

func (meter *ControllableMeter) TotalObservedComputationUsed() uint {
	return meter.observer.TotalObservedComputationUsed()
}

func (meter *ControllableMeter) TotalEnforcedComputationUsed() uint {
	return meter.enforcer.TotalEnforcedComputationUsed()
}

func (meter *ControllableMeter) TotalEnforcedComputationLimit() uint {
	return meter.enforcer.TotalEnforcedComputationLimit()
}

func (meter *ControllableMeter) MeterMemory(
	kind common.MemoryKind,
	intensity uint,
) error {
	err := meter.observer.MeterMemory(kind, intensity)
	if err != nil {
		return err
	}

	if !meter.payerIsServiceAccount && meter.enforceLimits {
		return meter.enforcer.MeterMemory(kind, intensity)
	}

	return nil
}

func (meter *ControllableMeter) ObservedMemoryIntensities() MeteredMemoryIntensities {
	return meter.observer.ObservedMemoryIntensities()
}

func (meter *ControllableMeter) EnforcedMemoryIntensities() MeteredMemoryIntensities {
	return meter.enforcer.EnforcedMemoryIntensities()
}

func (meter *ControllableMeter) TotalObservedMemoryEstimate() uint {
	return meter.observer.TotalObservedMemoryEstimate()
}

func (meter *ControllableMeter) TotalEnforcedMemoryEstimate() uint {
	return meter.enforcer.TotalEnforcedMemoryEstimate()
}

func (meter *ControllableMeter) TotalEnforcedMemoryLimit() uint {
	return meter.enforcer.TotalEnforcedMemoryLimit()
}

func (meter *ControllableMeter) SetComputationWeights(
	weights ExecutionEffortWeights,
) {
	meter.observer.SetComputationWeights(weights)
	meter.enforcer.SetComputationWeights(weights)
}

func (meter *ControllableMeter) SetMemoryWeights(
	weights ExecutionMemoryWeights,
) {
	meter.observer.SetMemoryWeights(weights)
	meter.enforcer.SetMemoryWeights(weights)
}

func (meter *ControllableMeter) SetTotalMemoryLimit(limit uint64) {
	meter.enforcer.SetTotalMemoryLimit(limit)
}
