package backtester

import "testing"

func TestTakeProfitExitsLongAndShortPositions(t *testing.T) {
	tests := []struct {
		name      string
		exposure  float64
		bar       [4]float64
		direction PositionDirection
		level     float64
	}{
		{name: "long", exposure: 1, bar: [4]float64{100, 111, 99, 109}, direction: DirectionLong, level: 110},
		{name: "short", exposure: -1, bar: [4]float64{100, 101, 89, 91}, direction: DirectionShort, level: 90},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bars := exitTestBars([4]float64{100, 100, 100, 100}, test.bar)
			targets := testTargets(bars, test.exposure, test.exposure)
			config := testConfig(ExecutionAtOpen)
			config.Exits.TakeProfit = &TakeProfitPolicy{Percent: 0.10, Timing: ExitSameBar}

			result, err := Run(bars, targets, config)
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			assertClose(t, result.Path[1].GrossReturn, 0.10)
			assertExitEvent(t, result.ExitEvents, ExitKindTakeProfit, ExitRuleFixedPercentage, ExitSameBar, test.direction, test.level, test.level, false, false, test.exposure, 0)
		})
	}
}
