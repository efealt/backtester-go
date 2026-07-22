// Package backtester executes timestamp-aligned target exposures against
// ordered market bars. It applies causal execution timing, trading costs,
// financing, optional exits, position and trade accounting, and performance
// metrics in one deterministic run.
//
// Callers supply market observations, strategy-produced targets, and a Config.
// A Target describes the exposure desired after observing its matching bar;
// Config.ExecutionLag determines the later bar on which that target executes.
//
// Run executes one validated accounting path. Evaluate executes the same path
// for one primary rule plus only the caller-declared fixed-target references,
// target scales, chronological split, and chronological folds. Evaluate does
// not infer scenario names, configs, defaults, strategy semantics, or whether a
// result is evidence of an edge.
package backtester
