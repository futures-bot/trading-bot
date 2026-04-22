package strategy

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestCalculateEMA(t *testing.T) {
	tests := []struct {
		name     string
		prices   []decimal.Decimal
		period   int
		expected decimal.Decimal
	}{
		{
			name: "simple case",
			prices: []decimal.Decimal{
				decimal.NewFromFloat(1.0),
				decimal.NewFromFloat(1.1),
				decimal.NewFromFloat(1.2),
				decimal.NewFromFloat(1.3),
				decimal.NewFromFloat(1.4),
			},
			period:   5,
			expected: decimal.NewFromFloat(1.2),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ema := calculateEMA(tt.prices, tt.period)
			assert.True(t, tt.expected.Equal(ema), "expected %s, got %s", tt.expected, ema)
		})
	}
}
