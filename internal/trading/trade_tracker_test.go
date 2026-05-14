package trading

import (
	"testing"

	"trading-bot/internal/trading/domain"

	"github.com/shopspring/decimal"
)

func TestTradeTracker(t *testing.T) {
	tracker := NewTradeTracker(0.02, 0.01, 3, 0.005)

	// Test ConfirmSignal
	if tracker.ConfirmSignal(domain.SignalBuy) {
		t.Errorf("Expected false, got true")
	}
	if tracker.ConfirmSignal(domain.SignalBuy) {
		t.Errorf("Expected false, got true")
	}
	if !tracker.ConfirmSignal(domain.SignalBuy) {
		t.Errorf("Expected true, got false")
	}

	// Test StartTrade and EndTrade
	tracker.StartTrade(decimal.NewFromInt(100), domain.SignalBuy)
	if !tracker.InTrade() {
		t.Errorf("Expected to be in trade")
	}
	tracker.EndTrade()
	if tracker.InTrade() {
		t.Errorf("Expected to not be in trade")
	}

	// Test ShouldExit
	tracker.StartTrade(decimal.NewFromInt(100), domain.SignalBuy)
	if !tracker.ShouldExit(decimal.NewFromInt(101), "TAKE_PROFIT") {
		t.Errorf("Expected true, got false")
	}
}
