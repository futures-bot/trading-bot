package domain

import "github.com/shopspring/decimal"

// ExchangeConfig holds API credentials for an exchange.
type ExchangeConfig struct {
	APIKey    string `json:"api_key,omitempty"`
	APISecret string `json:"api_secret,omitempty"`
}

// StartRequest is the request body for the /start endpoint.
type StartRequest struct {
	UserID         string                 `json:"user_id"`
	Mode           string                 `json:"mode"`
	ExchangeConfig *ExchangeConfig        `json:"exchange_config,omitempty"`
	Symbol         string                 `json:"symbol"`
	Params         map[string]interface{} `json:"params"`
}

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
	IsBreakEvenSet  bool
	HighestPrice    decimal.Decimal
}

type Order struct {
	Symbol   string
	Side     string
	Type     string
	Quantity decimal.Decimal
	Price    decimal.Decimal
}
