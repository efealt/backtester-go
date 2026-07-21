package backtester

func evaluateTakeProfit(
	position positionState,
	bar MarketBar,
	policy *TakeProfitPolicy,
	allowGap bool,
	blockedSignal float64,
) (exitDecision, bool) {
	if policy == nil || !position.active || !position.takeProfitActive {
		return exitDecision{}, false
	}
	level := position.takeProfitLevel
	if !takeProfitLevelTriggered(position.direction, bar, level) {
		return exitDecision{}, false
	}
	fillPrice, gapFill := levelFill(ExitKindTakeProfit, position.direction, level, bar, allowGap)
	return exitDecision{
		triggerTimestamp:    bar.Timestamp,
		kind:                ExitKindTakeProfit,
		rule:                ExitRuleFixedPercentage,
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

func takeProfitLevelTriggered(direction PositionDirection, bar MarketBar, level float64) bool {
	if !hasOHLC(bar) {
		if direction == DirectionShort {
			return bar.Close <= level
		}
		return bar.Close >= level
	}
	if direction == DirectionShort {
		return bar.Low <= level
	}
	return bar.High >= level
}
