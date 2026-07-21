package backtester

import (
	"math"
	"strings"
	"testing"
	"time"
)

func TestValidateMarketBarsAcceptsOrderedFiniteOHLC(t *testing.T) {
	if err := ValidateMarketBars(validBars()); err != nil {
		t.Fatalf("ValidateMarketBars() error = %v", err)
	}
}

func TestValidateMarketBarsAcceptsCloseOnlyData(t *testing.T) {
	bars := validBars()
	for index := range bars {
		bars[index].Open = 0
		bars[index].High = 0
		bars[index].Low = 0
	}
	if err := ValidateMarketBars(bars); err != nil {
		t.Fatalf("ValidateMarketBars() error = %v", err)
	}
}

func TestValidateMarketBarsRejectsMalformedBars(t *testing.T) {
	tests := []struct {
		name string
		edit func([]MarketBar) []MarketBar
		want string
	}{
		{name: "empty", edit: func([]MarketBar) []MarketBar { return nil }, want: "must not be empty"},
		{name: "zero timestamp", edit: func(b []MarketBar) []MarketBar { b[0].Timestamp = time.Time{}; return b }, want: "timestamp is required"},
		{name: "non finite price", edit: func(b []MarketBar) []MarketBar { b[0].Close = math.NaN(); return b }, want: "close price must be finite and positive"},
		{name: "non positive price", edit: func(b []MarketBar) []MarketBar { b[0].Close = 0; return b }, want: "close price must be finite and positive"},
		{name: "partial OHLC", edit: func(b []MarketBar) []MarketBar { b[0].Open = 0; return b }, want: "open price must be finite and positive"},
		{name: "bad high", edit: func(b []MarketBar) []MarketBar { b[0].High = 100; return b }, want: "inconsistent OHLC"},
		{name: "bad low", edit: func(b []MarketBar) []MarketBar { b[0].Low = 102; return b }, want: "inconsistent OHLC"},
		{name: "duplicate timestamp", edit: func(b []MarketBar) []MarketBar { b[1].Timestamp = b[0].Timestamp; return b }, want: "strictly increasing"},
		{name: "descending timestamp", edit: func(b []MarketBar) []MarketBar { b[1].Timestamp = b[0].Timestamp.Add(-time.Hour); return b }, want: "strictly increasing"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateMarketBars(test.edit(validBars()))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ValidateMarketBars() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func validBars() []MarketBar {
	first := time.Date(2025, time.January, 2, 0, 0, 0, 0, time.UTC)
	return []MarketBar{
		{Timestamp: first, Open: 100, High: 103, Low: 99, Close: 102},
		{Timestamp: first.AddDate(0, 0, 1), Open: 102, High: 104, Low: 101, Close: 103},
	}
}
