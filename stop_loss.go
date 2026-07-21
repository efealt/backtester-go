package backtester

func evaluateStopLoss(
	position positionState,
	bar MarketBar,
	policy *StopLossPolicy,
	allowGap bool,
	blockedSignal float64,
) (exitDecision, bool) {
	if policy == nil || !position.active {
		return exitDecision{}, false
	}

	fixedTriggered := position.fixedStopActive && stopLevelTriggered(position.direction, bar, position.fixedStopLevel)
	trailingTriggered := position.trailingStopActive && stopLevelTriggered(position.direction, bar, position.trailingStopLevel)
	if !fixedTriggered && !trailingTriggered {
		return exitDecision{}, false
	}

	rule := ExitRuleFixed
	level := position.fixedStopLevel
	switch {
	case fixedTriggered && trailingTriggered:
		rule = ExitRuleFixedAndTrailing
		level = moreProtectiveStop(position.direction, position.fixedStopLevel, position.trailingStopLevel)
	case trailingTriggered:
		rule = ExitRuleTrailing
		level = position.trailingStopLevel
	}
	fillPrice, gapFill := levelFill(ExitKindStopLoss, position.direction, level, bar, allowGap)
	return exitDecision{
		triggerTimestamp:    bar.Timestamp,
		kind:                ExitKindStopLoss,
		rule:                rule,
		timing:              policy.Timing,
		direction:           position.direction,
		activeLevel:         level,
		triggerFillPrice:    fillPrice,
		triggerGapFill:      gapFill,
		responseFraction:    policy.TriggeredExposureFraction,
		waitForSignalChange: policy.WaitForSignalChange,
		blockedSignal:       blockedSignal,
	}, true
}

func stopLevelTriggered(direction PositionDirection, bar MarketBar, level float64) bool {
	if !hasOHLC(bar) {
		if direction == DirectionShort {
			return bar.Close >= level
		}
		return bar.Close <= level
	}
	if direction == DirectionShort {
		return bar.High >= level
	}
	return bar.Low <= level
}

func moreProtectiveStop(direction PositionDirection, first, second float64) float64 {
	if direction == DirectionShort {
		return minFloat(first, second)
	}
	return maxFloat(first, second)
}
