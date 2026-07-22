package backtester

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"
)

// EvaluationSpec explicitly selects the optional analyses performed around one
// full-period primary run. A nil component disables that component.
type EvaluationSpec struct {
	References *ReferenceEvaluationSpec `json:"references,omitempty"`
	Scaling    *ScalingEvaluationSpec   `json:"scaling,omitempty"`
	Split      *ChronologicalSplitSpec  `json:"split,omitempty"`
	Folds      *ChronologicalFoldsSpec  `json:"folds,omitempty"`
}

// ReferenceEvaluationSpec declares ordered fixed-target reference scenarios.
type ReferenceEvaluationSpec struct {
	Items []FixedTargetReference `json:"items"`
}

// FixedTargetReference declares one named scenario whose target exposure is
// constant across all supplied bars. Config is complete and independent from
// the primary config.
type FixedTargetReference struct {
	Name           string  `json:"name"`
	TargetExposure float64 `json:"target_exposure"`
	Config         Config  `json:"config"`
}

// ScalingEvaluationSpec declares independent full-period primary target scales.
type ScalingEvaluationSpec struct {
	Multipliers []float64 `json:"multipliers"`
}

// ChronologicalSplitSpec declares the fraction of return intervals assigned to
// the first of two independent chronological comparisons.
type ChronologicalSplitSpec struct {
	FirstFraction float64 `json:"first_fraction"`
}

// ChronologicalFoldsSpec declares the exact number of independent chronological
// primary runs.
type ChronologicalFoldsSpec struct {
	Count int `json:"count"`
}

// Evaluation contains the primary full-period comparison and every explicitly
// requested optional analysis.
type Evaluation struct {
	FullPeriod Comparison          `json:"full_period"`
	Scaling    []ScaledResult      `json:"scaling,omitempty"`
	Split      *ChronologicalSplit `json:"split,omitempty"`
	Folds      []ChronologicalFold `json:"folds,omitempty"`
}

// Comparison contains one primary run and its ordered named references.
type Comparison struct {
	Primary    Result            `json:"primary"`
	References []ReferenceResult `json:"references,omitempty"`
}

// ReferenceResult identifies the result of one declared reference scenario.
type ReferenceResult struct {
	Name   string `json:"name"`
	Result Result `json:"result"`
}

// ScaledResult identifies one independently scaled primary run.
type ScaledResult struct {
	Multiplier float64 `json:"multiplier"`
	Result     Result  `json:"result"`
}

// ChronologicalSplit describes two independent comparisons whose bar slices
// share one boundary bar and whose return intervals do not overlap.
type ChronologicalSplit struct {
	FirstFraction                   float64    `json:"first_fraction"`
	TotalIntervals                  int        `json:"total_intervals"`
	FirstIntervals                  int        `json:"first_intervals"`
	SecondIntervals                 int        `json:"second_intervals"`
	BoundaryTimestamp               time.Time  `json:"boundary_timestamp"`
	FirstSecondIntervalEndTimestamp time.Time  `json:"first_second_interval_end_timestamp"`
	First                           Comparison `json:"first"`
	Second                          Comparison `json:"second"`
}

// ChronologicalFold is one independent primary run over a consecutive interval
// partition. Index is one-based.
type ChronologicalFold struct {
	Index          int       `json:"index"`
	StartTimestamp time.Time `json:"start_timestamp"`
	EndTimestamp   time.Time `json:"end_timestamp"`
	Result         Result    `json:"result"`
}

type preparedRun struct {
	bars    []MarketBar
	targets []Target
	config  Config
}

type preparedReference struct {
	name string
	run  preparedRun
}

type preparedComparison struct {
	primary    preparedRun
	references []preparedReference
}

type preparedScaledRun struct {
	multiplier float64
	run        preparedRun
}

type preparedSplit struct {
	firstFraction                   float64
	totalIntervals                  int
	firstIntervals                  int
	secondIntervals                 int
	boundaryTimestamp               time.Time
	firstSecondIntervalEndTimestamp time.Time
	first                           preparedComparison
	second                          preparedComparison
}

type preparedFold struct {
	index          int
	startTimestamp time.Time
	endTimestamp   time.Time
	run            preparedRun
}

type preparedEvaluation struct {
	fullPeriod preparedComparison
	scaling    []preparedScaledRun
	split      *preparedSplit
	folds      []preparedFold
}

// Evaluate runs one full-period primary backtest plus only the reference,
// scaling, chronological split, and fold analyses explicitly enabled by spec.
// All primary and derived inputs are validated before any accounting run starts.
// An error returns no partial evaluation.
func Evaluate(ctx context.Context, bars []MarketBar, targets []Target, config Config, spec EvaluationSpec) (Evaluation, error) {
	if ctx == nil {
		return Evaluation{}, fmt.Errorf("evaluation context must not be nil")
	}
	prepared, err := prepareEvaluation(bars, targets, config, spec)
	if err != nil {
		return Evaluation{}, err
	}
	if err := ctx.Err(); err != nil {
		return Evaluation{}, fmt.Errorf("evaluate: %w", err)
	}

	fullPeriod, err := executePreparedComparison(ctx, prepared.fullPeriod, "full-period")
	if err != nil {
		return Evaluation{}, err
	}
	evaluation := Evaluation{FullPeriod: fullPeriod}

	if spec.Scaling != nil {
		evaluation.Scaling = make([]ScaledResult, 0, len(prepared.scaling))
		for _, scaled := range prepared.scaling {
			result, err := executePreparedRun(ctx, scaled.run, fmt.Sprintf("scaling %.4gx", scaled.multiplier))
			if err != nil {
				return Evaluation{}, err
			}
			evaluation.Scaling = append(evaluation.Scaling, ScaledResult{Multiplier: scaled.multiplier, Result: result})
		}
	}

	if prepared.split != nil {
		first, err := executePreparedComparison(ctx, prepared.split.first, "split first")
		if err != nil {
			return Evaluation{}, err
		}
		second, err := executePreparedComparison(ctx, prepared.split.second, "split second")
		if err != nil {
			return Evaluation{}, err
		}
		evaluation.Split = &ChronologicalSplit{
			FirstFraction:                   prepared.split.firstFraction,
			TotalIntervals:                  prepared.split.totalIntervals,
			FirstIntervals:                  prepared.split.firstIntervals,
			SecondIntervals:                 prepared.split.secondIntervals,
			BoundaryTimestamp:               prepared.split.boundaryTimestamp,
			FirstSecondIntervalEndTimestamp: prepared.split.firstSecondIntervalEndTimestamp,
			First:                           first,
			Second:                          second,
		}
	}

	if spec.Folds != nil {
		evaluation.Folds = make([]ChronologicalFold, 0, len(prepared.folds))
		for _, fold := range prepared.folds {
			result, err := executePreparedRun(ctx, fold.run, fmt.Sprintf("fold %d", fold.index))
			if err != nil {
				return Evaluation{}, err
			}
			evaluation.Folds = append(evaluation.Folds, ChronologicalFold{
				Index:          fold.index,
				StartTimestamp: fold.startTimestamp,
				EndTimestamp:   fold.endTimestamp,
				Result:         result,
			})
		}
	}
	return evaluation, nil
}

func prepareEvaluation(bars []MarketBar, targets []Target, config Config, spec EvaluationSpec) (preparedEvaluation, error) {
	primary := preparedRun{bars: bars, targets: targets, config: config}
	if err := ValidateInputs(primary.bars, primary.targets, primary.config); err != nil {
		return preparedEvaluation{}, fmt.Errorf("evaluate primary inputs: %w", err)
	}

	references, err := prepareReferences(bars, spec.References)
	if err != nil {
		return preparedEvaluation{}, err
	}
	prepared := preparedEvaluation{
		fullPeriod: preparedComparison{primary: primary, references: references},
	}
	prepared.scaling, err = prepareScaling(bars, targets, config, spec.Scaling)
	if err != nil {
		return preparedEvaluation{}, err
	}
	prepared.split, err = prepareSplit(bars, targets, config, references, spec.Split)
	if err != nil {
		return preparedEvaluation{}, err
	}
	prepared.folds, err = prepareFolds(bars, targets, config, spec.Folds)
	if err != nil {
		return preparedEvaluation{}, err
	}
	return prepared, nil
}

func prepareReferences(bars []MarketBar, spec *ReferenceEvaluationSpec) ([]preparedReference, error) {
	if spec == nil {
		return nil, nil
	}
	if len(spec.Items) == 0 {
		return nil, fmt.Errorf("evaluation references must contain at least one item")
	}
	names := make(map[string]struct{}, len(spec.Items))
	references := make([]preparedReference, 0, len(spec.Items))
	for index, item := range spec.Items {
		if item.Name == "" || strings.TrimSpace(item.Name) != item.Name {
			return nil, fmt.Errorf("evaluation reference %d name must be non-empty without surrounding whitespace", index)
		}
		if _, exists := names[item.Name]; exists {
			return nil, fmt.Errorf("evaluation reference name %q is duplicated", item.Name)
		}
		names[item.Name] = struct{}{}
		if !isFinite(item.TargetExposure) {
			return nil, fmt.Errorf("evaluation reference %q target exposure must be finite", item.Name)
		}
		targets := fixedTargets(bars, item.TargetExposure)
		run := preparedRun{bars: bars, targets: targets, config: item.Config}
		if err := ValidateInputs(run.bars, run.targets, run.config); err != nil {
			return nil, fmt.Errorf("evaluation reference %q: %w", item.Name, err)
		}
		references = append(references, preparedReference{name: item.Name, run: run})
	}
	return references, nil
}

func prepareScaling(bars []MarketBar, targets []Target, config Config, spec *ScalingEvaluationSpec) ([]preparedScaledRun, error) {
	if spec == nil {
		return nil, nil
	}
	if len(spec.Multipliers) == 0 {
		return nil, fmt.Errorf("evaluation scaling must contain at least one multiplier")
	}
	seen := make(map[float64]struct{}, len(spec.Multipliers))
	scaling := make([]preparedScaledRun, 0, len(spec.Multipliers))
	for index, multiplier := range spec.Multipliers {
		if !isFinite(multiplier) || multiplier <= 0 {
			return nil, fmt.Errorf("evaluation scaling multiplier %d must be finite and positive", index)
		}
		if _, exists := seen[multiplier]; exists {
			return nil, fmt.Errorf("evaluation scaling multiplier %.4g is duplicated", multiplier)
		}
		seen[multiplier] = struct{}{}

		scaledTargets := make([]Target, len(targets))
		for targetIndex, target := range targets {
			exposure := target.Exposure * multiplier
			if !isFinite(exposure) {
				return nil, fmt.Errorf("evaluation scaling %.4gx produces non-finite target exposure at %d", multiplier, targetIndex)
			}
			scaledTargets[targetIndex] = Target{Timestamp: target.Timestamp, Exposure: exposure}
		}
		scaledConfig := config
		scaledConfig.InitialExposure *= multiplier
		if !isFinite(scaledConfig.InitialExposure) {
			return nil, fmt.Errorf("evaluation scaling %.4gx produces non-finite initial exposure", multiplier)
		}
		run := preparedRun{bars: bars, targets: scaledTargets, config: scaledConfig}
		if err := ValidateInputs(run.bars, run.targets, run.config); err != nil {
			return nil, fmt.Errorf("evaluation scaling %.4gx: %w", multiplier, err)
		}
		scaling = append(scaling, preparedScaledRun{multiplier: multiplier, run: run})
	}
	return scaling, nil
}

func prepareSplit(bars []MarketBar, targets []Target, config Config, references []preparedReference, spec *ChronologicalSplitSpec) (*preparedSplit, error) {
	if spec == nil {
		return nil, nil
	}
	if !isFinite(spec.FirstFraction) || spec.FirstFraction <= 0 || spec.FirstFraction >= 1 {
		return nil, fmt.Errorf("evaluation split first fraction must be finite and strictly between zero and one")
	}
	totalIntervals := len(bars) - 1
	firstIntervals := int(math.Floor(float64(totalIntervals) * spec.FirstFraction))
	if firstIntervals < 1 || firstIntervals >= totalIntervals {
		return nil, fmt.Errorf("evaluation split first fraction leaves an empty chronological partition")
	}

	firstPrimary := preparedRun{bars: bars[:firstIntervals+1], targets: targets[:firstIntervals+1], config: config}
	secondPrimary := preparedRun{bars: bars[firstIntervals:], targets: targets[firstIntervals:], config: config}
	if err := ValidateInputs(firstPrimary.bars, firstPrimary.targets, firstPrimary.config); err != nil {
		return nil, fmt.Errorf("evaluation split first primary: %w", err)
	}
	if err := ValidateInputs(secondPrimary.bars, secondPrimary.targets, secondPrimary.config); err != nil {
		return nil, fmt.Errorf("evaluation split second primary: %w", err)
	}

	firstReferences := make([]preparedReference, 0, len(references))
	secondReferences := make([]preparedReference, 0, len(references))
	for _, reference := range references {
		firstRun := preparedRun{
			bars:    reference.run.bars[:firstIntervals+1],
			targets: reference.run.targets[:firstIntervals+1],
			config:  reference.run.config,
		}
		secondRun := preparedRun{
			bars:    reference.run.bars[firstIntervals:],
			targets: reference.run.targets[firstIntervals:],
			config:  reference.run.config,
		}
		if err := ValidateInputs(firstRun.bars, firstRun.targets, firstRun.config); err != nil {
			return nil, fmt.Errorf("evaluation split first reference %q: %w", reference.name, err)
		}
		if err := ValidateInputs(secondRun.bars, secondRun.targets, secondRun.config); err != nil {
			return nil, fmt.Errorf("evaluation split second reference %q: %w", reference.name, err)
		}
		firstReferences = append(firstReferences, preparedReference{name: reference.name, run: firstRun})
		secondReferences = append(secondReferences, preparedReference{name: reference.name, run: secondRun})
	}

	return &preparedSplit{
		firstFraction:                   spec.FirstFraction,
		totalIntervals:                  totalIntervals,
		firstIntervals:                  firstIntervals,
		secondIntervals:                 totalIntervals - firstIntervals,
		boundaryTimestamp:               bars[firstIntervals].Timestamp,
		firstSecondIntervalEndTimestamp: bars[firstIntervals+1].Timestamp,
		first:                           preparedComparison{primary: firstPrimary, references: firstReferences},
		second:                          preparedComparison{primary: secondPrimary, references: secondReferences},
	}, nil
}

func prepareFolds(bars []MarketBar, targets []Target, config Config, spec *ChronologicalFoldsSpec) ([]preparedFold, error) {
	if spec == nil {
		return nil, nil
	}
	totalIntervals := len(bars) - 1
	if spec.Count < 1 {
		return nil, fmt.Errorf("evaluation fold count must be positive")
	}
	if spec.Count > totalIntervals {
		return nil, fmt.Errorf("evaluation fold count %d exceeds %d return intervals", spec.Count, totalIntervals)
	}
	folds := make([]preparedFold, 0, spec.Count)
	for index := 0; index < spec.Count; index++ {
		first := index * totalIntervals / spec.Count
		last := (index + 1) * totalIntervals / spec.Count
		run := preparedRun{bars: bars[first : last+1], targets: targets[first : last+1], config: config}
		if err := ValidateInputs(run.bars, run.targets, run.config); err != nil {
			return nil, fmt.Errorf("evaluation fold %d: %w", index+1, err)
		}
		folds = append(folds, preparedFold{
			index:          index + 1,
			startTimestamp: bars[first].Timestamp,
			endTimestamp:   bars[last].Timestamp,
			run:            run,
		})
	}
	return folds, nil
}

func executePreparedComparison(ctx context.Context, prepared preparedComparison, label string) (Comparison, error) {
	primary, err := executePreparedRun(ctx, prepared.primary, label+" primary")
	if err != nil {
		return Comparison{}, err
	}
	comparison := Comparison{Primary: primary}
	if len(prepared.references) > 0 {
		comparison.References = make([]ReferenceResult, 0, len(prepared.references))
		for _, reference := range prepared.references {
			result, err := executePreparedRun(ctx, reference.run, label+" reference "+reference.name)
			if err != nil {
				return Comparison{}, err
			}
			comparison.References = append(comparison.References, ReferenceResult{Name: reference.name, Result: result})
		}
	}
	return comparison, nil
}

func executePreparedRun(ctx context.Context, prepared preparedRun, label string) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, fmt.Errorf("evaluate %s: %w", label, err)
	}
	result := runValidated(prepared.bars, prepared.targets, prepared.config)
	if err := ctx.Err(); err != nil {
		return Result{}, fmt.Errorf("evaluate %s: %w", label, err)
	}
	return result, nil
}

func fixedTargets(bars []MarketBar, exposure float64) []Target {
	targets := make([]Target, len(bars))
	for index, bar := range bars {
		targets[index] = Target{Timestamp: bar.Timestamp, Exposure: exposure}
	}
	return targets
}
