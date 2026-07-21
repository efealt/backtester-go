package backtester

import (
	"math"
	"strings"
	"testing"
)

func TestDefaultConfigIsValidAndCausal(t *testing.T) {
	config := DefaultConfig()
	if config.ExecutionLag < 1 {
		t.Fatalf("DefaultConfig().ExecutionLag = %d, want positive", config.ExecutionLag)
	}
	if config.StartingCapital != 100_000 || config.CommissionBPS != 0 || config.SlippageBPS != 0 || config.PeriodsPerYear != 252 {
		t.Fatalf("DefaultConfig() = %+v, want neutral trading-cost assumptions", config)
	}
	if err := ValidateConfig(config); err != nil {
		t.Fatalf("ValidateConfig(DefaultConfig()) error = %v", err)
	}
}

func TestValidateConfigAcceptsCompleteExitPolicies(t *testing.T) {
	fixed := 0.03
	trailing := 0.05
	config := DefaultConfig()
	config.Exits = ExitPolicies{
		StopLoss: &StopLossPolicy{
			FixedPercent:              &fixed,
			TrailingPercent:           &trailing,
			Timing:                    ExitSameBar,
			TriggeredExposureFraction: 0.5,
			WaitForSignalChange:       true,
		},
		TakeProfit: &TakeProfitPolicy{
			Percent:                   0.10,
			Timing:                    ExitNextBar,
			TriggeredExposureFraction: 0,
		},
	}
	if err := ValidateConfig(config); err != nil {
		t.Fatalf("ValidateConfig() error = %v", err)
	}
}

func TestValidateConfigRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Config)
		want string
	}{
		{name: "capital", edit: func(c *Config) { c.StartingCapital = math.NaN() }, want: "starting capital"},
		{name: "initial exposure", edit: func(c *Config) { c.InitialExposure = math.Inf(1) }, want: "initial exposure"},
		{name: "lag", edit: func(c *Config) { c.ExecutionLag = 0 }, want: "execution lag"},
		{name: "execution timing", edit: func(c *Config) { c.ExecutionTiming = "intrabar" }, want: "execution timing"},
		{name: "annualization", edit: func(c *Config) { c.PeriodsPerYear = 0 }, want: "periods per year"},
		{name: "commission", edit: func(c *Config) { c.CommissionBPS = -1 }, want: "commission BPS"},
		{name: "slippage", edit: func(c *Config) { c.SlippageBPS = math.Inf(1) }, want: "slippage BPS"},
		{name: "cash rate", edit: func(c *Config) { c.CashAnnualRate = -1 }, want: "cash annual rate"},
		{name: "financing rate", edit: func(c *Config) { c.FinancingAnnualRate = -0.01 }, want: "financing annual rate"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := DefaultConfig()
			test.edit(&config)
			err := ValidateConfig(config)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ValidateConfig() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestValidateConfigRejectsInvalidExitPolicies(t *testing.T) {
	invalidLoss := 1.0
	tests := []struct {
		name  string
		exits ExitPolicies
		want  string
	}{
		{name: "empty stop", exits: ExitPolicies{StopLoss: &StopLossPolicy{}}, want: "requires a fixed or trailing"},
		{name: "loss percentage", exits: ExitPolicies{StopLoss: &StopLossPolicy{FixedPercent: &invalidLoss, Timing: ExitSameBar}}, want: "fixed stop-loss percentage"},
		{name: "stop timing", exits: ExitPolicies{StopLoss: stopPolicy(0.03, "later", 0)}, want: "stop loss: timing"},
		{name: "stop response", exits: ExitPolicies{StopLoss: stopPolicy(0.03, ExitSameBar, 1.1)}, want: "stop loss: triggered exposure fraction"},
		{name: "profit percentage", exits: ExitPolicies{TakeProfit: &TakeProfitPolicy{Timing: ExitSameBar}}, want: "take-profit percentage"},
		{name: "profit timing", exits: ExitPolicies{TakeProfit: &TakeProfitPolicy{Percent: 0.1, Timing: "later"}}, want: "take profit: timing"},
		{name: "profit response", exits: ExitPolicies{TakeProfit: &TakeProfitPolicy{Percent: 0.1, Timing: ExitNextBar, TriggeredExposureFraction: -0.1}}, want: "take profit: triggered exposure fraction"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := DefaultConfig()
			config.Exits = test.exits
			err := ValidateConfig(config)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ValidateConfig() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func stopPolicy(percent float64, timing ExitTiming, response float64) *StopLossPolicy {
	return &StopLossPolicy{
		FixedPercent:              &percent,
		Timing:                    timing,
		TriggeredExposureFraction: response,
	}
}
