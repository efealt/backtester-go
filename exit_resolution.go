package backtester

import (
	"math"
	"time"
)

type exitDecision struct {
	triggerTimestamp    time.Time
	kind                ExitKind
	rule                ExitRule
	timing              ExitTiming
	direction           PositionDirection
	activeLevel         float64
	triggerFillPrice    float64
	triggerGapFill      bool
	collision           bool
	responseFraction    float64
	waitForSignalChange bool
	blockedSignal       float64
}

func resolveBar(
	index int,
	bars []MarketBar,
	targets []Target,
	state *runState,
	config Config,
) (executionStep, *ExitEvent) {
	if state.pendingExit != nil {
		return applyPendingExit(index, bars, targets, state, config)
	}
	if config.ExecutionTiming == ExecutionAtOpen {
		return resolveOpenBar(index, bars, targets, state, config)
	}
	return resolveCloseBar(index, bars, targets, state, config)
}

func resolveOpenBar(
	index int,
	bars []MarketBar,
	targets []Target,
	state *runState,
	config Config,
) (executionStep, *ExitEvent) {
	bar := bars[index]
	previousBar := bars[index-1]
	step := baseExecutionStep(index, bars, targets)
	previousExposure := state.exposure
	rawSignal := dueSignalExposure(index, targets, previousExposure, config.ExecutionLag)
	desiredExposure := state.resolveSignal(rawSignal)
	previousDirection := directionForExposure(previousExposure)
	desiredDirection := directionForExposure(desiredExposure)
	positionExistedAtOpen := previousDirection != DirectionFlat && previousDirection == desiredDirection

	step.executedExposure = desiredExposure
	step.returnExposure = desiredExposure
	step.accrualExposure = previousExposure
	step.turnover = math.Abs(desiredExposure - previousExposure)
	overnightReturn := bar.Open/previousBar.Close - 1
	intradayReturn := bar.Close/bar.Open - 1
	step.grossReturn = previousExposure*overnightReturn + desiredExposure*intradayReturn
	state.applySignal(desiredExposure, bar.Open, config)

	decision, triggered := evaluateExit(state.position, bar, config, positionExistedAtOpen, state.signalExposure)
	if !triggered {
		recordOpenActions(&step, index, bar, previousBar, previousExposure, desiredExposure)
		state.position.observe(bar, config)
		return step, nil
	}
	if decision.timing == ExitNextBar {
		recordOpenActions(&step, index, bar, previousBar, previousExposure, desiredExposure)
		state.pendingExit = &decision
		return step, nil
	}

	responseExposure := desiredExposure * decision.responseFraction
	fillPrice := decision.triggerFillPrice
	step.grossReturn = previousExposure*overnightReturn +
		desiredExposure*(fillPrice/bar.Open-1) +
		responseExposure*(bar.Close/fillPrice-1)
	step.turnover += math.Abs(responseExposure - desiredExposure)
	step.executedExposure = responseExposure
	step.addReturn(index, previousExposure, previousExposure*overnightReturn)
	step.addFinancing(previousExposure)
	step.addTransition(bar.Timestamp, bar.Open, previousExposure, desiredExposure, TradeExitSignal)
	step.addReturn(index, desiredExposure, desiredExposure*(fillPrice/bar.Open-1))
	step.addTransition(bar.Timestamp, fillPrice, desiredExposure, responseExposure, tradeReasonForExit(decision.kind))
	step.addReturn(index, responseExposure, responseExposure*(bar.Close/fillPrice-1))
	state.applyExit(responseExposure, fillPrice, decision, config)
	state.position.observe(bar, config)
	event := exitEvent(decision, fillPrice, decision.triggerGapFill, desiredExposure, responseExposure)
	return step, &event
}

func resolveCloseBar(
	index int,
	bars []MarketBar,
	targets []Target,
	state *runState,
	config Config,
) (executionStep, *ExitEvent) {
	bar := bars[index]
	previousBar := bars[index-1]
	step := baseExecutionStep(index, bars, targets)
	previousExposure := state.exposure
	step.executedExposure = previousExposure
	step.returnExposure = previousExposure
	step.accrualExposure = previousExposure
	step.grossReturn = previousExposure * step.assetReturn
	rawSignal := dueSignalExposure(index, targets, previousExposure, config.ExecutionLag)

	decision, triggered := evaluateExit(state.position, bar, config, true, state.signalExposure)
	if triggered {
		if decision.timing == ExitNextBar {
			step.addFinancing(previousExposure)
			step.addReturn(index, previousExposure, step.grossReturn)
			state.pendingExit = &decision
			return step, nil
		}
		responseExposure := previousExposure * decision.responseFraction
		fillPrice := decision.triggerFillPrice
		step.grossReturn = previousExposure*(fillPrice/previousBar.Close-1) +
			responseExposure*(bar.Close/fillPrice-1)
		step.turnover = math.Abs(responseExposure - previousExposure)
		step.executedExposure = responseExposure
		step.addFinancing(previousExposure)
		step.addReturn(index, previousExposure, previousExposure*(fillPrice/previousBar.Close-1))
		step.addTransition(bar.Timestamp, fillPrice, previousExposure, responseExposure, tradeReasonForExit(decision.kind))
		step.addReturn(index, responseExposure, responseExposure*(bar.Close/fillPrice-1))
		state.applyExit(responseExposure, fillPrice, decision, config)
		state.position.observe(bar, config)
		event := exitEvent(decision, fillPrice, decision.triggerGapFill, previousExposure, responseExposure)
		return step, &event
	}

	state.position.observe(bar, config)
	desiredExposure := state.resolveSignal(rawSignal)
	step.executedExposure = desiredExposure
	step.turnover = math.Abs(desiredExposure - previousExposure)
	step.addFinancing(previousExposure)
	step.addReturn(index, previousExposure, step.grossReturn)
	step.addTransition(bar.Timestamp, bar.Close, previousExposure, desiredExposure, TradeExitSignal)
	state.applySignal(desiredExposure, bar.Close, config)
	return step, nil
}

func applyPendingExit(
	index int,
	bars []MarketBar,
	targets []Target,
	state *runState,
	config Config,
) (executionStep, *ExitEvent) {
	bar := bars[index]
	previousBar := bars[index-1]
	decision := *state.pendingExit
	state.pendingExit = nil
	previousExposure := state.exposure
	responseExposure := previousExposure * decision.responseFraction
	step := baseExecutionStep(index, bars, targets)
	step.executedExposure = responseExposure
	step.accrualExposure = previousExposure
	step.turnover = math.Abs(responseExposure - previousExposure)

	fillPrice := bar.Close
	gapFill := false
	if hasOHLC(bar) {
		fillPrice = bar.Open
		gapFill = openBeyondLevel(decision.kind, decision.direction, decision.activeLevel, bar.Open)
		overnightReturn := bar.Open/previousBar.Close - 1
		intradayReturn := bar.Close/bar.Open - 1
		step.returnExposure = responseExposure
		step.grossReturn = previousExposure*overnightReturn + responseExposure*intradayReturn
		step.addReturn(index, previousExposure, previousExposure*overnightReturn)
		step.addFinancing(previousExposure)
		step.addTransition(bar.Timestamp, fillPrice, previousExposure, responseExposure, tradeReasonForExit(decision.kind))
		step.addReturn(index, responseExposure, responseExposure*intradayReturn)
	} else {
		step.returnExposure = previousExposure
		step.grossReturn = previousExposure * step.assetReturn
		step.addFinancing(previousExposure)
		step.addReturn(index, previousExposure, step.grossReturn)
		step.addTransition(bar.Timestamp, fillPrice, previousExposure, responseExposure, tradeReasonForExit(decision.kind))
	}

	state.applyExit(responseExposure, fillPrice, decision, config)
	state.position.observe(bar, config)
	event := exitEvent(decision, fillPrice, gapFill, previousExposure, responseExposure)
	return step, &event
}

func recordOpenActions(
	step *executionStep,
	index int,
	bar MarketBar,
	previousBar MarketBar,
	previousExposure float64,
	desiredExposure float64,
) {
	overnightContribution := previousExposure * (bar.Open/previousBar.Close - 1)
	intradayContribution := desiredExposure * (bar.Close/bar.Open - 1)
	step.addReturn(index, previousExposure, overnightContribution)
	step.addFinancing(previousExposure)
	step.addTransition(bar.Timestamp, bar.Open, previousExposure, desiredExposure, TradeExitSignal)
	step.addReturn(index, desiredExposure, intradayContribution)
}

func tradeReasonForExit(kind ExitKind) TradeExitReason {
	if kind == ExitKindTakeProfit {
		return TradeExitTakeProfit
	}
	return TradeExitStopLoss
}

func evaluateExit(
	position positionState,
	bar MarketBar,
	config Config,
	allowGap bool,
	blockedSignal float64,
) (exitDecision, bool) {
	stop, stopTriggered := evaluateStopLoss(position, bar, config.Exits.StopLoss, allowGap, blockedSignal)
	profit, profitTriggered := evaluateTakeProfit(position, bar, config.Exits.TakeProfit, allowGap, blockedSignal)
	if stopTriggered {
		stop.collision = profitTriggered
		return stop, true
	}
	return profit, profitTriggered
}

func levelFill(
	kind ExitKind,
	direction PositionDirection,
	level float64,
	bar MarketBar,
	allowGap bool,
) (float64, bool) {
	if !hasOHLC(bar) {
		return bar.Close, false
	}
	if allowGap && openBeyondLevel(kind, direction, level, bar.Open) {
		return bar.Open, true
	}
	return level, false
}

func openBeyondLevel(kind ExitKind, direction PositionDirection, level, open float64) bool {
	if kind == ExitKindStopLoss {
		if direction == DirectionShort {
			return open > level
		}
		return open < level
	}
	if direction == DirectionShort {
		return open < level
	}
	return open > level
}

func exitEvent(
	decision exitDecision,
	fillPrice float64,
	gapFill bool,
	previousExposure float64,
	responseExposure float64,
) ExitEvent {
	return ExitEvent{
		Timestamp:        decision.triggerTimestamp,
		Kind:             decision.kind,
		Rule:             decision.rule,
		Timing:           decision.timing,
		Direction:        decision.direction,
		ActiveLevel:      decision.activeLevel,
		FillPrice:        fillPrice,
		GapFill:          gapFill,
		Collision:        decision.collision,
		PreviousExposure: previousExposure,
		ResponseExposure: responseExposure,
	}
}
