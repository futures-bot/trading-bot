package strategy

import (
	"trading-bot/internal/domain"

	"github.com/shopspring/decimal"
)

// Strategy defines the interface for a trading strategy.
type Strategy interface {
	Calculate([]domain.Candle) (domain.Signal, decimal.Decimal)
	UpdateLastTradeTime()
}
