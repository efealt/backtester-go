package backtester

import (
	"fmt"
	"time"
)

// MarketBar is one caller-supplied market observation. Open, High, and Low may
// all be zero when only a closing-price series is available.
type MarketBar struct {
	// Timestamp identifies the observation and must increase strictly across the
	// input slice.
	Timestamp time.Time `json:"timestamp"`
	// Open is the bar's opening price. Open, High, and Low must either all be
	// positive or all be zero for a closing-price-only series.
	Open float64 `json:"open"`
	// High is the bar's highest price and must be consistent with Open, Low, and
	// Close when OHLC data is supplied.
	High float64 `json:"high"`
	// Low is the bar's lowest price and must be consistent with Open, High, and
	// Close when OHLC data is supplied.
	Low float64 `json:"low"`
	// Close is the required positive closing price.
	Close float64 `json:"close"`
}

// ValidateMarketBars checks that bars are usable and strictly chronological.
func ValidateMarketBars(bars []MarketBar) error {
	if len(bars) == 0 {
		return fmt.Errorf("market bars must not be empty")
	}
	for index, bar := range bars {
		if err := validateMarketBar(index, bar); err != nil {
			return err
		}
		if index > 0 && !bars[index-1].Timestamp.Before(bar.Timestamp) {
			return fmt.Errorf(
				"market bar timestamps must be strictly increasing: bar %d is %s before bar %d is %s",
				index-1,
				canonicalTimestamp(bars[index-1].Timestamp),
				index,
				canonicalTimestamp(bar.Timestamp),
			)
		}
	}
	return nil
}

func validateMarketBar(index int, bar MarketBar) error {
	if bar.Timestamp.IsZero() {
		return fmt.Errorf("market bar %d timestamp is required", index)
	}
	if !isFinite(bar.Close) || bar.Close <= 0 {
		return fmt.Errorf("market bar %d close price must be finite and positive", index)
	}
	if bar.Open == 0 && bar.High == 0 && bar.Low == 0 {
		return nil
	}
	prices := []struct {
		name  string
		value float64
	}{
		{name: "open", value: bar.Open},
		{name: "high", value: bar.High},
		{name: "low", value: bar.Low},
	}
	for _, price := range prices {
		if !isFinite(price.value) || price.value <= 0 {
			return fmt.Errorf("market bar %d %s price must be finite and positive", index, price.name)
		}
	}
	if bar.High < maxFloat(bar.Open, bar.Close, bar.Low) || bar.Low > minFloat(bar.Open, bar.Close, bar.High) {
		return fmt.Errorf("market bar %d has inconsistent OHLC prices", index)
	}
	return nil
}

func hasOHLC(bar MarketBar) bool {
	return bar.Open != 0 && bar.High != 0 && bar.Low != 0
}

func canonicalTimestamp(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}
