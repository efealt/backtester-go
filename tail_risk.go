package backtester

import (
	"math"
	"sort"
)

func summarizeTailRisk(returns []float64) (worstPeriod, valueAtRisk95, expectedShortfall95 float64) {
	if len(returns) == 0 {
		return 0, 0, 0
	}
	ordered := append([]float64(nil), returns...)
	sort.Float64s(ordered)
	worstPeriod = ordered[0]
	tailCount := maxInt(1, int(math.Ceil(float64(len(ordered))*0.05)))
	valueAtRisk95 = math.Max(0, -ordered[tailCount-1])
	tailMean := mean(ordered[:tailCount])
	expectedShortfall95 = math.Max(0, -tailMean)
	return worstPeriod, valueAtRisk95, expectedShortfall95
}

func worstRollingYear(path []PathPoint, periodsPerYear float64) *float64 {
	window := maxInt(1, int(math.Round(periodsPerYear)))
	if len(path)-1 < window {
		return nil
	}
	worst := math.Inf(1)
	for end := window; end < len(path); end++ {
		startEquity := path[end-window].Equity
		rollingReturn := -1.0
		if startEquity > 0 {
			rollingReturn = path[end].Equity/startEquity - 1
		}
		worst = math.Min(worst, rollingReturn)
	}
	return floatPointer(worst)
}
