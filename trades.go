package backtester

import (
	"math"
	"time"
)

type tradeActionKind uint8

const (
	tradeReturnAction tradeActionKind = iota
	tradeTransitionAction
	tradeFinancingAction
)

type tradeAction struct {
	kind              tradeActionKind
	barIndex          int
	timestamp         time.Time
	price             float64
	exposure          float64
	grossContribution float64
	fromExposure      float64
	toExposure        float64
	reason            TradeExitReason
}

type activeTrade struct {
	trade       Trade
	entryEquity float64
	lastHeldBar int
}

type tradeTracker struct {
	completed              []Trade
	completedEntryEquities []float64
	active                 *activeTrade
}

func (step *executionStep) addReturn(barIndex int, exposure, grossContribution float64) {
	if exposure == 0 {
		return
	}
	step.tradeActions = append(step.tradeActions, tradeAction{
		kind:              tradeReturnAction,
		barIndex:          barIndex,
		exposure:          exposure,
		grossContribution: grossContribution,
	})
}

func (step *executionStep) addTransition(
	timestamp time.Time,
	price float64,
	fromExposure float64,
	toExposure float64,
	reason TradeExitReason,
) {
	if fromExposure == toExposure {
		return
	}
	step.tradeActions = append(step.tradeActions, tradeAction{
		kind:         tradeTransitionAction,
		timestamp:    timestamp,
		price:        price,
		fromExposure: fromExposure,
		toExposure:   toExposure,
		reason:       reason,
	})
}

func (step *executionStep) addFinancing(exposure float64) {
	step.tradeActions = append(step.tradeActions, tradeAction{
		kind:     tradeFinancingAction,
		exposure: exposure,
	})
}

func newTradeTracker(firstBar MarketBar, initialExposure, startingEquity float64) tradeTracker {
	tracker := tradeTracker{completed: make([]Trade, 0)}
	if initialExposure != 0 {
		tracker.open(firstBar.Timestamp, firstBar.Close, initialExposure, startingEquity)
	}
	return tracker
}

func (tracker *tradeTracker) applyStep(
	step executionStep,
	accounting accountingStep,
	equityBefore float64,
	config Config,
) {
	equityAtAction := equityBefore + equityBefore*accounting.cashReturn - accounting.financingCost
	financingApplied := false
	for _, action := range step.tradeActions {
		switch action.kind {
		case tradeReturnAction:
			grossPnL := equityBefore * action.grossContribution
			tracker.applyReturn(action, grossPnL)
			equityAtAction += grossPnL
		case tradeTransitionAction:
			equityAtAction = tracker.applyTransition(action, equityBefore, equityAtAction, config)
		case tradeFinancingAction:
			if !financingApplied {
				financingApplied = true
				tracker.applyFinancing(action.exposure, accounting.financingCost)
			}
		}
	}

	actualPositionPnL := accounting.netPnL - equityBefore*accounting.cashReturn
	theoreticalPositionPnL := equityBefore*step.grossReturn - accounting.totalCost
	adjustment := actualPositionPnL - theoreticalPositionPnL
	if math.Abs(adjustment) > 1e-9 {
		tracker.applyRuinAdjustment(adjustment)
	}
}

func (tracker *tradeTracker) applyReturn(action tradeAction, grossPnL float64) {
	if tracker.active == nil || directionForExposure(action.exposure) != tracker.active.trade.Direction {
		return
	}
	tracker.active.trade.GrossPnL += grossPnL
	if tracker.active.lastHeldBar != action.barIndex {
		tracker.active.trade.BarsHeld++
		tracker.active.lastHeldBar = action.barIndex
	}
}

func (tracker *tradeTracker) applyTransition(
	action tradeAction,
	barEquity float64,
	transitionEquity float64,
	config Config,
) float64 {
	fromDirection := directionForExposure(action.fromExposure)
	toDirection := directionForExposure(action.toExposure)
	turnover := math.Abs(action.toExposure - action.fromExposure)
	cost := barEquity * turnover * (config.CommissionBPS + config.SlippageBPS) / 10_000

	switch {
	case fromDirection == DirectionFlat:
		tracker.open(action.timestamp, action.price, action.toExposure, transitionEquity)
		tracker.addCost(cost)
	case toDirection == DirectionFlat:
		tracker.addCost(cost)
		tracker.finish(action.timestamp, action.price, 0, action.reason)
	case fromDirection == toDirection:
		tracker.addCost(cost)
	default:
		closeTurnover := math.Abs(action.fromExposure)
		openTurnover := math.Abs(action.toExposure)
		totalTurnover := closeTurnover + openTurnover
		closeCost := 0.0
		if totalTurnover != 0 {
			closeCost = cost * closeTurnover / totalTurnover
		}
		tracker.addCost(closeCost)
		tracker.finish(action.timestamp, action.price, 0, action.reason)
		tracker.open(action.timestamp, action.price, action.toExposure, transitionEquity-closeCost)
		tracker.addCost(cost - closeCost)
	}
	return transitionEquity - cost
}

func (tracker *tradeTracker) applyFinancing(exposure, cost float64) {
	if tracker.active == nil || directionForExposure(exposure) != tracker.active.trade.Direction {
		return
	}
	tracker.addCost(cost)
}

func (tracker *tradeTracker) open(timestamp time.Time, price, exposure, equity float64) {
	if exposure == 0 {
		return
	}
	tracker.active = &activeTrade{
		trade: Trade{
			EntryTime:     timestamp,
			Direction:     directionForExposure(exposure),
			EntryPrice:    price,
			EntryExposure: exposure,
		},
		entryEquity: equity,
		lastHeldBar: -1,
	}
}

func (tracker *tradeTracker) addCost(cost float64) {
	if tracker.active != nil {
		tracker.active.trade.Costs += cost
	}
}

func (tracker *tradeTracker) finish(
	timestamp time.Time,
	price float64,
	exitExposure float64,
	reason TradeExitReason,
) {
	if tracker.active == nil {
		return
	}
	trade := tracker.active.trade
	trade.ExitTime = timePointer(timestamp)
	trade.ExitPrice = floatPointer(price)
	trade.ExitExposure = exitExposure
	trade.ExitReason = reason
	finalizeTrade(&trade, tracker.active.entryEquity)
	tracker.completed = append(tracker.completed, trade)
	tracker.completedEntryEquities = append(tracker.completedEntryEquities, tracker.active.entryEquity)
	tracker.active = nil
}

func (tracker *tradeTracker) closeForRuin(timestamp time.Time, price float64) {
	tracker.finish(timestamp, price, 0, TradeExitRuin)
}

func (tracker *tradeTracker) applyRuinAdjustment(adjustment float64) {
	if tracker.active != nil {
		tracker.active.trade.GrossPnL += adjustment
		return
	}
	if len(tracker.completed) == 0 {
		return
	}
	index := len(tracker.completed) - 1
	tracker.completed[index].GrossPnL += adjustment
	finalizeTrade(&tracker.completed[index], tracker.completedEntryEquities[index])
}

func (tracker tradeTracker) result(currentExposure float64) []Trade {
	trades := append([]Trade(nil), tracker.completed...)
	if tracker.active == nil {
		return trades
	}
	trade := tracker.active.trade
	trade.ExitExposure = currentExposure
	finalizeTrade(&trade, tracker.active.entryEquity)
	return append(trades, trade)
}

func finalizeTrade(trade *Trade, entryEquity float64) {
	trade.NetPnL = trade.GrossPnL - trade.Costs
	if entryEquity != 0 {
		trade.Return = trade.NetPnL / entryEquity
	}
}

func timePointer(value time.Time) *time.Time {
	copy := value
	return &copy
}
