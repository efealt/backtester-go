package backtester

import (
	"reflect"
	"testing"
)

func TestRunStopsDeterministicallyAtRuin(t *testing.T) {
	bars := testBars(
		[2]float64{100, 100},
		[2]float64{100, 200},
		[2]float64{200, 210},
	)
	targets := testTargets(bars, -2, -2, -2)
	config := testConfig(ExecutionAtOpen)

	first, err := Run(bars, targets, config)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	second, err := Run(bars, targets, config)
	if err != nil {
		t.Fatalf("second Run() error = %v", err)
	}

	if !reflect.DeepEqual(first, second) {
		t.Fatalf("identical inputs produced different results")
	}
	if !first.Ruined {
		t.Fatalf("Run().Ruined = false, want true")
	}
	if len(first.Path) != 2 {
		t.Fatalf("len(Run().Path) = %d, want 2", len(first.Path))
	}
	assertClose(t, first.Path[1].GrossReturn, -2)
	assertClose(t, first.Path[1].GrossPnL, -200_000)
	assertClose(t, first.Path[1].NetReturn, -1)
	assertClose(t, first.Path[1].NetPnL, -100_000)
	assertClose(t, first.Path[1].Equity, 0)
	assertClose(t, first.Path[1].Drawdown, -1)
}
