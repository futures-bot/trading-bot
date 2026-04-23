package risk

import (
	"testing"
	"trading-bot/internal/exchange"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestCalculatePositionSize(t *testing.T) {
	mockRestClient := &exchange.RestClient{
		StepSize: decimal.NewFromFloat(0.1),
	}
	pm := NewPositionManager(decimal.NewFromInt(1000), decimal.NewFromFloat(0.01), 10, 100.0, mockRestClient, 0.04, 0.02, 0.001, 0.001)
	pm.SetLeverage(10)

	price := decimal.NewFromFloat(0.5)

	quantity, err := pm.CalculatePositionSize(price, decimal.NewFromInt(100))

	assert.NoError(t, err)

	// Quantity <= 10 * Leverage / Price
	// 10 * 10 / 0.5 = 200
	expectedQuantity := decimal.NewFromFloat(200)

	assert.True(t, expectedQuantity.Equal(quantity))
}
