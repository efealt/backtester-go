package backtester

import (
	"testing"
	"time"
)

func TestCloseOnlyDataEvaluatesExitsAtObservedClose(t *testing.T) {
	bars := closeOnlyExitTestBars(100, 94)
	targets := testTargets(bars, 1, 1)
	config := testConfig(ExecutionAtClose)
	config.InitialExposure = 1
	config.Exits.StopLoss = fixedStop(0.05, ExitSameBar, 0, false)

	result, err := Run(bars, targets, config)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	assertClose(t, result.Path[1].GrossReturn, -0.06)
	assertExitEvent(t, result.ExitEvents, ExitKindStopLoss, ExitRuleFixed, ExitSameBar, DirectionLong, 95, 94, false, false, 1, 0)
}

func TestCloseOnlyDataEvaluatesTakeProfitAtObservedClose(t *testing.T) {
	bars := closeOnlyExitTestBars(100, 94)
	targets := testTargets(bars, -1, -1)
	config := testConfig(ExecutionAtClose)
	config.InitialExposure = -1
	config.Exits.TakeProfit = &TakeProfitPolicy{Percent: 0.05, Timing: ExitSameBar}

	result, err := Run(bars, targets, config)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	assertClose(t, result.Path[1].GrossReturn, 0.06)
	assertExitEvent(t, result.ExitEvents, ExitKindTakeProfit, ExitRuleFixedPercentage, ExitSameBar, DirectionShort, 95, 94, false, false, -1, 0)
}

func TestCloseOnlyNextBarExitUsesTheNextObservedClose(t *testing.T) {
	bars := closeOnlyExitTestBars(100, 94, 90)
	targets := testTargets(bars, 1, 1, 1)
	config := testConfig(ExecutionAtClose)
	config.InitialExposure = 1
	config.Exits.StopLoss = fixedStop(0.05, ExitNextBar, 0, false)

	result, err := Run(bars, targets, config)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	assertClose(t, result.Path[1].ExecutedExposure, 1)
	assertClose(t, result.Path[2].GrossReturn, 90.0/94.0-1)
	assertClose(t, result.Path[2].ExecutedExposure, 0)
	assertExitEvent(t, result.ExitEvents, ExitKindStopLoss, ExitRuleFixed, ExitNextBar, DirectionLong, 95, 90, false, false, 1, 0)
	if !result.ExitEvents[0].Timestamp.Equal(bars[1].Timestamp) {
		t.Fatalf("exit decision timestamp = %s, want %s", result.ExitEvents[0].Timestamp, bars[1].Timestamp)
	}
}

func TestNextBarExitUsesNextOpenAndRealizableGapPrice(t *testing.T) {
	bars := exitTestBars(
		[4]float64{100, 100, 100, 100},
		[4]float64{100, 101, 94, 96},
		[4]float64{90, 92, 88, 91},
	)
	targets := testTargets(bars, 1, 1, 1)
	config := testConfig(ExecutionAtClose)
	config.InitialExposure = 1
	config.Exits.StopLoss = fixedStop(0.05, ExitNextBar, 0, false)

	result, err := Run(bars, targets, config)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(result.ExitEvents) != 1 {
		t.Fatalf("exit event count = %d, want 1", len(result.ExitEvents))
	}
	assertClose(t, result.Path[1].ExecutedExposure, 1)
	assertClose(t, result.Path[2].GrossReturn, 90.0/96.0-1)
	assertClose(t, result.Path[2].ExecutedExposure, 0)
	assertExitEvent(t, result.ExitEvents, ExitKindStopLoss, ExitRuleFixed, ExitNextBar, DirectionLong, 95, 90, true, false, 1, 0)
	if !result.ExitEvents[0].Timestamp.Equal(bars[1].Timestamp) {
		t.Fatalf("exit decision timestamp = %s, want %s", result.ExitEvents[0].Timestamp, bars[1].Timestamp)
	}
}

func TestGapFillsUseTheOpenForEveryDirectionAndExitKind(t *testing.T) {
	tests := []struct {
		name      string
		exposure  float64
		bar       [4]float64
		kind      ExitKind
		direction PositionDirection
		level     float64
		fill      float64
		configure func(*Config)
	}{
		{name: "long stop", exposure: 1, bar: [4]float64{90, 92, 89, 91}, kind: ExitKindStopLoss, direction: DirectionLong, level: 95, fill: 90, configure: func(c *Config) { c.Exits.StopLoss = fixedStop(0.05, ExitSameBar, 0, false) }},
		{name: "short stop", exposure: -1, bar: [4]float64{110, 111, 108, 109}, kind: ExitKindStopLoss, direction: DirectionShort, level: 105, fill: 110, configure: func(c *Config) { c.Exits.StopLoss = fixedStop(0.05, ExitSameBar, 0, false) }},
		{name: "long profit", exposure: 1, bar: [4]float64{110, 112, 109, 111}, kind: ExitKindTakeProfit, direction: DirectionLong, level: 105, fill: 110, configure: func(c *Config) { c.Exits.TakeProfit = &TakeProfitPolicy{Percent: 0.05, Timing: ExitSameBar} }},
		{name: "short profit", exposure: -1, bar: [4]float64{90, 91, 88, 89}, kind: ExitKindTakeProfit, direction: DirectionShort, level: 95, fill: 90, configure: func(c *Config) { c.Exits.TakeProfit = &TakeProfitPolicy{Percent: 0.05, Timing: ExitSameBar} }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bars := exitTestBars([4]float64{100, 100, 100, 100}, test.bar)
			targets := testTargets(bars, test.exposure, test.exposure)
			config := testConfig(ExecutionAtClose)
			config.InitialExposure = test.exposure
			test.configure(&config)

			result, err := Run(bars, targets, config)
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			assertExitEvent(t, result.ExitEvents, test.kind, expectedRule(test.kind), ExitSameBar, test.direction, test.level, test.fill, true, false, test.exposure, 0)
		})
	}
}

func TestStopLossWinsWhenStopAndTakeProfitCollide(t *testing.T) {
	bars := exitTestBars([4]float64{100, 100, 100, 100}, [4]float64{100, 111, 94, 100})
	targets := testTargets(bars, 1, 1)
	config := testConfig(ExecutionAtClose)
	config.InitialExposure = 1
	config.Exits.StopLoss = fixedStop(0.05, ExitSameBar, 0, false)
	config.Exits.TakeProfit = &TakeProfitPolicy{Percent: 0.10, Timing: ExitSameBar}

	result, err := Run(bars, targets, config)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	assertExitEvent(t, result.ExitEvents, ExitKindStopLoss, ExitRuleFixed, ExitSameBar, DirectionLong, 95, 95, false, true, 1, 0)
}

func TestExitCanLeavePartialExposure(t *testing.T) {
	tests := []struct {
		name              string
		bars              []MarketBar
		configure         func(*Config)
		kind              ExitKind
		rule              ExitRule
		activeLevel       float64
		grossReturns      []float64
		wantFixedLevel    float64
		wantTrailingLevel float64
		wantProfitLevel   float64
	}{
		{
			name: "stop loss retains prior trailing watermark",
			bars: exitTestBars(
				[4]float64{100, 100, 100, 100},
				[4]float64{100, 110, 100, 108},
				[4]float64{108, 108, 98, 100},
				[4]float64{100, 105, 100, 104},
			),
			configure: func(config *Config) {
				fixed := 0.05
				trailing := 0.10
				config.Exits.StopLoss = &StopLossPolicy{
					FixedPercent:              &fixed,
					TrailingPercent:           &trailing,
					Timing:                    ExitSameBar,
					TriggeredExposureFraction: 0.5,
					WaitForSignalChange:       true,
				}
				config.Exits.TakeProfit = &TakeProfitPolicy{Percent: 0.20, Timing: ExitSameBar}
			},
			kind:              ExitKindStopLoss,
			rule:              ExitRuleTrailing,
			activeLevel:       99,
			grossReturns:      []float64{0.08, 99.0/108.0 - 1 + 0.5*(100.0/99.0-1), 0.5 * (104.0/100.0 - 1)},
			wantFixedLevel:    95,
			wantTrailingLevel: 99,
			wantProfitLevel:   120,
		},
		{
			name: "take profit retains entry based anchors",
			bars: exitTestBars(
				[4]float64{100, 100, 100, 100},
				[4]float64{100, 104, 99, 103},
				[4]float64{103, 106, 100, 104},
				[4]float64{104, 104, 100, 102},
			),
			configure: func(config *Config) {
				fixed := 0.10
				trailing := 0.10
				config.Exits.StopLoss = &StopLossPolicy{
					FixedPercent:    &fixed,
					TrailingPercent: &trailing,
					Timing:          ExitSameBar,
				}
				config.Exits.TakeProfit = &TakeProfitPolicy{
					Percent:                   0.05,
					Timing:                    ExitSameBar,
					TriggeredExposureFraction: 0.5,
					WaitForSignalChange:       true,
				}
			},
			kind:              ExitKindTakeProfit,
			rule:              ExitRuleFixedPercentage,
			activeLevel:       105,
			grossReturns:      []float64{0.03, 105.0/103.0 - 1 + 0.5*(104.0/105.0-1), 0.5 * (102.0/104.0 - 1)},
			wantFixedLevel:    90,
			wantTrailingLevel: 95.4,
			wantProfitLevel:   105,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			targets := testTargets(test.bars, 1, 1, 1, 1)
			config := testConfig(ExecutionAtClose)
			config.InitialExposure = 1
			config.CommissionBPS = 100
			test.configure(&config)

			result, err := Run(test.bars, targets, config)
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}

			const triggerIndex = 2
			triggerPoint := result.Path[triggerIndex]
			finalPoint := result.Path[len(result.Path)-1]
			assertClose(t, triggerPoint.ExecutedExposure, 0.5)
			assertClose(t, triggerPoint.Turnover, 0.5)
			assertOptionalLevel(t, "trigger entry", triggerPoint.EntryPrice, 100)
			assertOptionalLevel(t, "trigger fixed stop", triggerPoint.FixedStopLevel, test.wantFixedLevel)
			assertOptionalLevel(t, "trigger trailing stop", triggerPoint.TrailingStopLevel, test.wantTrailingLevel)
			assertOptionalLevel(t, "trigger take profit", triggerPoint.TakeProfitLevel, test.wantProfitLevel)
			assertOptionalLevel(t, "final entry", finalPoint.EntryPrice, 100)
			assertOptionalLevel(t, "fixed stop", finalPoint.FixedStopLevel, test.wantFixedLevel)
			assertOptionalLevel(t, "trailing stop", finalPoint.TrailingStopLevel, test.wantTrailingLevel)
			assertOptionalLevel(t, "take profit", finalPoint.TakeProfitLevel, test.wantProfitLevel)
			assertClose(t, finalPoint.ExecutedExposure, 0.5)
			assertExitEvent(t, result.ExitEvents, test.kind, test.rule, ExitSameBar, DirectionLong, test.activeLevel, test.activeLevel, false, false, 1, 0.5)

			if len(result.Trades) != 1 {
				t.Fatalf("trade count = %d, want 1", len(result.Trades))
			}
			trade := result.Trades[0]
			if trade.ExitTime != nil || trade.ExitPrice != nil || trade.ExitReason != "" {
				t.Fatalf("partial terminal trade = %+v, want open", trade)
			}
			assertClose(t, trade.EntryPrice, 100)
			assertClose(t, trade.EntryExposure, 1)
			assertClose(t, trade.ExitExposure, 0.5)

			equity := config.StartingCapital
			var wantGrossPnL float64
			var wantCosts float64
			for index, grossReturn := range test.grossReturns {
				grossPnL := equity * grossReturn
				cost := 0.0
				if index+1 == triggerIndex {
					cost = equity * 0.5 * config.CommissionBPS / 10_000
				}
				wantGrossPnL += grossPnL
				wantCosts += cost
				equity += grossPnL - cost
			}
			assertClose(t, trade.GrossPnL, wantGrossPnL)
			assertClose(t, trade.Costs, wantCosts)
			assertClose(t, trade.NetPnL, wantGrossPnL-wantCosts)
			assertClose(t, finalPoint.Equity, equity)

			var pathCosts float64
			for _, point := range result.Path {
				pathCosts += point.TotalCost
			}
			assertClose(t, trade.Costs, pathCosts)
		})
	}
}

func TestWaitForSignalChangeControlsReentry(t *testing.T) {
	bars := exitTestBars(
		[4]float64{100, 100, 100, 100},
		[4]float64{100, 101, 94, 96},
		[4]float64{96, 98, 96, 97},
		[4]float64{97, 99, 97, 98},
		[4]float64{98, 100, 98, 99},
	)

	t.Run("wait", func(t *testing.T) {
		targets := testTargets(bars, 1, 1, 0, 1, 1)
		config := testConfig(ExecutionAtOpen)
		config.InitialExposure = 1
		config.Exits.StopLoss = fixedStop(0.05, ExitSameBar, 0, true)
		result, err := Run(bars, targets, config)
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		want := []float64{1, 0, 0, 0, 1}
		for index, exposure := range want {
			assertClose(t, result.Path[index].ExecutedExposure, exposure)
		}
	})

	t.Run("immediate reentry allowed", func(t *testing.T) {
		targets := testTargets(bars, 1, 1, 1, 1, 1)
		config := testConfig(ExecutionAtOpen)
		config.InitialExposure = 1
		config.Exits.StopLoss = fixedStop(0.05, ExitSameBar, 0, false)
		result, err := Run(bars, targets, config)
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		assertClose(t, result.Path[1].ExecutedExposure, 0)
		assertClose(t, result.Path[2].ExecutedExposure, 1)
	})
}

func exitTestBars(values ...[4]float64) []MarketBar {
	start := time.Date(2025, time.January, 2, 0, 0, 0, 0, time.UTC)
	bars := make([]MarketBar, len(values))
	for index, value := range values {
		bars[index] = MarketBar{
			Timestamp: start.AddDate(0, 0, index),
			Open:      value[0],
			High:      value[1],
			Low:       value[2],
			Close:     value[3],
		}
	}
	return bars
}

func closeOnlyExitTestBars(closes ...float64) []MarketBar {
	start := time.Date(2025, time.January, 2, 0, 0, 0, 0, time.UTC)
	bars := make([]MarketBar, len(closes))
	for index, closePrice := range closes {
		bars[index] = MarketBar{Timestamp: start.AddDate(0, 0, index), Close: closePrice}
	}
	return bars
}

func expectedRule(kind ExitKind) ExitRule {
	if kind == ExitKindTakeProfit {
		return ExitRuleFixedPercentage
	}
	return ExitRuleFixed
}

func assertExitEvent(
	t *testing.T,
	events []ExitEvent,
	kind ExitKind,
	rule ExitRule,
	timing ExitTiming,
	direction PositionDirection,
	level float64,
	fill float64,
	gap bool,
	collision bool,
	previousExposure float64,
	responseExposure float64,
) {
	t.Helper()
	if len(events) != 1 {
		t.Fatalf("exit event count = %d, want 1", len(events))
	}
	event := events[0]
	if event.Timestamp.IsZero() {
		t.Fatalf("exit timestamp is zero")
	}
	if event.Kind != kind || event.Rule != rule || event.Timing != timing || event.Direction != direction {
		t.Fatalf("exit identity = %+v", event)
	}
	assertClose(t, event.ActiveLevel, level)
	assertClose(t, event.FillPrice, fill)
	assertClose(t, event.PreviousExposure, previousExposure)
	assertClose(t, event.ResponseExposure, responseExposure)
	if event.GapFill != gap || event.Collision != collision {
		t.Fatalf("exit flags = gap %v collision %v, want gap %v collision %v", event.GapFill, event.Collision, gap, collision)
	}
}
