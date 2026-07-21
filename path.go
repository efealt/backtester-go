package backtester

func initialPathPoint(
	bar MarketBar,
	target Target,
	startingCapital float64,
	initialExposure float64,
	position positionSnapshot,
) PathPoint {
	return PathPoint{
		Timestamp:         bar.Timestamp,
		TargetExposure:    target.Exposure,
		ExecutedExposure:  initialExposure,
		ReturnExposure:    initialExposure,
		PositionDirection: position.direction,
		EntryPrice:        position.entryPrice,
		FixedStopLevel:    position.fixedStopLevel,
		TrailingStopLevel: position.trailingStopLevel,
		TakeProfitLevel:   position.takeProfitLevel,
		Equity:            startingCapital,
	}
}

func pathPoint(
	bar MarketBar,
	execution executionStep,
	accounting accountingStep,
	drawdown float64,
	position positionSnapshot,
) PathPoint {
	return PathPoint{
		Timestamp:         bar.Timestamp,
		TargetExposure:    execution.targetExposure,
		ExecutedExposure:  execution.executedExposure,
		ReturnExposure:    execution.returnExposure,
		PositionDirection: position.direction,
		EntryPrice:        position.entryPrice,
		FixedStopLevel:    position.fixedStopLevel,
		TrailingStopLevel: position.trailingStopLevel,
		TakeProfitLevel:   position.takeProfitLevel,
		AssetReturn:       execution.assetReturn,
		GrossReturn:       execution.grossReturn,
		GrossPnL:          accounting.grossPnL,
		CashReturn:        accounting.cashReturn,
		FinancingCost:     accounting.financingCost,
		CommissionCost:    accounting.commissionCost,
		SlippageCost:      accounting.slippageCost,
		TotalCost:         accounting.totalCost,
		NetReturn:         accounting.netReturn,
		NetPnL:            accounting.netPnL,
		Equity:            accounting.equity,
		Turnover:          execution.turnover,
		Drawdown:          drawdown,
	}
}
