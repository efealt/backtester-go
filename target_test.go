package backtester

import (
	"math"
	"strings"
	"testing"
	"time"
)

func TestValidateInputsAcceptsAlignedLeveragedTargets(t *testing.T) {
	bars := validBars()
	targets := []Target{
		{Timestamp: bars[0].Timestamp, Exposure: -2.5},
		{Timestamp: bars[1].Timestamp, Exposure: 3.0},
	}
	if err := ValidateInputs(bars, targets, DefaultConfig()); err != nil {
		t.Fatalf("ValidateInputs() error = %v", err)
	}
}

func TestValidateInputsAcceptsCloseOnlyBarsForCloseExecution(t *testing.T) {
	bars := validBars()
	for index := range bars {
		bars[index].Open = 0
		bars[index].High = 0
		bars[index].Low = 0
	}
	if err := ValidateInputs(bars, targetsFor(bars), DefaultConfig()); err != nil {
		t.Fatalf("ValidateInputs() error = %v", err)
	}
}

func TestValidateInputsRejectsCloseOnlyBarsForOpenExecution(t *testing.T) {
	bars := validBars()
	for index := range bars {
		bars[index].Open = 0
		bars[index].High = 0
		bars[index].Low = 0
	}
	config := DefaultConfig()
	config.ExecutionTiming = ExecutionAtOpen
	err := ValidateInputs(bars, targetsFor(bars), config)
	if err == nil || !strings.Contains(err.Error(), "requires an open price") {
		t.Fatalf("ValidateInputs() error = %v, want OHLC requirement", err)
	}
}

func TestValidateInputsAcceptsCloseOnlyBarsWithExitPolicies(t *testing.T) {
	bars := validBars()
	for index := range bars {
		bars[index].Open = 0
		bars[index].High = 0
		bars[index].Low = 0
	}
	fixed := 0.03
	config := DefaultConfig()
	config.Exits.StopLoss = &StopLossPolicy{
		FixedPercent: &fixed,
		Timing:       ExitSameBar,
	}
	if err := ValidateInputs(bars, targetsFor(bars), config); err != nil {
		t.Fatalf("ValidateInputs() error = %v", err)
	}
}

func TestValidateInputsRejectsMalformedOrFutureUnsafeInputs(t *testing.T) {
	tests := []struct {
		name    string
		prepare func() ([]MarketBar, []Target, Config)
		want    string
	}{
		{
			name: "too few bars",
			prepare: func() ([]MarketBar, []Target, Config) {
				bars := validBars()[:1]
				return bars, targetsFor(bars), DefaultConfig()
			},
			want: "at least two market bars",
		},
		{
			name: "target count mismatch",
			prepare: func() ([]MarketBar, []Target, Config) {
				bars := validBars()
				return bars, targetsFor(bars)[:1], DefaultConfig()
			},
			want: "target count 1 does not match market bar count 2",
		},
		{
			name: "target timestamp mismatch",
			prepare: func() ([]MarketBar, []Target, Config) {
				bars := validBars()
				targets := targetsFor(bars)
				targets[1].Timestamp = targets[1].Timestamp.Add(time.Hour)
				return bars, targets, DefaultConfig()
			},
			want: "does not match market bar timestamp",
		},
		{
			name: "non finite exposure",
			prepare: func() ([]MarketBar, []Target, Config) {
				bars := validBars()
				targets := targetsFor(bars)
				targets[1].Exposure = math.Inf(1)
				return bars, targets, DefaultConfig()
			},
			want: "exposure must be finite",
		},
		{
			name: "same bar execution",
			prepare: func() ([]MarketBar, []Target, Config) {
				bars := validBars()
				config := DefaultConfig()
				config.ExecutionLag = 0
				return bars, targetsFor(bars), config
			},
			want: "execution lag must be at least one bar",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bars, targets, config := test.prepare()
			err := ValidateInputs(bars, targets, config)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ValidateInputs() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func targetsFor(bars []MarketBar) []Target {
	targets := make([]Target, len(bars))
	for index, bar := range bars {
		targets[index] = Target{Timestamp: bar.Timestamp}
	}
	return targets
}
