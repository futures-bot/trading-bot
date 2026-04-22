package strategy

import "trading-bot/internal/domain"

// Strategy defines the interface for a trading strategy.
type Strategy interface {
	Calculate([]domain.Candle) domain.Signal
	UpdateLastTradeTime()
}
