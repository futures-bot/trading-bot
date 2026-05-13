package logging

import (
	"sync"

	"github.com/shopspring/decimal"
)

type TradeCounter struct {
	mu          sync.Mutex
	totalTrades int
	wins        int
	losses      int
	totalProfit decimal.Decimal
}

func NewTradeCounter() *TradeCounter {
	return &TradeCounter{totalProfit: decimal.Zero}
}

func (tc *TradeCounter) Record(trade TradeLog) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.totalTrades++
	if trade.PnlUSDT.IsPositive() {
		tc.wins++
	} else {
		tc.losses++
	}
	tc.totalProfit = tc.totalProfit.Add(trade.PnlUSDT)
}

func (tc *TradeCounter) GetTotalTrades() int {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	return tc.totalTrades
}

func (tc *TradeCounter) GetWins() int {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	return tc.wins
}

func (tc *TradeCounter) GetLosses() int {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	return tc.losses
}

func (tc *TradeCounter) GetWinRate() float64 {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	if tc.totalTrades == 0 {
		return 0
	}
	return float64(tc.wins) / float64(tc.totalTrades) * 100
}

func (tc *TradeCounter) GetTotalProfit() decimal.Decimal {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	return tc.totalProfit
}
