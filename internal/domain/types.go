package domain

import "github.com/shopspring/decimal"

type Candle struct {
	Open   decimal.Decimal
	High   decimal.Decimal
	Low    decimal.Decimal
	Close  decimal.Decimal
	Volume decimal.Decimal
}

type Signal string

const (
	SignalBuy  Signal = "BUY"
	SignalSell Signal = "SELL"
	SignalHold Signal = "HOLD"
)

type Position struct {
	Symbol          string
	Side            string
	Quantity        decimal.Decimal
	Price           decimal.Decimal
	StopLossPrice   decimal.Decimal
	TakeProfitPrice decimal.Decimal
}

type Order struct {
	Symbol   string
	Side     string
	Type     string
	Quantity decimal.Decimal
	Price    decimal.Decimal
}
