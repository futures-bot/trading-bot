package strategy

import (
	"sync"
	"time"

	"trading-bot/internal/domain"

	"github.com/shopspring/decimal"
)

// TradeTracker holds the state of an active trade.
type TradeTracker struct {
	mutex                sync.RWMutex
	inTrade              bool
	IsClosed             bool
	entryPrice           decimal.Decimal
	takeProfit           decimal.Decimal
	stopLoss             decimal.Decimal
	Side                 domain.Signal
	lastTradeAttempt     time.Time
	minProfitBuffer      decimal.Decimal
	ExitReason           string
	cooldown             time.Duration
	takeProfitPct        float64
	stopLossPct          float64
	confirmationCount    int
	confirmationSignal   domain.Signal
	confirmationCounter  int
	minProfitForFlipExit float64
}

// ConfirmSignal confirms a signal and returns true if the confirmation count is reached.
func (t *TradeTracker) ConfirmSignal(signal domain.Signal) bool {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if signal == t.confirmationSignal {
		t.confirmationCounter++
	} else {
		t.confirmationSignal = signal
		t.confirmationCounter = 1
	}

	return t.confirmationCounter >= t.confirmationCount
}

// GetEntryPrice returns the entry price of the current trade.
func (t *TradeTracker) GetEntryPrice() decimal.Decimal {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	return t.entryPrice
}

// GetSide returns the side of the current trade.
func (t *TradeTracker) GetSide() domain.Signal {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	return t.Side
}

// NewTradeTracker creates a new TradeTracker.
func NewTradeTracker(takeProfitPct, stopLossPct float64, confirmationCount int, minProfitForFlipExit float64) *TradeTracker {
	return &TradeTracker{
		minProfitBuffer:      decimal.NewFromFloat(0.0015),
		cooldown:             5 * time.Minute, // This could be configurable too
		takeProfitPct:        takeProfitPct,
		stopLossPct:          stopLossPct,
		confirmationCount:    confirmationCount,
		confirmationCounter:  0,
		minProfitForFlipExit: minProfitForFlipExit,
	}
}

// StartTrade starts a new trade.
func (t *TradeTracker) StartTrade(entryPrice decimal.Decimal, side domain.Signal) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.inTrade = true
	t.IsClosed = false
	t.entryPrice = entryPrice
	t.Side = side

	if side == domain.SignalBuy {
		t.takeProfit = entryPrice.Mul(decimal.NewFromFloat(1).Add(decimal.NewFromFloat(t.takeProfitPct)))
		t.stopLoss = entryPrice.Mul(decimal.NewFromFloat(1).Sub(decimal.NewFromFloat(t.stopLossPct)))
	} else {
		t.takeProfit = entryPrice.Mul(decimal.NewFromFloat(1).Sub(decimal.NewFromFloat(t.takeProfitPct)))
		t.stopLoss = entryPrice.Mul(decimal.NewFromFloat(1).Add(decimal.NewFromFloat(t.stopLossPct)))
	}
}

func (t *TradeTracker) CanTrade() bool {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	return time.Since(t.lastTradeAttempt) > t.cooldown
}

func (t *TradeTracker) SetLastTradeAttempt() {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.lastTradeAttempt = time.Now()
}

// EndTrade ends the current trade.
func (t *TradeTracker) EndTrade() {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.inTrade = false
	t.IsClosed = true
	t.lastTradeAttempt = time.Now()
	t.confirmationCounter = 0
	t.confirmationSignal = domain.SignalHold
}

// CheckPrice checks if the current price has hit the take profit or stop loss.
func (t *TradeTracker) CheckPrice(currentPrice decimal.Decimal) string {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	if !t.inTrade {
		return ""
	}

	profit := currentPrice.Sub(t.entryPrice)
	if t.Side == domain.SignalSell {
		profit = t.entryPrice.Sub(currentPrice)
	}

	pnl := profit.Div(t.entryPrice)

	if t.Side == domain.SignalBuy {
		if currentPrice.GreaterThanOrEqual(t.takeProfit) && pnl.GreaterThan(t.minProfitBuffer) {
			t.ExitReason = "TAKE_PROFIT"
			return t.ExitReason
		} else if currentPrice.LessThanOrEqual(t.stopLoss) {
			t.ExitReason = "STOP_LOSS"
			return t.ExitReason
		}
	} else { // Handle SELL side logic
		if currentPrice.LessThanOrEqual(t.takeProfit) && pnl.GreaterThan(t.minProfitBuffer) {
			t.ExitReason = "TAKE_PROFIT"
			return t.ExitReason
		} else if currentPrice.GreaterThanOrEqual(t.stopLoss) {
			t.ExitReason = "STOP_LOSS"
			return t.ExitReason
		}
	}

	return ""
}

// InTrade returns true if there is an active trade.
func (t *TradeTracker) InTrade() bool {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	return t.inTrade
}

// ShouldExit determines if a trade should be exited based on the exit reason and PnL.
func (t *TradeTracker) ShouldExit(currentPrice decimal.Decimal, exitReason string) bool {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	if !t.inTrade {
		return false
	}

	if exitReason == "INDICATOR_FLIP" {
		profit := currentPrice.Sub(t.entryPrice)
		if t.Side == domain.SignalSell {
			profit = t.entryPrice.Sub(currentPrice)
		}

		pnlPct := profit.Div(t.entryPrice)
		minProfit := decimal.NewFromFloat(t.minProfitForFlipExit)

		return pnlPct.GreaterThanOrEqual(minProfit)
	}

	// For other reasons like TAKE_PROFIT or STOP_LOSS, we should always exit.
	return true
}

// CheckProfitAndLoss is a function that will check the profit and loss of a trade before closing it
func (t *TradeTracker) CheckProfitAndLoss(currentPrice decimal.Decimal) bool {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	if !t.inTrade {
		return true // Not in a trade, so we can technically exit
	}

	profit := currentPrice.Sub(t.entryPrice)
	if t.Side == domain.SignalSell {
		profit = t.entryPrice.Sub(currentPrice)
	}

	pnl := profit.Div(t.entryPrice)

	// If PnL is positive and above the buffer, allow exit.
	// If PnL is negative, it will be handled by the stop-loss logic.
	return pnl.GreaterThan(t.minProfitBuffer)
}
