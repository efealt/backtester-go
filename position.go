package backtester

import "math"

type positionState struct {
	active             bool
	direction          PositionDirection
	entryPrice         float64
	highWatermark      float64
	lowWatermark       float64
	fixedStopLevel     float64
	fixedStopActive    bool
	trailingStopLevel  float64
	trailingStopActive bool
	takeProfitLevel    float64
	takeProfitActive   bool
}

type reentryState struct {
	waiting       bool
	blockedSignal float64
}

type runState struct {
	exposure       float64
	signalExposure float64
	position       positionState
	reentry        reentryState
	pendingExit    *exitDecision
}

type positionSnapshot struct {
	direction         PositionDirection
	entryPrice        *float64
	fixedStopLevel    *float64
	trailingStopLevel *float64
	takeProfitLevel   *float64
}

func initialRunState(exposure, entryPrice float64, config Config) runState {
	state := runState{exposure: exposure, signalExposure: exposure}
	state.position = newPositionState(exposure, entryPrice, config)
	return state
}

func (state *runState) resolveSignal(signal float64) float64 {
	if !state.reentry.waiting {
		return signal
	}
	if signal == state.reentry.blockedSignal {
		return state.exposure
	}
	state.reentry = reentryState{}
	return signal
}

func (state *runState) applySignal(exposure, price float64, config Config) {
	state.signalExposure = exposure
	state.setExposure(exposure, price, config)
}

func (state *runState) applyExit(exposure, fillPrice float64, decision exitDecision, config Config) {
	state.setExposure(exposure, fillPrice, config)
	if decision.waitForSignalChange {
		state.reentry = reentryState{waiting: true, blockedSignal: decision.blockedSignal}
	}
}

func (state *runState) setExposure(exposure, price float64, config Config) {
	previousDirection := directionForExposure(state.exposure)
	nextDirection := directionForExposure(exposure)
	state.exposure = exposure
	if nextDirection == DirectionFlat {
		state.position = positionState{}
		return
	}
	if !state.position.active || previousDirection != nextDirection {
		state.position = newPositionState(exposure, price, config)
	}
}

func newPositionState(exposure, entryPrice float64, config Config) positionState {
	direction := directionForExposure(exposure)
	if direction == DirectionFlat {
		return positionState{}
	}
	state := positionState{
		active:        true,
		direction:     direction,
		entryPrice:    entryPrice,
		highWatermark: entryPrice,
		lowWatermark:  entryPrice,
	}
	state.refreshLevels(config)
	return state
}

func (state *positionState) observe(bar MarketBar, config Config) {
	if !state.active {
		return
	}
	high := bar.Close
	low := bar.Close
	if hasOHLC(bar) {
		high = bar.High
		low = bar.Low
	}
	state.highWatermark = math.Max(state.highWatermark, high)
	state.lowWatermark = math.Min(state.lowWatermark, low)
	state.refreshLevels(config)
}

func (state *positionState) refreshLevels(config Config) {
	state.fixedStopActive = false
	state.trailingStopActive = false
	state.takeProfitActive = false
	if !state.active {
		return
	}
	if policy := config.Exits.StopLoss; policy != nil {
		if policy.FixedPercent != nil {
			state.fixedStopActive = true
			state.fixedStopLevel = lossLevel(state.entryPrice, *policy.FixedPercent, state.direction)
		}
		if policy.TrailingPercent != nil {
			state.trailingStopActive = true
			anchor := state.highWatermark
			if state.direction == DirectionShort {
				anchor = state.lowWatermark
			}
			state.trailingStopLevel = lossLevel(anchor, *policy.TrailingPercent, state.direction)
		}
	}
	if policy := config.Exits.TakeProfit; policy != nil {
		state.takeProfitActive = true
		state.takeProfitLevel = profitLevel(state.entryPrice, policy.Percent, state.direction)
	}
}

func (state positionState) snapshot() positionSnapshot {
	if !state.active {
		return positionSnapshot{direction: DirectionFlat}
	}
	snapshot := positionSnapshot{
		direction:  state.direction,
		entryPrice: floatPointer(state.entryPrice),
	}
	if state.fixedStopActive {
		snapshot.fixedStopLevel = floatPointer(state.fixedStopLevel)
	}
	if state.trailingStopActive {
		snapshot.trailingStopLevel = floatPointer(state.trailingStopLevel)
	}
	if state.takeProfitActive {
		snapshot.takeProfitLevel = floatPointer(state.takeProfitLevel)
	}
	return snapshot
}

func directionForExposure(exposure float64) PositionDirection {
	if exposure > 0 {
		return DirectionLong
	}
	if exposure < 0 {
		return DirectionShort
	}
	return DirectionFlat
}

func lossLevel(anchor, percent float64, direction PositionDirection) float64 {
	if direction == DirectionShort {
		return anchor * (1 + percent)
	}
	return anchor * (1 - percent)
}

func profitLevel(entryPrice, percent float64, direction PositionDirection) float64 {
	if direction == DirectionShort {
		return entryPrice * (1 - percent)
	}
	return entryPrice * (1 + percent)
}

func floatPointer(value float64) *float64 {
	copy := value
	return &copy
}
