package backtester

import "testing"

func TestFixedStopLossExitsLongAndShortPositions(t *testing.T) {
	tests := []struct {
		name      string
		exposure  float64
		bar       [4]float64
		direction PositionDirection
		level     float64
	}{
		{name: "long", exposure: 1, bar: [4]float64{100, 101, 94, 96}, direction: DirectionLong, level: 95},
		{name: "short", exposure: -1, bar: [4]float64{100, 106, 99, 104}, direction: DirectionShort, level: 105},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bars := exitTestBars([4]float64{100, 100, 100, 100}, test.bar)
			targets := testTargets(bars, test.exposure, test.exposure)
			config := testConfig(ExecutionAtOpen)
			config.Exits.StopLoss = fixedStop(0.05, ExitSameBar, 0, false)

			result, err := Run(bars, targets, config)
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			assertClose(t, result.Path[1].GrossReturn, -0.05)
			assertClose(t, result.Path[1].ExecutedExposure, 0)
			if result.Path[1].PositionDirection != DirectionFlat || result.Path[1].EntryPrice != nil {
				t.Fatalf("position after stop = %+v, want flat", result.Path[1])
			}
			assertExitEvent(t, result.ExitEvents, ExitKindStopLoss, ExitRuleFixed, ExitSameBar, test.direction, test.level, test.level, false, false, test.exposure, 0)
		})
	}
}

func TestTrailingStopLossExitsLongAndShortPositions(t *testing.T) {
	trailing := 0.05
	tests := []struct {
		name      string
		exposure  float64
		first     [4]float64
		second    [4]float64
		direction PositionDirection
		level     float64
	}{
		{name: "long", exposure: 1, first: [4]float64{100, 110, 100, 108}, second: [4]float64{108, 109, 104, 105}, direction: DirectionLong, level: 104.5},
		{name: "short", exposure: -1, first: [4]float64{100, 100, 90, 92}, second: [4]float64{92, 95, 91, 94}, direction: DirectionShort, level: 94.5},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bars := exitTestBars([4]float64{100, 100, 100, 100}, test.first, test.second)
			targets := testTargets(bars, test.exposure, test.exposure, test.exposure)
			config := testConfig(ExecutionAtOpen)
			config.Exits.StopLoss = &StopLossPolicy{TrailingPercent: &trailing, Timing: ExitSameBar}

			result, err := Run(bars, targets, config)
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			assertExitEvent(t, result.ExitEvents, ExitKindStopLoss, ExitRuleTrailing, ExitSameBar, test.direction, test.level, test.level, false, false, test.exposure, 0)
		})
	}
}

func TestStopLossRecordsWhenFixedAndTrailingRulesBothTrigger(t *testing.T) {
	fixed := 0.05
	trailing := 0.05
	bars := exitTestBars([4]float64{100, 100, 100, 100}, [4]float64{100, 101, 94, 96})
	targets := testTargets(bars, 1, 1)
	config := testConfig(ExecutionAtOpen)
	config.Exits.StopLoss = &StopLossPolicy{FixedPercent: &fixed, TrailingPercent: &trailing, Timing: ExitSameBar}

	result, err := Run(bars, targets, config)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	assertExitEvent(t, result.ExitEvents, ExitKindStopLoss, ExitRuleFixedAndTrailing, ExitSameBar, DirectionLong, 95, 95, false, false, 1, 0)
}

func fixedStop(percent float64, timing ExitTiming, fraction float64, wait bool) *StopLossPolicy {
	return &StopLossPolicy{
		FixedPercent:              &percent,
		Timing:                    timing,
		TriggeredExposureFraction: fraction,
		WaitForSignalChange:       wait,
	}
}
