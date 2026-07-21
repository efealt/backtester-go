package backtester

import "testing"

func TestRunTracksLongAndShortPositionStateAndActiveLevels(t *testing.T) {
	fixed := 0.10
	trailing := 0.05
	tests := []struct {
		name          string
		exposure      float64
		bar           [4]float64
		direction     PositionDirection
		fixedLevel    float64
		trailingLevel float64
		profitLevel   float64
	}{
		{name: "long", exposure: 1, bar: [4]float64{100, 110, 99, 108}, direction: DirectionLong, fixedLevel: 90, trailingLevel: 104.5, profitLevel: 120},
		{name: "short", exposure: -1, bar: [4]float64{100, 101, 90, 92}, direction: DirectionShort, fixedLevel: 110, trailingLevel: 94.5, profitLevel: 80},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bars := exitTestBars([4]float64{100, 100, 100, 100}, test.bar)
			targets := testTargets(bars, test.exposure, test.exposure)
			config := testConfig(ExecutionAtOpen)
			config.Exits = ExitPolicies{
				StopLoss:   &StopLossPolicy{FixedPercent: &fixed, TrailingPercent: &trailing, Timing: ExitSameBar},
				TakeProfit: &TakeProfitPolicy{Percent: 0.20, Timing: ExitSameBar},
			}

			result, err := Run(bars, targets, config)
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			point := result.Path[1]
			if point.PositionDirection != test.direction {
				t.Fatalf("position direction = %q, want %q", point.PositionDirection, test.direction)
			}
			assertOptionalLevel(t, "entry", point.EntryPrice, 100)
			assertOptionalLevel(t, "fixed stop", point.FixedStopLevel, test.fixedLevel)
			assertOptionalLevel(t, "trailing stop", point.TrailingStopLevel, test.trailingLevel)
			assertOptionalLevel(t, "take profit", point.TakeProfitLevel, test.profitLevel)
		})
	}
}

func assertOptionalLevel(t *testing.T, name string, got *float64, want float64) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s level is nil, want %.12f", name, want)
	}
	assertClose(t, *got, want)
}
