package logging

import (
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

// TradeLog represents a single closed trade.
type TradeLog struct {
	Time       string          `json:"time"`
	Side       string          `json:"side"`
	Entry      decimal.Decimal `json:"entry"`
	Exit       decimal.Decimal `json:"exit"`
	PnlUSDT    decimal.Decimal `json:"pnl_usdt"`
	Balance    decimal.Decimal `json:"balance"`
	RSI        decimal.Decimal `json:"rsi"`
	EMAGap     decimal.Decimal `json:"ema_gap"`
	Volatility decimal.Decimal `json:"volatility"`
	ExitReason string          `json:"exit_reason"`
}

// PnlLogger handles writing trade logs to a file.
type PnlLogger struct {
	file        *os.File
	mu          sync.Mutex
	totalTrades int
	wins        int
	losses      int
	totalProfit decimal.Decimal
}

// GetTotalTrades returns the total number of trades.
func (l *PnlLogger) GetTotalTrades() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.totalTrades
}

// GetWins returns the total number of wins.
func (l *PnlLogger) GetWins() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.wins
}

// GetLosses returns the total number of losses.
func (l *PnlLogger) GetLosses() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.losses
}

// GetWinRate returns the win rate.
func (l *PnlLogger) GetWinRate() float64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.totalTrades == 0 {
		return 0
	}
	return float64(l.wins) / float64(l.totalTrades) * 100
}

// GetTotalProfit returns the total profit.
func (l *PnlLogger) GetTotalProfit() decimal.Decimal {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.totalProfit
}

// NewPnlLogger creates a new PnlLogger.
func NewPnlLogger(filePath string) (*PnlLogger, error) {
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	return &PnlLogger{file: file, totalProfit: decimal.Zero}, nil
}

// LogTrade appends a trade log to the file.
func (l *PnlLogger) LogTrade(trade TradeLog) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.totalTrades++
	if trade.PnlUSDT.IsPositive() {
		l.wins++
	} else {
		l.losses++
	}
	l.totalProfit = l.totalProfit.Add(trade.PnlUSDT)

	trade.Time = time.Now().UTC().Format(time.RFC3339)

	data, err := trade.ToJSON()
	if err != nil {
		log.Printf("Failed to marshal trade log: %v", err)
		return
	}

	if _, err := l.file.WriteString(string(data) + "\n"); err != nil {
		log.Printf("Failed to write to trades log file: %v", err)
		return
	}

	if err := l.file.Sync(); err != nil {
		log.Printf("Failed to sync trades log file: %v", err)
	}
}

// Close closes the log file.
func (l *PnlLogger) Close() {
	l.file.Close()
}

func (t *TradeLog) ToJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Time       string `json:"time"`
		Side       string `json:"side"`
		Entry      string `json:"entry"`
		Exit       string `json:"exit"`
		PnlUSDT    string `json:"pnl_usdt"`
		Balance    string `json:"balance"`
		RSI        string `json:"rsi"`
		EMAGap     string `json:"ema_gap"`
		Volatility string `json:"volatility"`
		ExitReason string `json:"exit_reason"`
	}{
		Time:       t.Time,
		Side:       t.Side,
		Entry:      t.Entry.StringFixed(4),
		Exit:       t.Exit.StringFixed(4),
		PnlUSDT:    t.PnlUSDT.StringFixed(4),
		Balance:    t.Balance.StringFixed(4),
		RSI:        t.RSI.StringFixed(4),
		EMAGap:     t.EMAGap.StringFixed(4),
		Volatility: t.Volatility.StringFixed(4),
		ExitReason: t.ExitReason,
	})
}
