package backtester

import "math"

type accountingStep struct {
	grossPnL       float64
	cashReturn     float64
	financingCost  float64
	commissionCost float64
	slippageCost   float64
	totalCost      float64
	netReturn      float64
	netPnL         float64
	equity         float64
	ruined         bool
}

func accountBar(equity float64, execution executionStep, config Config) accountingStep {
	cashPeriodRate := annualToPeriod(config.CashAnnualRate, config.PeriodsPerYear)
	financingPeriodRate := annualToPeriod(config.FinancingAnnualRate, config.PeriodsPerYear)
	cashWeight := math.Max(1-math.Abs(execution.accrualExposure), 0)
	borrowedWeight := math.Max(math.Abs(execution.accrualExposure)-1, 0)
	cashReturn := cashWeight * cashPeriodRate
	financingRate := borrowedWeight * financingPeriodRate
	commissionRate := execution.turnover * config.CommissionBPS / 10_000
	slippageRate := execution.turnover * config.SlippageBPS / 10_000
	netReturn := execution.grossReturn + cashReturn - financingRate - commissionRate - slippageRate

	step := accountingStep{
		grossPnL:       equity * (execution.grossReturn + cashReturn),
		cashReturn:     cashReturn,
		financingCost:  equity * financingRate,
		commissionCost: equity * commissionRate,
		slippageCost:   equity * slippageRate,
		netReturn:      netReturn,
	}
	step.totalCost = step.financingCost + step.commissionCost + step.slippageCost
	step.equity = equity * (1 + netReturn)
	if !isFinite(step.equity) || step.equity <= 0 {
		step.equity = 0
		step.netReturn = -1
		step.ruined = true
	}
	step.netPnL = step.equity - equity
	return step
}

func annualToPeriod(annualRate, periodsPerYear float64) float64 {
	if annualRate == 0 {
		return 0
	}
	return math.Pow(1+annualRate, 1/periodsPerYear) - 1
}
