package database

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Session represents a trading session (a run of the bot).
type Session struct {
	gorm.Model
	StartTime    time.Time       `json:"start_time"`
	EndTime      *time.Time      `json:"end_time"`
	StartBalance decimal.Decimal `gorm:"type:numeric(20,8)" json:"start_balance"`
	EndBalance   decimal.Decimal `gorm:"type:numeric(20,8)" json:"end_balance"`
	TotalProfit  decimal.Decimal `gorm:"type:numeric(20,8)" json:"total_profit"`
	Status       string          `json:"status"` // "running", "completed", "failed"
	Mode         string          `json:"mode"`   // "paper", "live", "backtest"
	Trades       []Trade         `json:"trades" gorm:"foreignKey:SessionID"`
}

// Trade represents a closed trade in the database.
type Trade struct {
	gorm.Model
	SessionID  uint            `json:"session_id" gorm:"index"`
	Symbol     string          `json:"symbol" gorm:"index"`
	Time       time.Time       `json:"time"`
	Side       string          `json:"side"`
	Quantity   decimal.Decimal `gorm:"type:numeric(20,8)" json:"quantity"`
	Entry      decimal.Decimal `gorm:"type:numeric(20,8)" json:"entry"`
	Exit       decimal.Decimal `gorm:"type:numeric(20,8)" json:"exit"`
	PnlUSDT    decimal.Decimal `gorm:"type:numeric(20,8)" json:"pnl_usdt"`
	ExitReason string          `json:"exit_reason"` // "TAKE_PROFIT", "STOP_LOSS", "RSI_EXHAUSTION", etc.
}

// Kline represents a historical price candle.
type Kline struct {
	ID        uint            `gorm:"primaryKey"`
	Symbol    string          `gorm:"index:idx_sym_time,unique"`
	Interval  string          `gorm:"index:idx_sym_time,unique"`
	OpenTime  time.Time       `gorm:"index:idx_sym_time,unique"`
	CloseTime time.Time       `json:"close_time"`
	Open      decimal.Decimal `gorm:"type:numeric(20,8)" json:"open"`
	High      decimal.Decimal `gorm:"type:numeric(20,8)" json:"high"`
	Low       decimal.Decimal `gorm:"type:numeric(20,8)" json:"low"`
	Close     decimal.Decimal `gorm:"type:numeric(20,8)" json:"close"`
	Volume    decimal.Decimal `gorm:"type:numeric(20,8)" json:"volume"`
}
