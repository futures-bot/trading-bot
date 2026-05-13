package domain

import (
	"context"
	"time"

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
	IsRunning bool      `json:"is_running"`
	Uptime    string    `json:"uptime"`
	Balance   float64   `json:"balance"`
	StartTime time.Time `json:"start_time"`
}

type Trader interface {
	Start(ctx context.Context)
	Stop()
	GetStatus() Status
}

type SystemState struct {
	gorm.Model
	PaperBalance float64 `gorm:"type:decimal(20,8);"`
}

type Session struct {
	gorm.Model
	Mode         string  `json:"mode"`
	Symbol       string  `json:"symbol"`
	DurationSecs int     `json:"duration_secs"`
	TotalTrades  int     `json:"total_trades"`
	Wins         int     `json:"wins"`
	Losses       int     `json:"losses"`
	NetPnL       float64 `json:"net_pnl" gorm:"type:decimal(20,8);"`
	FinalBalance float64 `json:"final_balance" gorm:"type:decimal(20,8);"`
	ProfitFactor float64 `json:"profit_factor"`
	MaxDrawdown  float64 `json:"max_drawdown" gorm:"type:decimal(20,8);"`
	SharpeRatio  float64 `json:"sharpe_ratio"`
	Expectancy   float64 `json:"expectancy" gorm:"type:decimal(20,8);"`
	Status       string  `json:"status"`
}

type Kline struct {
	gorm.Model
	Symbol    string  `json:"symbol" gorm:"uniqueIndex:idx_kline_unique"`
	Interval  string  `json:"interval" gorm:"uniqueIndex:idx_kline_unique"`
	OpenTime  int64   `json:"open_time" gorm:"uniqueIndex:idx_kline_unique"`
	CloseTime int64   `json:"close_time"`
	Open      float64 `json:"open" gorm:"type:decimal(20,8);"`
	High      float64 `json:"high" gorm:"type:decimal(20,8);"`
	Low       float64 `json:"low" gorm:"type:decimal(20,8);"`
	Close     float64 `json:"close" gorm:"type:decimal(20,8);"`
	Volume    float64 `json:"volume" gorm:"type:decimal(20,8);"`
}

type TradeLog struct {
	gorm.Model
	Mode       string  `json:"mode"`
	Symbol     string  `json:"symbol"`
	Side       string  `json:"side"`
	Entry      float64 `json:"entry" gorm:"type:decimal(20,8);"`
	Exit       float64 `json:"exit" gorm:"type:decimal(20,8);"`
	PnlUSDT    float64 `json:"pnl_usdt" gorm:"type:decimal(20,8);"`
	ExitReason string  `json:"exit_reason"`
}

type MarketPulseLog struct {
	gorm.Model
	Mode   string  `json:"mode"`
	Symbol string  `json:"symbol"`
	Price  float64 `json:"price" gorm:"type:decimal(20,8);"`
	RSI    float64 `json:"rsi" gorm:"type:decimal(20,8);"`
	EMAGap float64 `json:"ema_gap" gorm:"type:decimal(20,8);"`
}

type BotLog struct {
	gorm.Model
	Mode    string `json:"mode"`
	Level   string `json:"level"`
	Message string `json:"message" gorm:"type:text"`
}
