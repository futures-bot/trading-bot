package domain

import (
	"context"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type Signal string

const (
	SignalBuy  Signal = "BUY"
	SignalSell Signal = "SELL"
	SignalHold Signal = "HOLD"
)

type Candle struct {
	Open   decimal.Decimal `json:"open"`
	High   decimal.Decimal `json:"high"`
	Low    decimal.Decimal `json:"low"`
	Close  decimal.Decimal `json:"close"`
	Volume decimal.Decimal `json:"volume"`
}

type Position struct {
	Symbol          string          `json:"symbol"`
	Side            string          `json:"side"`
	Quantity        decimal.Decimal `json:"quantity"`
	Price           decimal.Decimal `json:"price"`
	StopLossPrice   decimal.Decimal `json:"stop_loss_price"`
	TakeProfitPrice decimal.Decimal `json:"take_profit_price"`
	IsBreakEvenSet  bool            `json:"is_break_even_set"`
	HighestPrice    decimal.Decimal `json:"highest_price"`
}

type Trade struct {
	gorm.Model
	Symbol     string          `json:"symbol"`
	Side       string          `json:"side"`
	EntryPrice decimal.Decimal `json:"entry_price" gorm:"type:decimal(20,8);"`
	ExitPrice  decimal.Decimal `json:"exit_price" gorm:"type:decimal(20,8);"`
	Profit     decimal.Decimal `json:"profit" gorm:"type:decimal(20,8);"`
	ExitReason string          `json:"exit_reason"`
}

type Status struct {
	IsRunning bool    `json:"is_running"`
	Uptime    string  `json:"uptime"`
	Balance   float64 `json:"balance"`
}

type Trader interface {
	Start(ctx context.Context)
	Stop()
	GetStatus() Status
}
