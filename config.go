package backtester

import "fmt"

// ExecutionTiming selects the price within the execution bar.
type ExecutionTiming string

const (
	// ExecutionAtOpen executes a due target at the execution bar's open.
	ExecutionAtOpen ExecutionTiming = "open"
	// ExecutionAtClose executes a due target at the execution bar's close.
	ExecutionAtClose ExecutionTiming = "close"
)

// ExitTiming controls when an exit response changes exposure. Same-bar exits
// fill at the active level or a realizable gap-open price. Next-bar exits fill
// at the next open when OHLC data is present, otherwise at the next close.
type ExitTiming string

const (
	// ExitSameBar applies an exit response on the bar that triggers it.
	ExitSameBar ExitTiming = "same_bar"
	// ExitNextBar applies an exit response on the bar after it triggers.
	ExitNextBar ExitTiming = "next_bar"
)

// StopLossPolicy configures optional fixed and trailing stop losses.
// Percentages are positive fractions: 0.03 means three percent. A triggered
// exposure fraction of zero exits fully; 0.5 retains half of the exposure.
type StopLossPolicy struct {
	// FixedPercent is the fractional loss from entry that activates the fixed
	// stop. A nil value disables the fixed rule.
	FixedPercent *float64 `json:"fixed_percent,omitempty"`
	// TrailingPercent is the fractional reversal from the favorable price
	// watermark that activates the trailing stop. A nil value disables the
	// trailing rule.
	TrailingPercent *float64 `json:"trailing_percent,omitempty"`
	// Timing selects whether the response is applied on the trigger bar or the
	// following bar.
	Timing ExitTiming `json:"timing"`
	// TriggeredExposureFraction is the fraction of the current exposure retained
	// after the stop response. It must be between zero and one.
	TriggeredExposureFraction float64 `json:"triggered_exposure_fraction"`
	// WaitForSignalChange blocks restoration of the triggering target until the
	// caller supplies a different target exposure.
	WaitForSignalChange bool `json:"wait_for_signal_change"`
}

// TakeProfitPolicy configures a percentage take-profit exit. A triggered
// exposure fraction of zero exits fully; 0.5 retains half of the exposure.
type TakeProfitPolicy struct {
	// Percent is the positive fractional gain from entry that activates the
	// take-profit rule.
	Percent float64 `json:"percent"`
	// Timing selects whether the response is applied on the trigger bar or the
	// following bar.
	Timing ExitTiming `json:"timing"`
	// TriggeredExposureFraction is the fraction of the current exposure retained
	// after the take-profit response. It must be between zero and one.
	TriggeredExposureFraction float64 `json:"triggered_exposure_fraction"`
	// WaitForSignalChange blocks restoration of the triggering target until the
	// caller supplies a different target exposure.
	WaitForSignalChange bool `json:"wait_for_signal_change"`
}

// ExitPolicies combines independent stop-loss and take-profit controls.
type ExitPolicies struct {
	// StopLoss enables stop-loss behavior when non-nil.
	StopLoss *StopLossPolicy `json:"stop_loss,omitempty"`
	// TakeProfit enables take-profit behavior when non-nil.
	TakeProfit *TakeProfitPolicy `json:"take_profit,omitempty"`
}

// Config contains all assumptions needed to execute and account for one run.
type Config struct {
	// StartingCapital is initial account equity in the caller's currency.
	StartingCapital float64 `json:"starting_capital"`
	// InitialExposure is held before the first delayed target executes. Exposure
	// is signed: -1 is fully short, 0 is flat, and 1 is fully long.
	InitialExposure float64 `json:"initial_exposure"`
	// ExecutionLag is the number of supplied bars between a target observation
	// and its execution. It must be at least one.
	ExecutionLag int `json:"execution_lag"`
	// ExecutionTiming selects the execution price within the execution bar.
	ExecutionTiming ExecutionTiming `json:"execution_timing"`
	// CommissionBPS is commission per unit of turnover in basis points.
	CommissionBPS float64 `json:"commission_bps"`
	// SlippageBPS is slippage per unit of turnover in basis points.
	SlippageBPS float64 `json:"slippage_bps"`
	// CashAnnualRate is the effective annual return on uninvested cash, expressed
	// as a decimal fraction.
	CashAnnualRate float64 `json:"cash_annual_rate"`
	// FinancingAnnualRate is the effective annual cost on absolute exposure above
	// one, expressed as a decimal fraction.
	FinancingAnnualRate float64 `json:"financing_annual_rate"`
	// PeriodsPerYear is the expected number of supplied bars in one year. It
	// converts annual rates to per-bar rates and annualizes result statistics.
	// For example: monthly=12, daily US markets=252, and four-hour 24/7=2190.
	PeriodsPerYear float64 `json:"periods_per_year"`
	// Exits contains optional stop-loss and take-profit policies.
	Exits ExitPolicies `json:"exits"`
}

// DefaultConfig returns causal daily-bar defaults. No target can execute on
// the same bar that produced it.
func DefaultConfig() Config {
	return Config{
		StartingCapital: 100_000,
		ExecutionLag:    1,
		ExecutionTiming: ExecutionAtClose,
		PeriodsPerYear:  252,
	}
}

// ValidateConfig rejects undefined, non-causal, or economically invalid run
// assumptions.
func ValidateConfig(config Config) error {
	if !isFinite(config.StartingCapital) || config.StartingCapital <= 0 {
		return fmt.Errorf("starting capital must be finite and positive")
	}
	if !isFinite(config.InitialExposure) {
		return fmt.Errorf("initial exposure must be finite")
	}
	if config.ExecutionLag < 1 {
		return fmt.Errorf("execution lag must be at least one bar")
	}
	if config.ExecutionTiming != ExecutionAtOpen && config.ExecutionTiming != ExecutionAtClose {
		return fmt.Errorf("execution timing must be %q or %q", ExecutionAtOpen, ExecutionAtClose)
	}
	if !isFinite(config.PeriodsPerYear) || config.PeriodsPerYear <= 0 {
		return fmt.Errorf("periods per year must be finite and positive")
	}
	if err := validateNonNegative("commission BPS", config.CommissionBPS); err != nil {
		return err
	}
	if err := validateNonNegative("slippage BPS", config.SlippageBPS); err != nil {
		return err
	}
	if !isFinite(config.CashAnnualRate) || config.CashAnnualRate <= -1 {
		return fmt.Errorf("cash annual rate must be finite and greater than -1")
	}
	if err := validateNonNegative("financing annual rate", config.FinancingAnnualRate); err != nil {
		return err
	}
	return validateExitPolicies(config.Exits)
}

func validateExitPolicies(policies ExitPolicies) error {
	if policy := policies.StopLoss; policy != nil {
		if policy.FixedPercent == nil && policy.TrailingPercent == nil {
			return fmt.Errorf("stop loss requires a fixed or trailing percentage")
		}
		if policy.FixedPercent != nil {
			if err := validateLossPercent("fixed stop-loss percentage", *policy.FixedPercent); err != nil {
				return err
			}
		}
		if policy.TrailingPercent != nil {
			if err := validateLossPercent("trailing stop-loss percentage", *policy.TrailingPercent); err != nil {
				return err
			}
		}
		if err := validateExitTiming(policy.Timing); err != nil {
			return fmt.Errorf("stop loss: %w", err)
		}
		if err := validateResponseExposure(policy.TriggeredExposureFraction); err != nil {
			return fmt.Errorf("stop loss: %w", err)
		}
	}
	if policy := policies.TakeProfit; policy != nil {
		if !isFinite(policy.Percent) || policy.Percent <= 0 {
			return fmt.Errorf("take-profit percentage must be finite and positive")
		}
		if err := validateExitTiming(policy.Timing); err != nil {
			return fmt.Errorf("take profit: %w", err)
		}
		if err := validateResponseExposure(policy.TriggeredExposureFraction); err != nil {
			return fmt.Errorf("take profit: %w", err)
		}
	}
	return nil
}

func validateLossPercent(name string, value float64) error {
	if !isFinite(value) || value <= 0 || value >= 1 {
		return fmt.Errorf("%s must be finite and between 0 and 1", name)
	}
	return nil
}

func validateExitTiming(timing ExitTiming) error {
	if timing != ExitSameBar && timing != ExitNextBar {
		return fmt.Errorf("timing must be %q or %q", ExitSameBar, ExitNextBar)
	}
	return nil
}

func validateResponseExposure(value float64) error {
	if !isFinite(value) || value < 0 || value > 1 {
		return fmt.Errorf("triggered exposure fraction must be finite and between 0 and 1")
	}
	return nil
}

func validateNonNegative(name string, value float64) error {
	if !isFinite(value) || value < 0 {
		return fmt.Errorf("%s must be finite and non-negative", name)
	}
	return nil
}
