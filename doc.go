// Package backtester executes timestamp-aligned target exposures against
// ordered market bars. It applies causal execution timing, trading costs,
// financing, optional exits, position and trade accounting, and performance
// metrics in one deterministic run.
//
// Callers supply market observations, strategy-produced targets, and a Config.
// A Target describes the exposure desired after observing its matching bar;
// Config.ExecutionLag determines the later bar on which that target executes.
package backtester
