package meter_test

import (
	"math"
	"testing"

	"github.com/onflow/cadence/runtime/common"
	"github.com/stretchr/testify/require"

	meterPkg "github.com/onflow/flow-go/fvm/meter"
)

func checkControllableMeter(
	t *testing.T,
	meter meterPkg.Meter,
	expectedObservedComp uint,
	expectedEnforcedComp uint,
	expectedObservedMem uint,
	expectedEnforcedMem uint,
) {
	require.Equal(
		t,
		expectedObservedComp,
		meter.TotalObservedComputationUsed())

	require.Equal(
		t,
		expectedObservedComp,
		meter.Observer().TotalObservedComputationUsed(),
	)
	require.Equal(
		t,
		expectedEnforcedComp,
		meter.TotalEnforcedComputationUsed(),
	)
	require.Equal(
		t,
		expectedEnforcedComp,
		meter.Enforcer().TotalEnforcedComputationUsed(),
	)
	require.Equal(
		t,
		meter.Observer().ObservedComputationIntensities(),
		meter.ObservedComputationIntensities(),
	)
	require.Equal(
		t,
		meter.Enforcer().EnforcedComputationIntensities(),
		meter.EnforcedComputationIntensities(),
	)

	require.Equal(
		t,
		expectedObservedMem,
		meter.TotalObservedMemoryEstimate(),
	)
	require.Equal(
		t,
		expectedObservedMem,
		meter.Observer().TotalObservedMemoryEstimate(),
	)
	require.Equal(
		t,
		expectedEnforcedMem,
		meter.TotalEnforcedMemoryEstimate(),
	)
	require.Equal(
		t,
		expectedEnforcedMem,
		meter.Enforcer().TotalEnforcedMemoryEstimate(),
	)
	require.Equal(
		t,
		meter.Observer().ObservedMemoryIntensities(),
		meter.ObservedMemoryIntensities(),
	)
	require.Equal(
		t,
		meter.Enforcer().EnforcedMemoryIntensities(),
		meter.EnforcedMemoryIntensities(),
	)
}

func TestControllableMeterLimits(t *testing.T) {
	meter := meterPkg.NewControllableMeter(1000, 2000)

	// check computation limits

	require.Equal(
		t,
		uint(math.MaxUint>>meterPkg.MeterExecutionInternalPrecisionBytes),
		meter.Observer().TotalEnforcedComputationLimit(),
	)
	require.Equal(
		t,
		uint(1000),
		meter.Enforcer().TotalEnforcedComputationLimit(),
	)
	require.Equal(t, uint(1000), meter.TotalEnforcedComputationLimit())

	// check memory limits

	require.Equal(
		t,
		uint(math.MaxUint),
		meter.Observer().TotalEnforcedMemoryLimit(),
	)
	require.Equal(
		t,
		uint(2000),
		meter.Enforcer().TotalEnforcedMemoryLimit(),
	)
	require.Equal(t, uint(2000), meter.TotalEnforcedMemoryLimit())

	// check memory limits after update

	meter.SetTotalMemoryLimit(3000)

	require.Equal(
		t,
		uint(math.MaxUint),
		meter.Observer().TotalEnforcedMemoryLimit(),
	)
	require.Equal(
		t,
		uint(3000),
		meter.Enforcer().TotalEnforcedMemoryLimit(),
	)
	require.Equal(t, uint(3000), meter.TotalEnforcedMemoryLimit())
}

func TestControllableMeterWeights(t *testing.T) {
	meter := meterPkg.NewControllableMeter(
		math.MaxUint,
		math.MaxUint,
		meterPkg.WithComputationWeights(
			map[common.ComputationKind]uint64{
				1: 10 << meterPkg.MeterExecutionInternalPrecisionBytes,
				2: 20 << meterPkg.MeterExecutionInternalPrecisionBytes,
			},
		),
		meterPkg.WithMemoryWeights(
			map[common.MemoryKind]uint64{
				100: 1000,
				200: 2000,
			},
		),
	)

	checkControllableMeter(t, meter, 0, 0, 0, 0)

	meter.MeterComputation(1, 1)
	checkControllableMeter(t, meter, 10, 10, 0, 0)

	meter.MeterComputation(2, 1)
	checkControllableMeter(t, meter, 30, 30, 0, 0)

	meter.MeterComputation(9, 1) // 9 is an unknown computation kind
	checkControllableMeter(t, meter, 30, 30, 0, 0)

	meter.MeterMemory(200, 1)
	checkControllableMeter(t, meter, 30, 30, 2000, 2000)

	meter.MeterMemory(100, 1)
	checkControllableMeter(t, meter, 30, 30, 3000, 3000)

	meter.MeterMemory(900, 1) // 900 is an unknown memory kind
	checkControllableMeter(t, meter, 30, 30, 3000, 3000)

	// Verify updating weights works as expected

	meter.SetComputationWeights(
		map[common.ComputationKind]uint64{
			1: 10 << meterPkg.MeterExecutionInternalPrecisionBytes,
			2: 20 << meterPkg.MeterExecutionInternalPrecisionBytes,
			3: 30 << meterPkg.MeterExecutionInternalPrecisionBytes,
		},
	)

	meter.SetMemoryWeights(
		map[common.MemoryKind]uint64{
			100: 1000,
			200: 2000,
			300: 3000,
		},
	)

	meter.MeterComputation(3, 1)
	checkControllableMeter(t, meter, 60, 60, 3000, 3000)

	meter.MeterMemory(300, 1)
	checkControllableMeter(t, meter, 60, 60, 6000, 6000)
}

func TestControllableMeterEnforcementToggling(t *testing.T) {
	meter := meterPkg.NewControllableMeter(
		math.MaxUint,
		math.MaxUint,
		meterPkg.WithComputationWeights(
			map[common.ComputationKind]uint64{
				1: 10 << meterPkg.MeterExecutionInternalPrecisionBytes,
				2: 20 << meterPkg.MeterExecutionInternalPrecisionBytes,
			},
		),
		meterPkg.WithMemoryWeights(
			map[common.MemoryKind]uint64{
				100: 1000,
				200: 2000,
			},
		),
	)

	meter.DisableAllLimitEnforcements()

	checkControllableMeter(t, meter, 0, 0, 0, 0)

	meter.MeterComputation(1, 1)
	checkControllableMeter(t, meter, 10, 0, 0, 0)

	meter.MeterComputation(2, 1)
	checkControllableMeter(t, meter, 30, 0, 0, 0)

	meter.MeterMemory(200, 1)
	checkControllableMeter(t, meter, 30, 0, 2000, 0)

	meter.MeterMemory(100, 1)
	checkControllableMeter(t, meter, 30, 0, 3000, 0)

	meter.EnableAllLimitEnforcements()

	meter.MeterComputation(1, 1)
	checkControllableMeter(t, meter, 40, 10, 3000, 0)

	meter.MeterComputation(2, 1)
	checkControllableMeter(t, meter, 60, 30, 3000, 0)

	meter.MeterMemory(200, 1)
	checkControllableMeter(t, meter, 60, 30, 5000, 2000)

	meter.MeterMemory(100, 1)
	checkControllableMeter(t, meter, 60, 30, 6000, 3000)
}

func TestControllableMeterChildEnforcementToggling(t *testing.T) {
	parent := meterPkg.NewControllableMeter(
		math.MaxUint,
		math.MaxUint,
		meterPkg.WithComputationWeights(
			map[common.ComputationKind]uint64{
				1: 1 << meterPkg.MeterExecutionInternalPrecisionBytes,
				2: 10 << meterPkg.MeterExecutionInternalPrecisionBytes,
			},
		),
		meterPkg.WithMemoryWeights(
			map[common.MemoryKind]uint64{
				3: 100,
				4: 1000,
			},
		),
	)

	// TODO(patrick): rm type casting
	child := parent.NewChild().(*meterPkg.ControllableMeter)

	checkControllableMeter(t, parent, 0, 0, 0, 0)
	checkControllableMeter(t, child, 0, 0, 0, 0)

	// Toggling child's control don't impact the parent

	child.DisableAllLimitEnforcements()

	parent.MeterComputation(1, 1)
	checkControllableMeter(t, parent, 1, 1, 0, 0)
	checkControllableMeter(t, child, 0, 0, 0, 0)

	child.MeterComputation(2, 1)
	checkControllableMeter(t, parent, 1, 1, 0, 0)
	checkControllableMeter(t, child, 10, 0, 0, 0)

	parent.MeterMemory(3, 1)
	checkControllableMeter(t, parent, 1, 1, 100, 100)
	checkControllableMeter(t, child, 10, 0, 0, 0)

	child.MeterMemory(4, 1)
	checkControllableMeter(t, parent, 1, 1, 100, 100)
	checkControllableMeter(t, child, 10, 0, 1000, 0)

	child.EnableAllLimitEnforcements()

	parent.MeterComputation(1, 1)
	checkControllableMeter(t, parent, 2, 2, 100, 100)
	checkControllableMeter(t, child, 10, 0, 1000, 0)

	child.MeterComputation(2, 1)
	checkControllableMeter(t, parent, 2, 2, 100, 100)
	checkControllableMeter(t, child, 20, 10, 1000, 0)

	parent.MeterMemory(3, 1)
	checkControllableMeter(t, parent, 2, 2, 200, 200)
	checkControllableMeter(t, child, 20, 10, 1000, 0)

	child.MeterMemory(4, 1)
	checkControllableMeter(t, parent, 2, 2, 200, 200)
	checkControllableMeter(t, child, 20, 10, 2000, 1000)

	// Toggling parent's control don't impact the child

	parent.DisableAllLimitEnforcements()

	parent.MeterComputation(1, 1)
	checkControllableMeter(t, parent, 3, 2, 200, 200)
	checkControllableMeter(t, child, 20, 10, 2000, 1000)

	child.MeterComputation(2, 1)
	checkControllableMeter(t, parent, 3, 2, 200, 200)
	checkControllableMeter(t, child, 30, 20, 2000, 1000)

	parent.MeterMemory(3, 1)
	checkControllableMeter(t, parent, 3, 2, 300, 200)
	checkControllableMeter(t, child, 30, 20, 2000, 1000)

	child.MeterMemory(4, 1)
	checkControllableMeter(t, parent, 3, 2, 300, 200)
	checkControllableMeter(t, child, 30, 20, 3000, 2000)

	// Merging works as expected

	err := parent.MergeMeter(child, true)
	require.NoError(t, err)

	checkControllableMeter(t, parent, 33, 22, 3300, 2200)
	checkControllableMeter(t, child, 30, 20, 3000, 2000)
}
