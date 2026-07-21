package backtester

// Run validates and executes timestamp-aligned targets against market bars.
// The returned Result contains the processed equity path, trade ledger, exit
// events, metrics, and ruin status. Identical inputs produce identical results.
func Run(bars []MarketBar, targets []Target, config Config) (Result, error) {
	if err := ValidateInputs(bars, targets, config); err != nil {
		return Result{}, err
	}

	equity := config.StartingCapital
	peak := equity
	state := initialRunState(config.InitialExposure, bars[0].Close, config)
	tradeTracker := newTradeTracker(bars[0], config.InitialExposure, equity)
	path := make([]PathPoint, 0, len(bars))
	path = append(path, initialPathPoint(bars[0], targets[0], equity, state.exposure, state.position.snapshot()))
	exitEvents := make([]ExitEvent, 0)
	ruined := false

	for index := 1; index < len(bars); index++ {
		equityBefore := equity
		execution, exitEvent := resolveBar(index, bars, targets, &state, config)
		accounting := accountBar(equity, execution, config)
		tradeTracker.applyStep(execution, accounting, equityBefore, config)
		var drawdown float64
		peak, drawdown = updateDrawdown(peak, accounting.equity)
		path = append(path, pathPoint(bars[index], execution, accounting, drawdown, state.position.snapshot()))
		if exitEvent != nil {
			exitEvents = append(exitEvents, *exitEvent)
		}

		equity = accounting.equity
		if accounting.ruined {
			ruined = true
			tradeTracker.closeForRuin(bars[index].Timestamp, bars[index].Close)
			break
		}
	}

	trades := tradeTracker.result(state.exposure)
	metrics := calculateMetrics(path, trades, config)
	return Result{Path: path, Trades: trades, ExitEvents: exitEvents, Metrics: metrics, Ruined: ruined}, nil
}
