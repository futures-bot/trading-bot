package trading

import (
	"testing"

	"trading-bot/internal/config"
	"trading-bot/internal/marketdata"
	"trading-bot/internal/trading/domain"

	"github.com/shopspring/decimal"
)

func TestPositionManager(t *testing.T) {
	cfg := &config.Config{
		TakeProfitPct:       0.02,
		StopLossPct:         0.01,
		BreakEvenTriggerPct: 0.01,
		TrailDistancePct:    0.005,
		Leverage:            10,
		SessionBudget:       100.0,
	}
	pm := NewPositionManager(
		decimal.NewFromInt(1000),
		decimal.NewFromFloat(0.1),
		10,
		100,
		&marketdata.RestClient{StepSize: decimal.NewFromFloat(0.01)},
		cfg,
		nil,
	)

	// Test CalculatePositionSize
	price := decimal.NewFromInt(100)
	qty, err := pm.CalculatePositionSize(price, decimal.NewFromInt(1000), decimal.NewFromFloat(0.002))
	if err != nil {
		t.Errorf("Error calculating position size: %v", err)
	}
	if !qty.Equal(decimal.NewFromFloat(10.0)) {
		t.Errorf("Expected quantity to be 10.0, got %s", qty)
	}

	// Test UpdateBalance
	pm.UpdateBalance(decimal.NewFromInt(50))
	if !pm.Balance().Equal(decimal.NewFromInt(1050)) {
		t.Errorf("Expected balance to be 1050, got %s", pm.Balance())
	}

	// Test Evaluate
	position := &domain.Position{
		Price:           decimal.NewFromInt(100),
		Side:            string(domain.SignalBuy),
		TakeProfitPrice: decimal.NewFromInt(102),
		StopLossPrice:   decimal.NewFromInt(99),
	}
	pm.SetCurrentPosition(position)
}
