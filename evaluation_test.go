package backtester

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestEvaluateRunsEveryDeclaredComponentWithRunSemantics(t *testing.T) {
	bars := testBars(
		[2]float64{100, 100},
		[2]float64{100, 110},
		[2]float64{110, 90},
		[2]float64{90, 95},
		[2]float64{95, 80},
		[2]float64{80, 85},
	)
	targets := testTargets(bars, 1, 1, 1, 0, 1, 1)
	fixed := 0.05
	config := testConfig(ExecutionAtClose)
	config.InitialExposure = 0.5
	config.CommissionBPS = 1
	config.SlippageBPS = 2
	config.Exits.StopLoss = &StopLossPolicy{
		FixedPercent:              &fixed,
		Timing:                    ExitSameBar,
		TriggeredExposureFraction: 0,
		WaitForSignalChange:       true,
	}

	marketConfig := config
	marketConfig.InitialExposure = 1
	marketConfig.Exits = ExitPolicies{}
	cashConfig := config
	cashConfig.InitialExposure = 0
	cashConfig.Exits = ExitPolicies{}
	spec := EvaluationSpec{
		References: &ReferenceEvaluationSpec{Items: []FixedTargetReference{
			{Name: "market", TargetExposure: 1, Config: marketConfig},
			{Name: "cash", TargetExposure: 0, Config: cashConfig},
		}},
		Scaling: &ScalingEvaluationSpec{Multipliers: []float64{0.5, 2}},
		Split:   &ChronologicalSplitSpec{FirstFraction: 0.6},
		Folds:   &ChronologicalFoldsSpec{Count: 3},
	}

	evaluation, err := Evaluate(context.Background(), bars, targets, config, spec)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}

	expectedPrimary := mustRun(t, bars, targets, config)
	if !reflect.DeepEqual(evaluation.FullPeriod.Primary, expectedPrimary) {
		t.Fatal("full-period primary differs from Run")
	}
	if len(evaluation.FullPeriod.Primary.ExitEvents) == 0 || evaluation.FullPeriod.Primary.Metrics.TotalCosts <= 0 {
		t.Fatal("full-period primary did not preserve exits and costs")
	}
	if len(evaluation.FullPeriod.References) != 2 {
		t.Fatalf("reference count = %d, want 2", len(evaluation.FullPeriod.References))
	}
	expectedMarket := mustRun(t, bars, fixedTargets(bars, 1), marketConfig)
	expectedCash := mustRun(t, bars, fixedTargets(bars, 0), cashConfig)
	if evaluation.FullPeriod.References[0].Name != "market" || !reflect.DeepEqual(evaluation.FullPeriod.References[0].Result, expectedMarket) {
		t.Fatal("market reference differs from its independently configured Run")
	}
	if evaluation.FullPeriod.References[1].Name != "cash" || !reflect.DeepEqual(evaluation.FullPeriod.References[1].Result, expectedCash) {
		t.Fatal("cash reference differs from its independently configured Run")
	}

	if len(evaluation.Scaling) != 2 {
		t.Fatalf("scaling count = %d, want 2", len(evaluation.Scaling))
	}
	for index, multiplier := range []float64{0.5, 2} {
		scaledConfig := config
		scaledConfig.InitialExposure *= multiplier
		expected := mustRun(t, bars, scaleTargets(targets, multiplier), scaledConfig)
		if evaluation.Scaling[index].Multiplier != multiplier || !reflect.DeepEqual(evaluation.Scaling[index].Result, expected) {
			t.Fatalf("scaling result %d differs from Run", index)
		}
	}
	if evaluation.Scaling[1].Result.Path[0].ExecutedExposure != 1 {
		t.Fatalf("scaled initial exposure = %v, want 1", evaluation.Scaling[1].Result.Path[0].ExecutedExposure)
	}

	split := evaluation.Split
	if split == nil {
		t.Fatal("split is nil")
	}
	if split.TotalIntervals != 5 || split.FirstIntervals != 3 || split.SecondIntervals != 2 {
		t.Fatalf("split interval counts = %d/%d/%d", split.TotalIntervals, split.FirstIntervals, split.SecondIntervals)
	}
	if !split.BoundaryTimestamp.Equal(bars[3].Timestamp) || !split.FirstSecondIntervalEndTimestamp.Equal(bars[4].Timestamp) {
		t.Fatal("split timestamps do not identify the shared boundary and first second-partition interval")
	}
	if !reflect.DeepEqual(split.First.Primary, mustRun(t, bars[:4], targets[:4], config)) {
		t.Fatal("first split primary differs from Run")
	}
	if !reflect.DeepEqual(split.Second.Primary, mustRun(t, bars[3:], targets[3:], config)) {
		t.Fatal("second split primary differs from Run")
	}
	if !reflect.DeepEqual(split.First.References[0].Result, mustRun(t, bars[:4], fixedTargets(bars[:4], 1), marketConfig)) {
		t.Fatal("first split reference differs from Run")
	}
	if !reflect.DeepEqual(split.Second.References[1].Result, mustRun(t, bars[3:], fixedTargets(bars[3:], 0), cashConfig)) {
		t.Fatal("second split reference differs from Run")
	}

	if len(evaluation.Folds) != 3 {
		t.Fatalf("fold count = %d, want 3", len(evaluation.Folds))
	}
	totalFoldIntervals := 0
	for index, bounds := range [][2]int{{0, 1}, {1, 3}, {3, 5}} {
		fold := evaluation.Folds[index]
		if fold.Index != index+1 || !fold.StartTimestamp.Equal(bars[bounds[0]].Timestamp) || !fold.EndTimestamp.Equal(bars[bounds[1]].Timestamp) {
			t.Fatalf("fold %d metadata is incorrect", index+1)
		}
		expected := mustRun(t, bars[bounds[0]:bounds[1]+1], targets[bounds[0]:bounds[1]+1], config)
		if !reflect.DeepEqual(fold.Result, expected) {
			t.Fatalf("fold %d differs from Run", index+1)
		}
		totalFoldIntervals += fold.Result.Metrics.Observations
	}
	if totalFoldIntervals != evaluation.FullPeriod.Primary.Metrics.Observations {
		t.Fatalf("fold intervals = %d, want %d", totalFoldIntervals, evaluation.FullPeriod.Primary.Metrics.Observations)
	}
}

func TestEvaluateZeroSpecReturnsOnlyPrimary(t *testing.T) {
	bars := testBars([2]float64{100, 100}, [2]float64{100, 105})
	targets := testTargets(bars, 1, 1)
	evaluation, err := Evaluate(context.Background(), bars, targets, testConfig(ExecutionAtClose), EvaluationSpec{})
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if len(evaluation.FullPeriod.Primary.Path) != len(bars) || evaluation.FullPeriod.References != nil || evaluation.Scaling != nil || evaluation.Split != nil || evaluation.Folds != nil {
		t.Fatalf("zero-spec evaluation contains undeclared analyses: %#v", evaluation)
	}
	payload, err := json.Marshal(evaluation)
	if err != nil {
		t.Fatal(err)
	}
	for _, absent := range []string{"references", "scaling", "split", "folds"} {
		if strings.Contains(string(payload), "\""+absent+"\"") {
			t.Fatalf("zero-spec JSON unexpectedly contains %q", absent)
		}
	}
}

func TestEvaluatePreservesRuinedPaths(t *testing.T) {
	bars := testBars(
		[2]float64{100, 100},
		[2]float64{100, 200},
		[2]float64{200, 210},
	)
	targets := testTargets(bars, -2, -2, -2)
	evaluation, err := Evaluate(
		context.Background(),
		bars,
		targets,
		testConfig(ExecutionAtOpen),
		EvaluationSpec{Scaling: &ScalingEvaluationSpec{Multipliers: []float64{1}}},
	)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if !evaluation.FullPeriod.Primary.Ruined || !evaluation.Scaling[0].Result.Ruined {
		t.Fatal("ruin status was not preserved")
	}
	if !reflect.DeepEqual(evaluation.FullPeriod.Primary, evaluation.Scaling[0].Result) {
		t.Fatal("1x scaling differs from the primary ruined result")
	}
}

func TestEvaluateRejectsEveryIncompleteDeclaration(t *testing.T) {
	bars := testBars([2]float64{100, 100}, [2]float64{100, 101}, [2]float64{101, 102})
	targets := testTargets(bars, 1, 2, 1)
	config := testConfig(ExecutionAtClose)
	invalidConfig := config
	invalidConfig.ExecutionLag = 0

	tests := []struct {
		name string
		spec EvaluationSpec
	}{
		{name: "empty references", spec: EvaluationSpec{References: &ReferenceEvaluationSpec{}}},
		{name: "blank reference name", spec: EvaluationSpec{References: &ReferenceEvaluationSpec{Items: []FixedTargetReference{{Name: " ", Config: config}}}}},
		{name: "duplicate reference name", spec: EvaluationSpec{References: &ReferenceEvaluationSpec{Items: []FixedTargetReference{{Name: "same", Config: config}, {Name: "same", Config: config}}}}},
		{name: "non-finite reference", spec: EvaluationSpec{References: &ReferenceEvaluationSpec{Items: []FixedTargetReference{{Name: "bad", TargetExposure: math.NaN(), Config: config}}}}},
		{name: "invalid reference config", spec: EvaluationSpec{References: &ReferenceEvaluationSpec{Items: []FixedTargetReference{{Name: "bad", Config: invalidConfig}}}}},
		{name: "empty scaling", spec: EvaluationSpec{Scaling: &ScalingEvaluationSpec{}}},
		{name: "zero scaling", spec: EvaluationSpec{Scaling: &ScalingEvaluationSpec{Multipliers: []float64{0}}}},
		{name: "duplicate scaling", spec: EvaluationSpec{Scaling: &ScalingEvaluationSpec{Multipliers: []float64{2, 2}}}},
		{name: "overflow scaling", spec: EvaluationSpec{Scaling: &ScalingEvaluationSpec{Multipliers: []float64{math.MaxFloat64}}}},
		{name: "zero split", spec: EvaluationSpec{Split: &ChronologicalSplitSpec{}}},
		{name: "empty split partition", spec: EvaluationSpec{Split: &ChronologicalSplitSpec{FirstFraction: 0.1}}},
		{name: "zero folds", spec: EvaluationSpec{Folds: &ChronologicalFoldsSpec{}}},
		{name: "too many folds", spec: EvaluationSpec{Folds: &ChronologicalFoldsSpec{Count: 3}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := Evaluate(context.Background(), bars, targets, config, test.spec); err == nil {
				t.Fatal("Evaluate() accepted an invalid declaration")
			}
		})
	}

	if _, err := Evaluate(nil, bars, targets, config, EvaluationSpec{}); err == nil {
		t.Fatal("Evaluate() accepted a nil context")
	}
	badTargets := append([]Target(nil), targets...)
	badTargets[0].Timestamp = bars[1].Timestamp
	if _, err := Evaluate(context.Background(), bars, badTargets, config, EvaluationSpec{}); err == nil {
		t.Fatal("Evaluate() accepted invalid primary inputs")
	}
}

func TestEvaluateReturnsContextCancellationWithoutPartialResults(t *testing.T) {
	bars := testBars([2]float64{100, 100}, [2]float64{100, 101})
	targets := testTargets(bars, 1, 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	evaluation, err := Evaluate(ctx, bars, targets, testConfig(ExecutionAtClose), EvaluationSpec{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Evaluate() error = %v, want context cancellation", err)
	}
	if !reflect.DeepEqual(evaluation, Evaluation{}) {
		t.Fatalf("canceled evaluation returned partial results: %#v", evaluation)
	}
}

func mustRun(t *testing.T, bars []MarketBar, targets []Target, config Config) Result {
	t.Helper()
	result, err := Run(bars, targets, config)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	return result
}

func scaleTargets(targets []Target, multiplier float64) []Target {
	scaled := make([]Target, len(targets))
	for index, target := range targets {
		scaled[index] = Target{Timestamp: target.Timestamp, Exposure: target.Exposure * multiplier}
	}
	return scaled
}
