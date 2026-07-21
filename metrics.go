package backtester

import "math"

func calculateMetrics(path []PathPoint, trades []Trade, config Config) Metrics {
	returns := pathReturns(path)
	observations := len(returns)
	metrics := Metrics{
		Observations: observations,
		TradeCount:   len(trades),
	}
	if len(path) > 0 && path[0].Equity > 0 {
		metrics.TotalReturn = path[len(path)-1].Equity/path[0].Equity - 1
	}
	if observations > 0 {
		if metrics.TotalReturn <= -1 {
			metrics.AnnualizedReturn = -1
		} else {
			metrics.AnnualizedReturn = math.Pow(1+metrics.TotalReturn, config.PeriodsPerYear/float64(observations)) - 1
		}
		periodMean := mean(returns)
		periodVolatility := sampleStandardDeviation(returns, periodMean)
		periodDownside := downsideDeviation(returns)
		annualizationScale := math.Sqrt(config.PeriodsPerYear)
		metrics.AnnualizedVolatility = periodVolatility * annualizationScale
		metrics.DownsideDeviation = periodDownside * annualizationScale
		if periodVolatility > 0 {
			metrics.Sharpe = periodMean / periodVolatility * annualizationScale
		}
		if periodDownside > 0 {
			metrics.Sortino = periodMean / periodDownside * annualizationScale
		}
	}

	metrics.MaximumDrawdown,
		metrics.MaximumDrawdownDuration,
		metrics.RecoveryTime,
		metrics.UlcerIndex = summarizeDrawdowns(path)
	if metrics.MaximumDrawdown < 0 {
		metrics.Calmar = metrics.AnnualizedReturn / math.Abs(metrics.MaximumDrawdown)
	}
	metrics.WorstRolling12MReturn = worstRollingYear(path, config.PeriodsPerYear)
	metrics.WorstPeriodReturn,
		metrics.ValueAtRisk95,
		metrics.ExpectedShortfall95 = summarizeTailRisk(returns)

	for index := 1; index < len(path); index++ {
		metrics.Turnover += path[index].Turnover
		metrics.TotalCosts += path[index].TotalCost
		metrics.AverageExposure += math.Abs(path[index].ReturnExposure)
	}
	if observations > 0 {
		metrics.AverageExposure /= float64(observations)
	}
	applyTradeMetrics(&metrics, trades)
	return metrics
}

func pathReturns(path []PathPoint) []float64 {
	if len(path) < 2 {
		return nil
	}
	returns := make([]float64, 0, len(path)-1)
	for index := 1; index < len(path); index++ {
		previousEquity := path[index-1].Equity
		if previousEquity <= 0 {
			break
		}
		returns = append(returns, path[index].Equity/previousEquity-1)
	}
	return returns
}

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func sampleStandardDeviation(values []float64, average float64) float64 {
	if len(values) < 2 {
		return 0
	}
	squared := 0.0
	for _, value := range values {
		difference := value - average
		squared += difference * difference
	}
	return math.Sqrt(squared / float64(len(values)-1))
}

func downsideDeviation(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	squared := 0.0
	for _, value := range values {
		if value < 0 {
			squared += value * value
		}
	}
	return math.Sqrt(squared / float64(len(values)))
}

func applyTradeMetrics(metrics *Metrics, trades []Trade) {
	completed := 0
	wins := 0
	losses := 0
	totalWin := 0.0
	totalLoss := 0.0
	for _, trade := range trades {
		if trade.ExitTime == nil {
			continue
		}
		completed++
		if trade.NetPnL > 0 {
			wins++
			totalWin += trade.NetPnL
		} else if trade.NetPnL < 0 {
			losses++
			totalLoss += -trade.NetPnL
		}
	}
	if completed > 0 {
		metrics.HitRate = float64(wins) / float64(completed)
	}
	if wins > 0 && losses > 0 {
		metrics.PayoffRatio = (totalWin / float64(wins)) / (totalLoss / float64(losses))
		metrics.ProfitFactor = totalWin / totalLoss
	}
}
