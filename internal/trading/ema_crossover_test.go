package trading

import (
	"testing"

	"trading-bot/internal/trading/domain"
)

func TestEMACrossover(t *testing.T) {
	// Test with not enough candles
	tracker := NewTradeTracker(0.02, 0.01, 3, 0.005)
	strategy := NewEMACrossover(12, 26, 0, tracker)
	signal, _ := strategy.Calculate([]domain.Candle{})
	if signal != domain.SignalHold {
		t.Errorf("Expected SignalHold, got %s", signal)
	}
}
