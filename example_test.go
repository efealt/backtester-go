package backtester_test

import (
	"fmt"
	"time"

	backtester "github.com/efealt/backtester-go"
)

func ExampleValidateInputs() {
	first := time.Date(2025, time.January, 2, 0, 0, 0, 0, time.UTC)
	second := first.AddDate(0, 0, 1)
	bars := []backtester.MarketBar{
		{Timestamp: first, Open: 100, High: 102, Low: 99, Close: 101},
		{Timestamp: second, Open: 101, High: 104, Low: 100, Close: 103},
	}
	targets := []backtester.Target{
		{Timestamp: first, Exposure: 0},
		{Timestamp: second, Exposure: 1},
	}

	fmt.Println(backtester.ValidateInputs(bars, targets, backtester.DefaultConfig()))
	// Output: <nil>
}

func ExampleRun_ruleBasedTargets() {
	bars := closeOnlyBars(100, 110, 121, 108.9)
	targets := make([]backtester.Target, len(bars))
	targets[0] = backtester.Target{Timestamp: bars[0].Timestamp, Exposure: 0}
	for index := 1; index < len(bars); index++ {
		exposure := -1.0
		if bars[index].Close > bars[index-1].Close {
			exposure = 1
		}
		targets[index] = backtester.Target{Timestamp: bars[index].Timestamp, Exposure: exposure}
	}

	config := backtester.DefaultConfig()
	config.ExecutionTiming = backtester.ExecutionAtClose
	config.PeriodsPerYear = 4
	result, err := backtester.Run(bars, targets, config)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("%.0f\n", result.Path[len(result.Path)-1].Equity)
	// Output: 90000
}

func ExampleRun_modelBasedTargets() {
	bars := closeOnlyBars(100, 100, 110)
	modelScores := []float64{0.80, 0.70, -0.60}
	targets := make([]backtester.Target, len(bars))
	for index, score := range modelScores {
		exposure := 0.0
		if score > 0.5 {
			exposure = 1
		} else if score < -0.5 {
			exposure = -1
		}
		targets[index] = backtester.Target{Timestamp: bars[index].Timestamp, Exposure: exposure}
	}

	config := backtester.DefaultConfig()
	config.ExecutionTiming = backtester.ExecutionAtClose
	result, err := backtester.Run(bars, targets, config)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("%.0f\n", result.Path[len(result.Path)-1].Equity)
	// Output: 110000
}

func ExampleRun_externallyGeneratedTargets() {
	bars := closeOnlyBars(100, 100, 90)
	externalTargets := []backtester.Target{
		{Timestamp: bars[0].Timestamp, Exposure: 1},
		{Timestamp: bars[1].Timestamp, Exposure: 1},
		{Timestamp: bars[2].Timestamp, Exposure: -1},
	}

	config := backtester.DefaultConfig()
	config.ExecutionTiming = backtester.ExecutionAtClose
	result, err := backtester.Run(bars, externalTargets, config)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("%.0f %d\n", result.Path[len(result.Path)-1].Equity, result.Metrics.TradeCount)
	// Output: 90000 1
}

func ExampleExitPolicies() {
	fixedStop := 0.05
	trailingStop := 0.10

	config := backtester.DefaultConfig()
	config.Exits = backtester.ExitPolicies{
		StopLoss: &backtester.StopLossPolicy{
			FixedPercent:              &fixedStop,
			TrailingPercent:           &trailingStop,
			Timing:                    backtester.ExitSameBar,
			TriggeredExposureFraction: 0,
			WaitForSignalChange:       true,
		},
		TakeProfit: &backtester.TakeProfitPolicy{
			Percent:                   0.20,
			Timing:                    backtester.ExitSameBar,
			TriggeredExposureFraction: 0,
			WaitForSignalChange:       true,
		},
	}

	first := time.Date(2025, time.January, 2, 0, 0, 0, 0, time.UTC)
	bars := []backtester.MarketBar{
		{Timestamp: first, Open: 100, High: 100, Low: 100, Close: 100},
		{Timestamp: first.AddDate(0, 0, 1), Open: 100, High: 100, Low: 100, Close: 100},
		{Timestamp: first.AddDate(0, 0, 2), Open: 100, High: 103, Low: 94, Close: 96},
	}
	targets := []backtester.Target{
		{Timestamp: bars[0].Timestamp, Exposure: 1},
		{Timestamp: bars[1].Timestamp, Exposure: 1},
		{Timestamp: bars[2].Timestamp, Exposure: 1},
	}

	result, err := backtester.Run(bars, targets, config)
	if err != nil {
		fmt.Println(err)
		return
	}
	if len(result.ExitEvents) != 1 {
		fmt.Printf("exit events: %d\n", len(result.ExitEvents))
		return
	}
	event := result.ExitEvents[0]
	fmt.Printf("%s %s %.2f %.0f\n", event.Kind, event.Rule, event.FillPrice, event.ResponseExposure)
	// Output: stop_loss fixed 95.00 0
}

func closeOnlyBars(closes ...float64) []backtester.MarketBar {
	first := time.Date(2025, time.January, 2, 0, 0, 0, 0, time.UTC)
	bars := make([]backtester.MarketBar, len(closes))
	for index, closePrice := range closes {
		bars[index] = backtester.MarketBar{
			Timestamp: first.AddDate(0, 0, index),
			Close:     closePrice,
		}
	}
	return bars
}
