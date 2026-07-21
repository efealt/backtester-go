# Test fixtures

This directory contains only generic, deterministic backtest inputs and expected
outputs. Fixtures must not depend on a consuming application, private dataset,
network service, or local machine state.

`synthetic_bars.csv` is the fixed generic market path. Its paired
`synthetic_result.json` records the complete expected `Result` for the regression
configuration in `regression_test.go`.
