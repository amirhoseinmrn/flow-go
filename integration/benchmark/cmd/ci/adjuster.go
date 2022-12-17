package main

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"go.einride.tech/pid"

	"github.com/onflow/flow-go/integration/benchmark"
)

type adjuster struct {
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}

	params AdjusterParams

	lg                 *benchmark.ContLoadGenerator
	workerStatsTracker *benchmark.WorkerStatsTracker
	log                zerolog.Logger
}
type AdjusterParams struct {
	Interval    time.Duration
	InitialTPS  uint
	MinTPS      uint
	MaxTPS      uint
	MaxInflight uint
}

type adjusterState struct {
	timestamp time.Time
	tps       float64

	executed  uint
	timedout  uint
	targetTPS uint
}

func NewTPSAdjuster(
	ctx context.Context,
	log zerolog.Logger,
	lg *benchmark.ContLoadGenerator,
	workerStatsTracker *benchmark.WorkerStatsTracker,
	params AdjusterParams,
) *adjuster {
	ctx, cancel := context.WithCancel(ctx)
	a := &adjuster{
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}),

		params: params,

		lg:                 lg,
		workerStatsTracker: workerStatsTracker,
		log:                log,
	}

	go func() {
		defer close(a.done)

		err := a.adjustTPSForever()
		if err != nil && err != context.Canceled {
			log.Error().Err(err).Msg("adjuster failed")
		}
	}()

	return a
}

func (a *adjuster) Stop() {
	a.cancel()
	<-a.done
}

func (a *adjuster) adjustTPSForever() (err error) {
	initialStats := a.workerStatsTracker.GetStats()
	lastState := adjusterState{
		timestamp: time.Now(),
		tps:       0,
		targetTPS: a.params.InitialTPS,
		executed:  uint(initialStats.TxsExecuted),
		timedout:  uint(initialStats.TxsTimedout),
	}

	c := &pid.Controller{
		// P controller.
		// We do not use I and D terms becuase they would likely lead to excessive oscillation.
		Config: pid.ControllerConfig{
			ProportionalGain: 1,
			IntegralGain:     1,
		},
	}

	for {
		select {
		// NOTE: not using a ticker here since adjustOnce
		// can take a while and lead to uneven feedback intervals.
		case nowTs := <-time.After(a.params.Interval):
			lastState, err = a.adjustOnce(c, nowTs, lastState)
			if err != nil {
				return fmt.Errorf("adjusting TPS: %w", err)
			}
		case <-a.ctx.Done():
			return a.ctx.Err()
		}
	}
}

// adjustOnce tries to find the maximum TPS that the network can handle using a simple AIMD algorithm.
// The algorithm starts with minTPS as a target.  Each time it is able to reach the target TPS, it
// increases the target by `additiveIncrease`. Each time it fails to reach the target TPS, it decreases
// the target by `multiplicativeDecrease` factor.
//
// To avoid oscillation and speedup conversion we skip the adjustment stage if TPS grew
// compared to the last round.
//
// Target TPS is always bounded by [minTPS, maxTPS].
func (a *adjuster) adjustOnce(c *pid.Controller, nowTs time.Time, lastState adjusterState) (adjusterState, error) {
	timeDiff := nowTs.Sub(lastState.timestamp)
	currentStats := a.workerStatsTracker.GetStats()

	// number of timed out transactions in the last interval
	txsTimedout := currentStats.TxsTimedout - int(lastState.timedout)
	currentTPS := float64(currentStats.TxsExecuted-int(lastState.executed)) / timeDiff.Seconds()

	inflight := float64(currentStats.TxsSent - currentStats.TxsExecuted)
	c.Update(pid.ControllerInput{
		ReferenceSignal:  float64(a.params.MaxInflight),
		ActualSignal:     inflight,
		SamplingInterval: timeDiff,
	})
	targetInflight := inflight + c.State.ControlSignal
	ratio := targetInflight / inflight

	unboundedTPS := uint(currentTPS * ratio)
	boundedTPS := boundTPS(unboundedTPS, a.params.MinTPS, a.params.MaxTPS)
	a.log.Info().
		Uint("lastTargetTPS", lastState.targetTPS).
		Float64("lastTPS", lastState.tps).
		Float64("currentTPS", currentTPS).
		Uint("unboundedTPS", unboundedTPS).
		Uint("targetTPS", boundedTPS).
		Float64("inflight", inflight).
		Int("txsTimedout", txsTimedout).
		Msg("adjusting TPS")

	err := a.lg.SetTPS(boundedTPS)
	if err != nil {
		return lastState, fmt.Errorf("unable to set tps: %w", err)
	}

	return adjusterState{
		timestamp: nowTs,
		tps:       currentTPS,
		targetTPS: boundedTPS,

		timedout: uint(currentStats.TxsTimedout),
		executed: uint(currentStats.TxsExecuted),
	}, nil
}

func boundTPS(tps, min, max uint) uint {
	switch {
	case tps < min:
		return min
	case tps > max:
		return max
	default:
		return tps
	}
}
