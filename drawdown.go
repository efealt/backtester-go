package backtester

import "math"

func updateDrawdown(peak, equity float64) (float64, float64) {
	if equity > peak {
		peak = equity
	}
	if peak == 0 {
		return peak, -1
	}
	return peak, equity/peak - 1
}

func summarizeDrawdowns(path []PathPoint) (maximum float64, duration int, recovery int, ulcer float64) {
	if len(path) < 2 {
		return 0, 0, 0, 0
	}
	peak := path[0].Equity
	peakForMaximum := peak
	troughIndex := -1
	underwaterDuration := 0
	squaredDrawdowns := 0.0

	for index := 1; index < len(path); index++ {
		equity := path[index].Equity
		if equity >= peak {
			peak = equity
			underwaterDuration = 0
		} else {
			underwaterDuration++
			duration = maxInt(duration, underwaterDuration)
		}
		drawdown := 0.0
		if peak > 0 {
			drawdown = equity/peak - 1
		}
		squaredDrawdowns += drawdown * drawdown
		if drawdown < maximum {
			maximum = drawdown
			peakForMaximum = peak
			troughIndex = index
		}
	}

	ulcer = math.Sqrt(squaredDrawdowns / float64(len(path)-1))
	if troughIndex < 0 {
		return maximum, duration, 0, ulcer
	}
	recovery = -1
	for index := troughIndex + 1; index < len(path); index++ {
		if path[index].Equity >= peakForMaximum {
			recovery = index - troughIndex
			break
		}
	}
	return maximum, duration, recovery, ulcer
}
