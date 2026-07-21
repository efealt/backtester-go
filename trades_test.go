package backtester

import "testing"

func TestRunBuildsCompleteSignalClosedTrade(t *testing.T) {
	bars := testBars([2]float64{100, 100}, [2]float64{100, 110}, [2]float64{110, 110})
	targets := testTargets(bars, 1, 0, 0)
	result, err := Run(bars, targets, testConfig(ExecutionAtOpen))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(result.Trades) != 1 {
		t.Fatalf("trade count = %d, want 1", len(result.Trades))
	}
	trade := result.Trades[0]
	if trade.Direction != DirectionLong || trade.ExitReason != TradeExitSignal {
		t.Fatalf("trade identity = %+v", trade)
	}
	if !trade.EntryTime.Equal(bars[1].Timestamp) {
		t.Fatalf("entry time = %s, want %s", trade.EntryTime, bars[1].Timestamp)
	}
	if trade.ExitTime == nil || !trade.ExitTime.Equal(bars[2].Timestamp) {
		t.Fatalf("exit time = %v, want %s", trade.ExitTime, bars[2].Timestamp)
	}
	assertOptionalLevel(t, "exit price", trade.ExitPrice, 110)
	assertClose(t, trade.EntryPrice, 100)
	assertClose(t, trade.EntryExposure, 1)
	assertClose(t, trade.ExitExposure, 0)
	assertClose(t, trade.GrossPnL, 10_000)
	assertClose(t, trade.Costs, 0)
	assertClose(t, trade.NetPnL, 10_000)
	assertClose(t, trade.Return, 0.10)
	if trade.BarsHeld != 2 {
		t.Fatalf("bars held = %d, want 2", trade.BarsHeld)
	}
}

func TestRunReturnsNoTradesForAnAlwaysFlatPath(t *testing.T) {
	bars := testBars([2]float64{100, 100}, [2]float64{100, 110})
	targets := testTargets(bars, 0, 0)

	result, err := Run(bars, targets, testConfig(ExecutionAtClose))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(result.Trades) != 0 || result.Metrics.TradeCount != 0 {
		t.Fatalf("flat run trades = %d/%d, want 0/0", len(result.Trades), result.Metrics.TradeCount)
	}
}

func TestRunSplitsSignalReversalIntoClosedAndOpenTrades(t *testing.T) {
	bars := testBars([2]float64{100, 100}, [2]float64{100, 110}, [2]float64{110, 99})
	targets := testTargets(bars, 1, -1, -1)
	result, err := Run(bars, targets, testConfig(ExecutionAtOpen))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(result.Trades) != 2 {
		t.Fatalf("trade count = %d, want 2", len(result.Trades))
	}
	longTrade := result.Trades[0]
	shortTrade := result.Trades[1]
	if longTrade.ExitReason != TradeExitSignal || longTrade.ExitTime == nil {
		t.Fatalf("closed long trade = %+v", longTrade)
	}
	if shortTrade.Direction != DirectionShort || shortTrade.ExitTime != nil || shortTrade.ExitPrice != nil || shortTrade.ExitReason != "" {
		t.Fatalf("terminal short trade = %+v", shortTrade)
	}
	assertClose(t, shortTrade.EntryPrice, 110)
	assertClose(t, shortTrade.EntryExposure, -1)
	assertClose(t, shortTrade.ExitExposure, -1)
	assertClose(t, shortTrade.GrossPnL, 11_000)
	if shortTrade.BarsHeld != 1 {
		t.Fatalf("short bars held = %d, want 1", shortTrade.BarsHeld)
	}
}

func TestTradeLedgerAllocatesTradingAndFinancingCosts(t *testing.T) {
	t.Run("entry and exit commission", func(t *testing.T) {
		bars := testBars([2]float64{100, 100}, [2]float64{100, 100}, [2]float64{100, 100})
		targets := testTargets(bars, 1, 0, 0)
		config := testConfig(ExecutionAtOpen)
		config.CommissionBPS = 100
		result, err := Run(bars, targets, config)
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		trade := result.Trades[0]
		assertClose(t, trade.Costs, 1_990)
		assertClose(t, trade.NetPnL, -1_990)
		assertClose(t, trade.Return, -0.0199)
	})

	t.Run("leverage financing", func(t *testing.T) {
		bars := testBars([2]float64{100, 100}, [2]float64{100, 100})
		targets := testTargets(bars, 2, 2)
		config := testConfig(ExecutionAtOpen)
		config.InitialExposure = 2
		config.PeriodsPerYear = 1
		config.FinancingAnnualRate = 0.10
		result, err := Run(bars, targets, config)
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		trade := result.Trades[0]
		assertClose(t, trade.Costs, 10_000)
		assertClose(t, trade.NetPnL, -10_000)
		assertClose(t, trade.Return, -0.10)
	})
}

func TestTradeReturnUsesEquityAtActualTransition(t *testing.T) {
	type expectedTrade struct {
		grossPnL    float64
		costs       float64
		entryEquity float64
	}
	tests := []struct {
		name            string
		timing          ExecutionTiming
		bars            []MarketBar
		initialExposure float64
		targetExposure  float64
		cashAnnualRate  float64
		financingRate   float64
		wantEquity      float64
		wantCashPnL     float64
		wantTrades      []expectedTrade
	}{
		{
			name:           "open entry after cash accrual and before intraday return",
			timing:         ExecutionAtOpen,
			bars:           testBars([2]float64{100, 100}, [2]float64{100, 110}),
			targetExposure: 1,
			cashAnnualRate: 0.10,
			wantEquity:     119_000,
			wantCashPnL:    10_000,
			wantTrades:     []expectedTrade{{grossPnL: 10_000, costs: 1_000, entryEquity: 110_000}},
		},
		{
			name:           "close entry after cash accrual and full bar",
			timing:         ExecutionAtClose,
			bars:           testBars([2]float64{100, 100}, [2]float64{100, 110}),
			targetExposure: 1,
			cashAnnualRate: 0.10,
			wantEquity:     109_000,
			wantCashPnL:    10_000,
			wantTrades:     []expectedTrade{{costs: 1_000, entryEquity: 110_000}},
		},
		{
			name:            "open reversal after return financing and closing cost",
			timing:          ExecutionAtOpen,
			bars:            testBars([2]float64{100, 100}, [2]float64{110, 99}),
			initialExposure: 2,
			targetExposure:  -1,
			financingRate:   0.10,
			wantEquity:      117_000,
			wantTrades:      []expectedTrade{{grossPnL: 20_000, costs: 12_000, entryEquity: 100_000}, {grossPnL: 10_000, costs: 1_000, entryEquity: 108_000}},
		},
		{
			name:            "close reversal after return financing and closing cost",
			timing:          ExecutionAtClose,
			bars:            testBars([2]float64{100, 100}, [2]float64{100, 110}),
			initialExposure: 2,
			targetExposure:  -1,
			financingRate:   0.10,
			wantEquity:      107_000,
			wantTrades:      []expectedTrade{{grossPnL: 20_000, costs: 12_000, entryEquity: 100_000}, {costs: 1_000, entryEquity: 108_000}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			targets := testTargets(test.bars, test.targetExposure, test.targetExposure)
			config := testConfig(test.timing)
			config.InitialExposure = test.initialExposure
			config.CommissionBPS = 100
			config.PeriodsPerYear = 1
			config.CashAnnualRate = test.cashAnnualRate
			config.FinancingAnnualRate = test.financingRate

			result, err := Run(test.bars, targets, config)
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if len(result.Trades) != len(test.wantTrades) {
				t.Fatalf("trade count = %d, want %d", len(result.Trades), len(test.wantTrades))
			}

			var tradeNetPnL float64
			var tradeCosts float64
			for index, want := range test.wantTrades {
				trade := result.Trades[index]
				wantNetPnL := want.grossPnL - want.costs
				assertClose(t, trade.GrossPnL, want.grossPnL)
				assertClose(t, trade.Costs, want.costs)
				assertClose(t, trade.NetPnL, wantNetPnL)
				assertClose(t, trade.Return, wantNetPnL/want.entryEquity)
				tradeNetPnL += trade.NetPnL
				tradeCosts += trade.Costs
			}

			point := result.Path[1]
			assertClose(t, point.Equity, test.wantEquity)
			assertClose(t, tradeNetPnL+test.wantCashPnL, point.NetPnL)
			assertClose(t, tradeCosts, point.TotalCost)
		})
	}
}

func TestTradeLedgerRecordsExitReasonsAndPartialTerminalPosition(t *testing.T) {
	tests := []struct {
		name   string
		config func(*Config)
		reason TradeExitReason
	}{
		{name: "stop loss", config: func(c *Config) { c.Exits.StopLoss = fixedStop(0.05, ExitSameBar, 0, false) }, reason: TradeExitStopLoss},
		{name: "take profit", config: func(c *Config) { c.Exits.TakeProfit = &TakeProfitPolicy{Percent: 0.05, Timing: ExitSameBar} }, reason: TradeExitTakeProfit},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bars := exitTestBars([4]float64{100, 100, 100, 100}, [4]float64{100, 106, 94, 100})
			targets := testTargets(bars, 1, 1)
			config := testConfig(ExecutionAtClose)
			config.InitialExposure = 1
			test.config(&config)
			result, err := Run(bars, targets, config)
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if result.Trades[0].ExitReason != test.reason {
				t.Fatalf("exit reason = %q, want %q", result.Trades[0].ExitReason, test.reason)
			}
		})
	}

	t.Run("partial exit remains open", func(t *testing.T) {
		bars := exitTestBars([4]float64{100, 100, 100, 100}, [4]float64{100, 101, 94, 96})
		targets := testTargets(bars, 1, 1)
		config := testConfig(ExecutionAtClose)
		config.InitialExposure = 1
		config.Exits.StopLoss = fixedStop(0.05, ExitSameBar, 0.5, false)
		result, err := Run(bars, targets, config)
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		trade := result.Trades[0]
		if trade.ExitTime != nil || trade.ExitPrice != nil || trade.ExitReason != "" {
			t.Fatalf("partial terminal trade = %+v, want open", trade)
		}
		assertClose(t, trade.ExitExposure, 0.5)
	})
}

func TestRuinClosesTheActiveTradeAtActualAccountLoss(t *testing.T) {
	bars := testBars([2]float64{100, 100}, [2]float64{100, 200})
	targets := testTargets(bars, -2, -2)
	result, err := Run(bars, targets, testConfig(ExecutionAtOpen))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	trade := result.Trades[0]
	if trade.ExitReason != TradeExitRuin || trade.ExitTime == nil {
		t.Fatalf("ruined trade = %+v", trade)
	}
	assertClose(t, trade.NetPnL, -100_000)
	assertClose(t, trade.Return, -1)
}
