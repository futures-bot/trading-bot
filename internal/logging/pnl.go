package logging

import (
	"github.com/shopspring/decimal"
)

type TradeLog struct {
	Time       string          `json:"time"`
	Side       string          `json:"side"`
	Entry      decimal.Decimal `json:"entry"`
	Exit       decimal.Decimal `json:"exit"`
	PnlUSDT   decimal.Decimal `json:"pnl_usdt"`
	Balance    decimal.Decimal `json:"balance"`
	RSI        decimal.Decimal `json:"rsi"`
	EMAGap     decimal.Decimal `json:"ema_gap"`
	Volatility decimal.Decimal `json:"volatility"`
	ExitReason string          `json:"exit_reason"`
}
