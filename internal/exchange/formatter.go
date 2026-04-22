package exchange

import (
	"github.com/shopspring/decimal"
)

// FormatPrice formats the price to the correct precision.
func FormatPrice(price decimal.Decimal, precision int32) string {
	return price.StringFixed(precision)
}
