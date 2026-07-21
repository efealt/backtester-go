package backtester

import "testing"

func TestSummarizeTailRiskUsesWorstFivePercent(t *testing.T) {
	returns := make([]float64, 40)
	returns[0] = -0.40
	returns[1] = -0.20

	worst, valueAtRisk95, expectedShortfall95 := summarizeTailRisk(returns)

	assertClose(t, worst, -0.40)
	assertClose(t, valueAtRisk95, 0.20)
	assertClose(t, expectedShortfall95, 0.30)
}
