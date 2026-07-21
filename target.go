package backtester

import (
	"fmt"
	"time"
)

// Target is the exposure a strategy wants after observing the matching bar.
// Exposure is signed and unbounded: negative is short, zero is flat, positive
// is long, and magnitudes above one represent leverage.
type Target struct {
	// Timestamp must equal the timestamp of the MarketBar at the same slice index.
	Timestamp time.Time `json:"timestamp"`
	// Exposure is the signed position requested after observing the matching bar.
	// Magnitudes above one represent leverage.
	Exposure float64 `json:"exposure"`
}

// ValidateInputs checks all public inputs before a backtest run. Every target
// must match one bar exactly; Config.ExecutionLag then keeps its execution
// strictly after the observation that produced it.
func ValidateInputs(bars []MarketBar, targets []Target, config Config) error {
	if len(bars) < 2 {
		return fmt.Errorf("at least two market bars are required")
	}
	if err := ValidateMarketBars(bars); err != nil {
		return err
	}
	if len(targets) != len(bars) {
		return fmt.Errorf("target count %d does not match market bar count %d", len(targets), len(bars))
	}
	for index, target := range targets {
		if target.Timestamp.IsZero() {
			return fmt.Errorf("target %d timestamp is required", index)
		}
		if !target.Timestamp.Equal(bars[index].Timestamp) {
			return fmt.Errorf(
				"target %d timestamp %s does not match market bar timestamp %s",
				index,
				canonicalTimestamp(target.Timestamp),
				canonicalTimestamp(bars[index].Timestamp),
			)
		}
		if !isFinite(target.Exposure) {
			return fmt.Errorf("target %d exposure must be finite", index)
		}
	}
	if err := ValidateConfig(config); err != nil {
		return fmt.Errorf("config: %w", err)
	}
	if config.ExecutionTiming == ExecutionAtOpen {
		for index := 1; index < len(bars); index++ {
			if !hasOHLC(bars[index]) {
				return fmt.Errorf("market bar %d requires an open price for open execution", index)
			}
		}
	}
	return nil
}
