package backtester

import (
	"math"
	"testing"
)

func TestSummarizeDrawdownsMatchesHandCalculatedRecovery(t *testing.T) {
	path := []PathPoint{{Equity: 100}, {Equity: 80}, {Equity: 90}, {Equity: 100}}
	maximum, duration, recovery, ulcer := summarizeDrawdowns(path)

	assertClose(t, maximum, -0.20)
	if duration != 2 {
		t.Fatalf("duration = %d, want 2", duration)
	}
	if recovery != 2 {
		t.Fatalf("recovery = %d, want 2", recovery)
	}
	assertClose(t, ulcer, math.Sqrt((0.04+0.01)/3))
}
