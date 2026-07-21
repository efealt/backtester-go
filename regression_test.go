package backtester

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"reflect"
	"strconv"
	"testing"
	"time"
)

func TestSyntheticCSVRegressionCoversCompleteResult(t *testing.T) {
	bars := loadRegressionBars(t, "testdata/synthetic_bars.csv")
	targets := make([]Target, len(bars))
	for index, bar := range bars {
		targets[index] = Target{Timestamp: bar.Timestamp, Exposure: 1}
	}
	stopPercent := 0.05
	config := Config{
		StartingCapital: 100_000,
		ExecutionLag:    1,
		ExecutionTiming: ExecutionAtOpen,
		CommissionBPS:   10,
		SlippageBPS:     20,
		PeriodsPerYear:  3,
		Exits: ExitPolicies{StopLoss: &StopLossPolicy{
			FixedPercent:        &stopPercent,
			Timing:              ExitSameBar,
			WaitForSignalChange: true,
		}},
	}

	got, err := Run(bars, targets, config)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	expectedJSON, err := os.ReadFile("testdata/synthetic_result.json")
	if err != nil {
		t.Fatalf("read expected result: %v", err)
	}
	var want Result
	if err := json.Unmarshal(expectedJSON, &want); err != nil {
		t.Fatalf("decode expected result: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		actualJSON, _ := json.MarshalIndent(got, "", "  ")
		t.Fatalf("complete result changed\nactual:\n%s\nexpected:\n%s", actualJSON, expectedJSON)
	}

	var pathNetPnL float64
	var tradeNetPnL float64
	var tradeCosts float64
	for _, point := range got.Path[1:] {
		pathNetPnL += point.NetPnL
	}
	for _, trade := range got.Trades {
		tradeNetPnL += trade.NetPnL
		tradeCosts += trade.Costs
	}
	assertClose(t, pathNetPnL, got.Path[len(got.Path)-1].Equity-config.StartingCapital)
	assertClose(t, tradeNetPnL, pathNetPnL)
	assertClose(t, tradeCosts, got.Metrics.TotalCosts)
}

func loadRegressionBars(t *testing.T, path string) []MarketBar {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer file.Close()
	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if len(records) < 2 || !reflect.DeepEqual(records[0], []string{"timestamp", "open", "high", "low", "close"}) {
		t.Fatalf("unexpected fixture header: %v", records[0])
	}

	bars := make([]MarketBar, 0, len(records)-1)
	for rowIndex, record := range records[1:] {
		if len(record) != 5 {
			t.Fatalf("fixture row %d has %d columns, want 5", rowIndex+2, len(record))
		}
		timestamp, err := time.Parse(time.RFC3339, record[0])
		if err != nil {
			t.Fatalf("fixture row %d timestamp: %v", rowIndex+2, err)
		}
		values := make([]float64, 4)
		for column := 1; column < len(record); column++ {
			values[column-1], err = strconv.ParseFloat(record[column], 64)
			if err != nil {
				t.Fatalf("fixture row %d column %d: %v", rowIndex+2, column+1, err)
			}
		}
		bars = append(bars, MarketBar{
			Timestamp: timestamp,
			Open:      values[0],
			High:      values[1],
			Low:       values[2],
			Close:     values[3],
		})
	}
	if len(bars) != 4 {
		t.Fatalf("fixture bars = %d, want 4", len(bars))
	}
	return bars
}
