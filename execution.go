package backtester

type executionStep struct {
	targetExposure   float64
	executedExposure float64
	returnExposure   float64
	accrualExposure  float64
	assetReturn      float64
	grossReturn      float64
	turnover         float64
	tradeActions     []tradeAction
}

func baseExecutionStep(index int, bars []MarketBar, targets []Target) executionStep {
	return executionStep{
		targetExposure: targets[index].Exposure,
		assetReturn:    bars[index].Close/bars[index-1].Close - 1,
	}
}

func dueSignalExposure(index int, targets []Target, currentExposure float64, executionLag int) float64 {
	decisionIndex := index - executionLag
	if decisionIndex < 0 {
		return currentExposure
	}
	return targets[decisionIndex].Exposure
}
