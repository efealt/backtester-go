package backtester

import (
	"encoding/json"
	"math"
	"testing"
	"time"
)

func TestCalculateMetricsMatchesHandCalculatedPath(t *testing.T) {
	first := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
	second := first.AddDate(0, 0, 1)
	path := []PathPoint{
		{Timestamp: first, Equity: 100},
		{Timestamp: second, Equity: 110, ReturnExposure: 1, Turnover: 1, TotalCost: 1},
		{Timestamp: second.AddDate(0, 0, 1), Equity: 99, ReturnExposure: 0.5, Turnover: 2, TotalCost: 2},
	}
	winExit := second
	lossExit := second.AddDate(0, 0, 1)
	trades := []Trade{
		{ExitTime: &winExit, NetPnL: 10},
		{ExitTime: &lossExit, NetPnL: -5},
		{NetPnL: 3},
	}
	metrics := calculateMetrics(path, trades, Config{PeriodsPerYear: 2})

	assertClose(t, metrics.TotalReturn, -0.01)
	assertClose(t, metrics.AnnualizedReturn, -0.01)
	assertClose(t, metrics.AnnualizedVolatility, 0.20)
	assertClose(t, metrics.DownsideDeviation, 0.10)
	assertClose(t, metrics.Sharpe, 0)
	assertClose(t, metrics.Sortino, 0)
	assertClose(t, metrics.Calmar, -0.10)
	assertClose(t, metrics.MaximumDrawdown, -0.10)
	if metrics.MaximumDrawdownDuration != 1 {
		t.Fatalf("maximum drawdown duration = %d, want 1", metrics.MaximumDrawdownDuration)
	}
	if metrics.RecoveryTime != -1 {
		t.Fatalf("recovery time = %d, want -1", metrics.RecoveryTime)
	}
	assertClose(t, metrics.UlcerIndex, math.Sqrt(0.01/2))
	if metrics.WorstRolling12MReturn == nil {
		t.Fatal("worst rolling 12-month return is nil")
	}
	assertClose(t, *metrics.WorstRolling12MReturn, -0.01)
	assertClose(t, metrics.WorstPeriodReturn, -0.10)
	assertClose(t, metrics.ValueAtRisk95, 0.10)
	assertClose(t, metrics.ExpectedShortfall95, 0.10)
	assertClose(t, metrics.Turnover, 3)
	assertClose(t, metrics.TotalCosts, 3)
	assertClose(t, metrics.AverageExposure, 0.75)
	assertClose(t, metrics.HitRate, 0.50)
	assertClose(t, metrics.PayoffRatio, 2)
	assertClose(t, metrics.ProfitFactor, 2)
	if metrics.Observations != 2 || metrics.TradeCount != 3 {
		t.Fatalf("counts = observations %d, trades %d; want 2 and 3", metrics.Observations, metrics.TradeCount)
	}
}

func TestCalculateMetricsHasStableEmptyAndOpenTradeBehavior(t *testing.T) {
	empty := calculateMetrics(nil, nil, Config{PeriodsPerYear: 252})
	if empty != (Metrics{}) {
		t.Fatalf("empty metrics = %+v, want zero value", empty)
	}

	path := []PathPoint{{Equity: 100}, {Equity: 100}, {Equity: 100}}
	metrics := calculateMetrics(path, []Trade{{EntryExposure: 1}}, Config{PeriodsPerYear: 2})
	assertFiniteMetrics(t, metrics)
	assertClose(t, metrics.TotalReturn, 0)
	assertClose(t, metrics.AnnualizedReturn, 0)
	assertClose(t, metrics.AnnualizedVolatility, 0)
	assertClose(t, metrics.DownsideDeviation, 0)
	assertClose(t, metrics.Sharpe, 0)
	assertClose(t, metrics.Sortino, 0)
	assertClose(t, metrics.Calmar, 0)
	assertClose(t, metrics.HitRate, 0)
	assertClose(t, metrics.PayoffRatio, 0)
	assertClose(t, metrics.ProfitFactor, 0)
	if metrics.Observations != 2 || metrics.TradeCount != 1 {
		t.Fatalf("counts = observations %d, trades %d; want 2 and 1", metrics.Observations, metrics.TradeCount)
	}
	if metrics.WorstRolling12MReturn == nil {
		t.Fatal("worst rolling 12-month return is nil for a complete window")
	}
	assertClose(t, *metrics.WorstRolling12MReturn, 0)

	insufficient := calculateMetrics(path[:2], nil, Config{PeriodsPerYear: 2})
	if insufficient.WorstRolling12MReturn != nil {
		t.Fatalf("insufficient rolling return = %v, want nil", *insufficient.WorstRolling12MReturn)
	}
}

func TestTradeRatiosAreStableWhenAWinOrLossSubsetIsEmpty(t *testing.T) {
	exit := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		trades  []Trade
		hitRate float64
	}{
		{name: "only wins", trades: []Trade{{ExitTime: &exit, NetPnL: 10}}, hitRate: 1},
		{name: "only losses", trades: []Trade{{ExitTime: &exit, NetPnL: -10}}, hitRate: 0},
		{name: "only breakeven", trades: []Trade{{ExitTime: &exit, NetPnL: 0}}, hitRate: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			metrics := Metrics{}
			applyTradeMetrics(&metrics, test.trades)
			assertClose(t, metrics.HitRate, test.hitRate)
			assertClose(t, metrics.PayoffRatio, 0)
			assertClose(t, metrics.ProfitFactor, 0)
		})
	}
}

func TestCalculateMetricsHasStableRuinedPathBehavior(t *testing.T) {
	path := []PathPoint{{Equity: 100}, {Equity: 0}}
	metrics := calculateMetrics(path, nil, Config{PeriodsPerYear: 1})

	assertFiniteMetrics(t, metrics)
	assertClose(t, metrics.TotalReturn, -1)
	assertClose(t, metrics.AnnualizedReturn, -1)
	assertClose(t, metrics.AnnualizedVolatility, 0)
	assertClose(t, metrics.DownsideDeviation, 1)
	assertClose(t, metrics.Sharpe, 0)
	assertClose(t, metrics.Sortino, -1)
	assertClose(t, metrics.Calmar, -1)
	assertClose(t, metrics.MaximumDrawdown, -1)
	if metrics.MaximumDrawdownDuration != 1 || metrics.RecoveryTime != -1 {
		t.Fatalf("ruin duration/recovery = %d/%d, want 1/-1", metrics.MaximumDrawdownDuration, metrics.RecoveryTime)
	}
	assertClose(t, metrics.UlcerIndex, 1)
	if metrics.WorstRolling12MReturn == nil {
		t.Fatal("ruined rolling return is nil")
	}
	assertClose(t, *metrics.WorstRolling12MReturn, -1)
	assertClose(t, metrics.WorstPeriodReturn, -1)
	assertClose(t, metrics.ValueAtRisk95, 1)
	assertClose(t, metrics.ExpectedShortfall95, 1)
	if metrics.Observations != 1 {
		t.Fatalf("ruin observations = %d, want 1", metrics.Observations)
	}
}

func assertFiniteMetrics(t *testing.T, metrics Metrics) {
	t.Helper()
	if _, err := json.Marshal(metrics); err != nil {
		t.Fatalf("json.Marshal(metrics) error = %v", err)
	}
}
