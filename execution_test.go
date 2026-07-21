package backtester

import (
	"math"
	"testing"
	"time"
)

func TestRunHonorsCausalExecutionLagAtClose(t *testing.T) {
	bars := testBars(
		[2]float64{100, 100},
		[2]float64{100, 110},
		[2]float64{110, 121},
		[2]float64{121, 133.1},
	)
	targets := testTargets(bars, 1, 1, 1, 1)
	config := testConfig(ExecutionAtClose)
	config.ExecutionLag = 2

	result, err := Run(bars, targets, config)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Path[1].ExecutedExposure != 0 || result.Path[2].ExecutedExposure != 1 {
		t.Fatalf("executed exposures = [%v, %v], want [0, 1]", result.Path[1].ExecutedExposure, result.Path[2].ExecutedExposure)
	}
	assertClose(t, result.Path[2].GrossReturn, 0)
	assertClose(t, result.Path[3].GrossReturn, 0.10)
	assertClose(t, result.Path[3].Equity, 110_000)
}

func TestRunHonorsOpenAndCloseExecutionTiming(t *testing.T) {
	bars := testBars(
		[2]float64{100, 100},
		[2]float64{100, 110},
		[2]float64{110, 121},
	)
	targets := testTargets(bars, 1, 1, 1)

	openResult, err := Run(bars, targets, testConfig(ExecutionAtOpen))
	if err != nil {
		t.Fatalf("Run(open) error = %v", err)
	}
	closeResult, err := Run(bars, targets, testConfig(ExecutionAtClose))
	if err != nil {
		t.Fatalf("Run(close) error = %v", err)
	}

	assertClose(t, openResult.Path[1].GrossReturn, 0.10)
	assertClose(t, openResult.Path[1].Equity, 110_000)
	assertClose(t, closeResult.Path[1].GrossReturn, 0)
	assertClose(t, closeResult.Path[2].GrossReturn, 0.10)
	assertClose(t, closeResult.Path[2].Equity, 110_000)
}

func TestOpenExecutionDoesNotCreditTheNewTargetWithThePriorGap(t *testing.T) {
	bars := testBars([2]float64{100, 100}, [2]float64{110, 110})
	targets := testTargets(bars, 1, 1)

	result, err := Run(bars, targets, testConfig(ExecutionAtOpen))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	assertClose(t, result.Path[1].AssetReturn, 0.10)
	assertClose(t, result.Path[1].GrossReturn, 0)
	assertClose(t, result.Path[1].Equity, 100_000)
}

func TestRunSupportsSignedFractionalAndLeveragedExposure(t *testing.T) {
	tests := []struct {
		name           string
		exposure       float64
		wantReturn     float64
		wantFinalValue float64
	}{
		{name: "long", exposure: 1, wantReturn: 0.10, wantFinalValue: 110_000},
		{name: "short", exposure: -1, wantReturn: -0.10, wantFinalValue: 90_000},
		{name: "fractional", exposure: 0.5, wantReturn: 0.05, wantFinalValue: 105_000},
		{name: "leveraged", exposure: 2, wantReturn: 0.20, wantFinalValue: 120_000},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bars := testBars([2]float64{100, 100}, [2]float64{100, 110})
			targets := testTargets(bars, test.exposure, test.exposure)
			result, err := Run(bars, targets, testConfig(ExecutionAtOpen))
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			assertClose(t, result.Path[1].GrossReturn, test.wantReturn)
			assertClose(t, result.Path[1].Equity, test.wantFinalValue)
		})
	}
}

func TestRunUsesTheConfiguredInitialExposureUntilATargetExecutes(t *testing.T) {
	bars := testBars([2]float64{100, 100}, [2]float64{100, 110})
	targets := testTargets(bars, 0, 0)
	config := testConfig(ExecutionAtClose)
	config.ExecutionLag = 2
	config.InitialExposure = 1

	result, err := Run(bars, targets, config)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	assertClose(t, result.Path[0].ExecutedExposure, 1)
	assertClose(t, result.Path[1].ExecutedExposure, 1)
	assertClose(t, result.Path[1].Turnover, 0)
	assertClose(t, result.Path[1].GrossReturn, 0.10)
	assertClose(t, result.Path[1].Equity, 110_000)
}

func testConfig(timing ExecutionTiming) Config {
	config := DefaultConfig()
	config.ExecutionTiming = timing
	config.CommissionBPS = 0
	config.SlippageBPS = 0
	return config
}

func testBars(prices ...[2]float64) []MarketBar {
	start := time.Date(2025, time.January, 2, 0, 0, 0, 0, time.UTC)
	bars := make([]MarketBar, len(prices))
	for index, price := range prices {
		openPrice := price[0]
		closePrice := price[1]
		high := math.Max(openPrice, closePrice)
		low := math.Min(openPrice, closePrice)
		bars[index] = MarketBar{
			Timestamp: start.AddDate(0, 0, index),
			Open:      openPrice,
			High:      high,
			Low:       low,
			Close:     closePrice,
		}
	}
	return bars
}

func testTargets(bars []MarketBar, exposures ...float64) []Target {
	targets := make([]Target, len(bars))
	for index, bar := range bars {
		targets[index] = Target{Timestamp: bar.Timestamp, Exposure: exposures[index]}
	}
	return targets
}

func assertClose(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-10 {
		t.Fatalf("got %.12f, want %.12f", got, want)
	}
}
