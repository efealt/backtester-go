package backtester

import "time"

// PositionDirection is the sign of an active position.
type PositionDirection string

const (
	// DirectionFlat identifies zero exposure.
	DirectionFlat PositionDirection = "flat"
	// DirectionLong identifies positive exposure.
	DirectionLong PositionDirection = "long"
	// DirectionShort identifies negative exposure.
	DirectionShort PositionDirection = "short"
)

// ExitKind identifies the policy family that forced an exit.
type ExitKind string

const (
	// ExitKindStopLoss identifies an exit produced by a stop-loss policy.
	ExitKindStopLoss ExitKind = "stop_loss"
	// ExitKindTakeProfit identifies an exit produced by a take-profit policy.
	ExitKindTakeProfit ExitKind = "take_profit"
)

// ExitRule identifies the exact rule that fired.
type ExitRule string

const (
	// ExitRuleFixed identifies a fixed stop-loss trigger.
	ExitRuleFixed ExitRule = "fixed"
	// ExitRuleTrailing identifies a trailing stop-loss trigger.
	ExitRuleTrailing ExitRule = "trailing"
	// ExitRuleFixedAndTrailing identifies a bar where both stop-loss rules
	// triggered; the more protective level is selected.
	ExitRuleFixedAndTrailing ExitRule = "fixed_and_trailing"
	// ExitRuleFixedPercentage identifies a fixed-percentage take-profit trigger.
	ExitRuleFixedPercentage ExitRule = "fixed_percentage"
)

// TradeExitReason explains why a completed trade ended.
type TradeExitReason string

const (
	// TradeExitSignal identifies a flattening or reversal produced by a target.
	TradeExitSignal TradeExitReason = "signal"
	// TradeExitStopLoss identifies a trade closed by a stop-loss response.
	TradeExitStopLoss TradeExitReason = "stop_loss"
	// TradeExitTakeProfit identifies a trade closed by a take-profit response.
	TradeExitTakeProfit TradeExitReason = "take_profit"
	// TradeExitRuin identifies a trade closed when account equity reaches zero.
	TradeExitRuin TradeExitReason = "ruin"
)

// PathPoint is the complete accounting state for one market bar.
type PathPoint struct {
	// Timestamp is the matching market bar's timestamp.
	Timestamp time.Time `json:"timestamp"`
	// TargetExposure is the strategy target observed on this bar. Its execution
	// occurs on a later bar according to Config.ExecutionLag.
	TargetExposure float64 `json:"target_exposure"`
	// ExecutedExposure is the position exposure after every execution on this bar.
	ExecutedExposure float64 `json:"executed_exposure"`
	// ReturnExposure is the representative exposure held during this bar and is
	// used for average-exposure reporting. GrossReturn contains the exact
	// segmented return when exposure changes within the bar.
	ReturnExposure float64 `json:"return_exposure"`
	// PositionDirection is the direction after every execution on this bar.
	PositionDirection PositionDirection `json:"position_direction"`
	// EntryPrice is the active position's entry price after this bar, or nil when
	// flat.
	EntryPrice *float64 `json:"entry_price,omitempty"`
	// FixedStopLevel is the active fixed stop price after this bar, or nil when
	// inactive.
	FixedStopLevel *float64 `json:"fixed_stop_level,omitempty"`
	// TrailingStopLevel is the active trailing stop price after this bar, or nil
	// when inactive.
	TrailingStopLevel *float64 `json:"trailing_stop_level,omitempty"`
	// TakeProfitLevel is the active take-profit price after this bar, or nil when
	// inactive.
	TakeProfitLevel *float64 `json:"take_profit_level,omitempty"`
	// AssetReturn is the bar's unlevered close-to-close price return.
	AssetReturn float64 `json:"asset_return"`
	// GrossReturn is the exposure-weighted price return before cash yield and all
	// costs.
	GrossReturn float64 `json:"gross_return"`
	// GrossPnL is the currency PnL from GrossReturn plus CashReturn before costs.
	GrossPnL float64 `json:"gross_pnl"`
	// CashReturn is the decimal return earned on uninvested cash during the bar.
	CashReturn float64 `json:"cash_return"`
	// FinancingCost is the currency cost of absolute exposure above one.
	FinancingCost float64 `json:"financing_cost"`
	// CommissionCost is the currency commission charged for this bar's turnover.
	CommissionCost float64 `json:"commission_cost"`
	// SlippageCost is the currency slippage charged for this bar's turnover.
	SlippageCost float64 `json:"slippage_cost"`
	// TotalCost is FinancingCost plus CommissionCost plus SlippageCost.
	TotalCost float64 `json:"total_cost"`
	// NetReturn is the account's decimal return after cash yield and all costs.
	NetReturn float64 `json:"net_return"`
	// NetPnL is the currency change in account equity during this bar.
	NetPnL float64 `json:"net_pnl"`
	// Equity is account equity after this bar's return, cash yield, and costs.
	Equity float64 `json:"equity"`
	// Turnover is the sum of absolute exposure changes executed on this bar.
	Turnover float64 `json:"turnover"`
	// Drawdown is the non-positive decimal decline from the running equity peak.
	Drawdown float64 `json:"drawdown"`
}

// Trade records one complete position lifecycle. ExitTime and ExitPrice are
// nil while a terminal position remains open.
type Trade struct {
	// EntryTime is the timestamp at which exposure first moved away from zero.
	EntryTime time.Time `json:"entry_time"`
	// ExitTime is the timestamp at which the position flattened or reversed. It
	// is nil while the terminal position remains open.
	ExitTime *time.Time `json:"exit_time,omitempty"`
	// Direction is the sign of the position throughout this trade.
	Direction PositionDirection `json:"direction"`
	// EntryPrice is the execution price that opened the trade.
	EntryPrice float64 `json:"entry_price"`
	// ExitPrice is the execution price that closed the trade, or nil while open.
	ExitPrice *float64 `json:"exit_price,omitempty"`
	// EntryExposure is the signed exposure established when the trade opened.
	EntryExposure float64 `json:"entry_exposure"`
	// ExitExposure is zero for a completed trade and the final held exposure for
	// a terminal open trade.
	ExitExposure float64 `json:"exit_exposure"`
	// GrossPnL is exposure-weighted price PnL in the caller's currency before
	// trading and financing costs. It excludes cash yield.
	GrossPnL float64 `json:"gross_pnl"`
	// Costs is the trade's accumulated financing, commission, and slippage.
	Costs float64 `json:"costs"`
	// NetPnL is GrossPnL minus Costs.
	NetPnL float64 `json:"net_pnl"`
	// Return is NetPnL divided by account equity when the trade opened.
	Return float64 `json:"return"`
	// BarsHeld counts distinct input bars with a return segment attributed to the
	// trade.
	BarsHeld int `json:"bars_held"`
	// ExitReason identifies what closed the trade and is empty while it remains
	// open.
	ExitReason TradeExitReason `json:"exit_reason,omitempty"`
}

// ExitEvent records the factual stop-loss or take-profit decision applied by
// accounting. Timestamp is when the rule triggered; FillPrice is the realizable
// response price selected by Timing.
type ExitEvent struct {
	// Timestamp is when the exit rule triggered, including for a next-bar fill.
	Timestamp time.Time `json:"timestamp"`
	// Kind identifies whether stop loss or take profit triggered.
	Kind ExitKind `json:"kind"`
	// Rule identifies the exact fixed or trailing rule that triggered.
	Rule ExitRule `json:"rule"`
	// Timing is the configured response timing used for the event.
	Timing ExitTiming `json:"timing"`
	// Direction is the position direction when the rule triggered.
	Direction PositionDirection `json:"direction"`
	// ActiveLevel is the selected stop-loss or take-profit trigger price.
	ActiveLevel float64 `json:"active_level"`
	// FillPrice is the realizable price used to apply the exposure response.
	FillPrice float64 `json:"fill_price"`
	// GapFill reports whether an opening gap moved beyond ActiveLevel.
	GapFill bool `json:"gap_fill"`
	// Collision reports that stop-loss and take-profit rules crossed on the same
	// bar; stop loss takes precedence.
	Collision bool `json:"collision"`
	// PreviousExposure is the signed exposure immediately before the response.
	PreviousExposure float64 `json:"previous_exposure"`
	// ResponseExposure is the signed exposure immediately after the response.
	ResponseExposure float64 `json:"response_exposure"`
}

// Metrics summarizes the path and trades from one run. Undefined ratios are
// reported as zero so every value remains finite and JSON-safe.
type Metrics struct {
	// TotalReturn is final equity divided by starting equity minus one.
	TotalReturn float64 `json:"total_return"`
	// AnnualizedReturn is compounded TotalReturn annualized by PeriodsPerYear.
	AnnualizedReturn float64 `json:"annualized_return"`
	// AnnualizedVolatility is sample standard deviation of equity returns scaled
	// by the square root of PeriodsPerYear.
	AnnualizedVolatility float64 `json:"annualized_volatility"`
	// DownsideDeviation is the annualized root mean square of negative equity
	// returns, using all observations in the denominator.
	DownsideDeviation float64 `json:"downside_deviation"`
	// Sharpe is annualized mean equity return divided by sample volatility, with
	// a zero-return threshold.
	Sharpe float64 `json:"sharpe"`
	// Sortino is annualized mean equity return divided by DownsideDeviation, with
	// a zero-return threshold.
	Sortino float64 `json:"sortino"`
	// Calmar is AnnualizedReturn divided by the absolute MaximumDrawdown.
	Calmar float64 `json:"calmar"`
	// MaximumDrawdown is the most negative decline from a running equity peak.
	MaximumDrawdown float64 `json:"maximum_drawdown"`
	// MaximumDrawdownDuration is the longest consecutive number of underwater
	// observations.
	MaximumDrawdownDuration int `json:"maximum_drawdown_duration"`
	// RecoveryTime is the number of observations from the maximum-drawdown trough
	// until its prior peak is recovered. It is -1 when unrecovered.
	RecoveryTime int `json:"recovery_time"`
	// UlcerIndex is the root mean square of path drawdowns.
	UlcerIndex float64 `json:"ulcer_index"`
	// WorstRolling12MReturn is the lowest compounded return across the nearest
	// whole-number PeriodsPerYear window. It is nil when the path is too short.
	WorstRolling12MReturn *float64 `json:"worst_rolling_12_month_return,omitempty"`
	// WorstPeriodReturn is the lowest single-period equity return.
	WorstPeriodReturn float64 `json:"worst_period_return"`
	// ValueAtRisk95 is the non-negative historical loss magnitude at the boundary
	// of the worst five percent of equity returns.
	ValueAtRisk95 float64 `json:"value_at_risk_95"`
	// ExpectedShortfall95 is the non-negative mean loss magnitude across the worst
	// five percent of equity returns.
	ExpectedShortfall95 float64 `json:"expected_shortfall_95"`
	// Turnover is total absolute exposure change across the path.
	Turnover float64 `json:"turnover"`
	// TotalCosts is total financing, commission, and slippage in caller currency.
	TotalCosts float64 `json:"total_costs"`
	// AverageExposure is mean absolute ReturnExposure across observations.
	AverageExposure float64 `json:"average_exposure"`
	// HitRate is profitable completed trades divided by all completed trades.
	HitRate float64 `json:"hit_rate"`
	// PayoffRatio is average winning NetPnL divided by average losing magnitude
	// across completed trades.
	PayoffRatio float64 `json:"payoff_ratio"`
	// ProfitFactor is total winning NetPnL divided by total losing magnitude
	// across completed trades.
	ProfitFactor float64 `json:"profit_factor"`
	// Observations is the number of consecutive equity returns in the path.
	Observations int `json:"observations"`
	// TradeCount includes completed trades and any terminal open trade.
	TradeCount int `json:"trade_count"`
}

// Result contains every deterministic output from one backtest run.
type Result struct {
	// Path contains one accounting point per processed market bar.
	Path []PathPoint `json:"path"`
	// Trades contains completed trades and any position still open at the end.
	Trades []Trade `json:"trades"`
	// ExitEvents contains every stop-loss and take-profit response.
	ExitEvents []ExitEvent `json:"exit_events"`
	// Metrics summarizes Path and Trades.
	Metrics Metrics `json:"metrics"`
	// Ruined reports that equity reached zero and processing terminated.
	Ruined bool `json:"ruined"`
}
