package backtester

import "testing"

func TestRunTracksTurnoverTradingCostsAndCompletePath(t *testing.T) {
	bars := testBars([2]float64{100, 100}, [2]float64{100, 100})
	targets := testTargets(bars, 1, 0.5)
	config := testConfig(ExecutionAtOpen)
	config.CommissionBPS = 10
	config.SlippageBPS = 20

	result, err := Run(bars, targets, config)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	point := result.Path[1]
	assertClose(t, point.TargetExposure, 0.5)
	assertClose(t, point.ExecutedExposure, 1)
	assertClose(t, point.ReturnExposure, 1)
	assertClose(t, point.AssetReturn, 0)
	assertClose(t, point.GrossReturn, 0)
	assertClose(t, point.GrossPnL, 0)
	assertClose(t, point.Turnover, 1)
	assertClose(t, point.CommissionCost, 100)
	assertClose(t, point.SlippageCost, 200)
	assertClose(t, point.TotalCost, 300)
	assertClose(t, point.NetReturn, -0.003)
	assertClose(t, point.NetPnL, -300)
	assertClose(t, point.Equity, 99_700)
	assertClose(t, point.Drawdown, -0.003)
}

func TestRunReportsPerBarDollarPnL(t *testing.T) {
	bars := testBars([2]float64{100, 100}, [2]float64{100, 110})
	targets := testTargets(bars, 1, 1)

	result, err := Run(bars, targets, testConfig(ExecutionAtOpen))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	point := result.Path[1]
	assertClose(t, point.GrossReturn, 0.10)
	assertClose(t, point.GrossPnL, 10_000)
	assertClose(t, point.NetReturn, 0.10)
	assertClose(t, point.NetPnL, 10_000)
	assertClose(t, point.Equity, 110_000)
}

func TestRunChargesFullTurnoverWhenExposureReverses(t *testing.T) {
	bars := testBars(
		[2]float64{100, 100},
		[2]float64{100, 100},
		[2]float64{100, 100},
	)
	targets := testTargets(bars, 1, -1, -1)
	config := testConfig(ExecutionAtOpen)
	config.CommissionBPS = 10

	result, err := Run(bars, targets, config)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	assertClose(t, result.Path[1].Turnover, 1)
	assertClose(t, result.Path[2].Turnover, 2)
	assertClose(t, result.Path[2].CommissionCost, 199.8)
}

func TestRunAccountsForCashInterestAndLeverageFinancing(t *testing.T) {
	tests := []struct {
		name              string
		exposure          float64
		wantCashReturn    float64
		wantFinancingCost float64
		wantGrossPnL      float64
		wantNetPnL        float64
		wantEquity        float64
	}{
		{name: "all cash", exposure: 0, wantCashReturn: 0.10, wantGrossPnL: 10_000, wantNetPnL: 10_000, wantEquity: 110_000},
		{name: "unused cash", exposure: 0.5, wantCashReturn: 0.05, wantGrossPnL: 5_000, wantNetPnL: 5_000, wantEquity: 105_000},
		{name: "long borrowed exposure", exposure: 2, wantFinancingCost: 10_000, wantNetPnL: -10_000, wantEquity: 90_000},
		{name: "short borrowed exposure", exposure: -2, wantFinancingCost: 10_000, wantNetPnL: -10_000, wantEquity: 90_000},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bars := testBars([2]float64{100, 100}, [2]float64{100, 100})
			targets := testTargets(bars, test.exposure, test.exposure)
			config := testConfig(ExecutionAtOpen)
			config.InitialExposure = test.exposure
			config.PeriodsPerYear = 1
			config.CashAnnualRate = 0.10
			config.FinancingAnnualRate = 0.10

			result, err := Run(bars, targets, config)
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			point := result.Path[1]
			assertClose(t, point.CashReturn, test.wantCashReturn)
			assertClose(t, point.GrossPnL, test.wantGrossPnL)
			assertClose(t, point.FinancingCost, test.wantFinancingCost)
			assertClose(t, point.TotalCost, test.wantFinancingCost)
			assertClose(t, point.NetPnL, test.wantNetPnL)
			assertClose(t, point.Equity, test.wantEquity)
		})
	}
}

func TestRunAccruesCashAndFinancingOnExposureEnteringBar(t *testing.T) {
	tests := []struct {
		name              string
		initialExposure   float64
		targetExposure    float64
		wantCashReturn    float64
		wantFinancingCost float64
		wantEquity        float64
	}{
		{
			name:           "terminal leveraged entry",
			targetExposure: 2,
			wantCashReturn: 0.10,
			wantEquity:     110_000,
		},
		{
			name:              "leveraged exit",
			initialExposure:   2,
			wantFinancingCost: 10_000,
			wantEquity:        90_000,
		},
		{
			name:              "leveraged reversal",
			initialExposure:   2,
			targetExposure:    -2,
			wantFinancingCost: 10_000,
			wantEquity:        90_000,
		},
		{
			name:              "partial reduction to unleveraged exposure",
			initialExposure:   2,
			targetExposure:    1,
			wantFinancingCost: 10_000,
			wantEquity:        90_000,
		},
		{
			name:            "increase from partial exposure",
			initialExposure: 0.5,
			targetExposure:  1,
			wantCashReturn:  0.05,
			wantEquity:      105_000,
		},
		{
			name:              "leveraged position carried through terminal bar",
			initialExposure:   2,
			targetExposure:    2,
			wantFinancingCost: 10_000,
			wantEquity:        90_000,
		},
	}

	for _, timing := range []ExecutionTiming{ExecutionAtOpen, ExecutionAtClose} {
		for _, test := range tests {
			t.Run(string(timing)+"/"+test.name, func(t *testing.T) {
				bars := testBars([2]float64{100, 100}, [2]float64{100, 100})
				targets := testTargets(bars, test.targetExposure, test.targetExposure)
				config := testConfig(timing)
				config.InitialExposure = test.initialExposure
				config.PeriodsPerYear = 1
				config.CashAnnualRate = 0.10
				config.FinancingAnnualRate = 0.10

				result, err := Run(bars, targets, config)
				if err != nil {
					t.Fatalf("Run() error = %v", err)
				}

				point := result.Path[1]
				assertClose(t, point.CashReturn, test.wantCashReturn)
				assertClose(t, point.FinancingCost, test.wantFinancingCost)
				assertClose(t, point.TotalCost, test.wantFinancingCost)
				assertClose(t, point.Equity, test.wantEquity)
				assertClose(t, point.ExecutedExposure, test.targetExposure)

				var tradeCosts float64
				for _, trade := range result.Trades {
					tradeCosts += trade.Costs
				}
				assertClose(t, tradeCosts, point.FinancingCost)
				if test.name == "leveraged reversal" {
					assertClose(t, result.Trades[0].Costs, test.wantFinancingCost)
					assertClose(t, result.Trades[1].Costs, 0)
				}
			})
		}
	}
}
