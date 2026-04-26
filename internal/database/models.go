package database

import (
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// User represents a user of the trading bot.
// For now, API credentials are placeholders.
type User struct {
	gorm.Model
	Name      string `gorm:"unique"`
	ApiKey    string
	ApiSecret string
}

// Trade represents a single trade executed by the bot.
type Trade struct {
	gorm.Model
	Symbol     string          `json:"symbol"`
	Side       string          `json:"side"`
	EntryPrice decimal.Decimal `json:"entry_price" gorm:"type:decimal(20,8);"`
	ExitPrice  decimal.Decimal `json:"exit_price" gorm:"type:decimal(20,8);"`
	Profit     decimal.Decimal `json:"profit" gorm:"type:decimal(20,8);"`
	ExitReason string          `json:"exit_reason"`
}

// SystemState stores the system state, like paper balance.
type SystemState struct {
	gorm.Model
	PaperBalance decimal.Decimal `gorm:"type:decimal(20,8);"`
}
