package database

import (
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type Repository interface {
	SaveTrade(trade *Trade) error
	GetSystemState() (*SystemState, error)
	SaveSystemState(state *SystemState) error
	GetPaperBalance() (decimal.Decimal, error)
	SavePaperBalance(balance decimal.Decimal) error
	GetLastTrades(limit int) ([]Trade, error)
	GetPerformanceStats() (PerformanceStats, error)
}

type repo struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repo{db: db}
}

type PerformanceStats struct {
	WinRate     float64 `json:"win_rate"`
	TotalPnl    float64 `json:"total_pnl"`
	TotalTrades int64   `json:"total_trades"`
}

func (r *repo) SaveTrade(trade *Trade) error {
	return r.db.Create(trade).Error
}

func (r *repo) GetSystemState() (*SystemState, error) {
	var state SystemState
	err := r.db.First(&state).Error
	if err != nil {
		return nil, err
	}
	return &state, nil
}

func (r *repo) SaveSystemState(state *SystemState) error {
	return r.db.Save(state).Error
}

func (r *repo) GetPaperBalance() (decimal.Decimal, error) {
	state, err := r.GetSystemState()
	if err != nil {
		return decimal.Zero, err
	}
	return state.PaperBalance, nil
}

func (r *repo) SavePaperBalance(balance decimal.Decimal) error {
	state, err := r.GetSystemState()
	if err != nil {
		state = &SystemState{}
	}
	state.PaperBalance = balance
	return r.SaveSystemState(state)
}

func (r *repo) GetLastTrades(limit int) ([]Trade, error) {
	var trades []Trade
	err := r.db.Order("created_at desc").Limit(limit).Find(&trades).Error
	if err != nil {
		return nil, err
	}
	return trades, nil
}

func (r *repo) GetPerformanceStats() (PerformanceStats, error) {
	var trades []Trade
	if err := r.db.Find(&trades).Error; err != nil {
		return PerformanceStats{}, err
	}

	var wins int64
	var totalPnl float64
	totalTrades := len(trades)

	if totalTrades == 0 {
		return PerformanceStats{}, nil
	}

	for _, trade := range trades {
		pnl, _ := trade.Profit.Float64()
		if pnl > 0 {
			wins++
		}
		totalPnl += pnl
	}

	winRate := (float64(wins) / float64(totalTrades)) * 100

	return PerformanceStats{
		WinRate:     winRate,
		TotalPnl:    totalPnl,
		TotalTrades: int64(totalTrades),
	}, nil
}
