package database

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Trade represents a trade in the database.
// This is a simplified model and should be expanded as needed.
type Trade struct {
	gorm.Model
	Time    time.Time       `json:"time"`
	Side    string          `json:"side"`
	Entry   decimal.Decimal `json:"entry"`
	Exit    decimal.Decimal `json:"exit"`
	PnlUSDT decimal.Decimal `json:"pnl_usdt"`
	Balance decimal.Decimal `json:"balance"`
}
