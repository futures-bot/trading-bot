package database

import (
	tradingv1 "trading-bot/internal/trading/domain"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository interface {
	SaveTrade(trade *tradingv1.Trade) error
	GetPaperBalance() (decimal.Decimal, error)
	SavePaperBalance(balance decimal.Decimal) error
	GetLastTrades(limit int) ([]tradingv1.Trade, error)
	GetTradeByID(id uint) (*tradingv1.Trade, error)
	GetPerformanceStats() (PerformanceStats, error)
	SaveSession(session *Session) error
	GetLastSessions(limit int) ([]Session, error)

	SaveKlines(klines []Kline) error
	GetKlines(symbol, interval string, limit int) ([]Kline, error)
	GetLatestKlineTime(symbol, interval string) (int64, error)

	SaveTradeLog(log *TradeLog) error
	SaveMarketPulse(pulse *MarketPulseLog) error
	SaveBotLog(entry *BotLog) error
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

func (r *repo) SaveTrade(trade *tradingv1.Trade) error {
	return r.db.Create(trade).Error
}

func (r *repo) GetPaperBalance() (decimal.Decimal, error) {
	var state SystemState
	err := r.db.First(&state).Error
	if err != nil {
		return decimal.Zero, err
	}
	return decimal.NewFromFloat(state.PaperBalance), nil
}

func (r *repo) SavePaperBalance(balance decimal.Decimal) error {
	var state SystemState
	err := r.db.First(&state).Error
	if err != nil {
		state = SystemState{}
	}
	f, _ := balance.Float64()
	state.PaperBalance = f
	return r.db.Save(&state).Error
}

func (r *repo) GetLastTrades(limit int) ([]tradingv1.Trade, error) {
	var trades []tradingv1.Trade
	err := r.db.Order("created_at desc").Limit(limit).Find(&trades).Error
	return trades, err
}

func (r *repo) GetTradeByID(id uint) (*tradingv1.Trade, error) {
	var trade tradingv1.Trade
	err := r.db.First(&trade, id).Error
	return &trade, err
}

func (r *repo) GetPerformanceStats() (PerformanceStats, error) {
	var trades []tradingv1.Trade
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

func (r *repo) SaveSession(session *Session) error {
	return r.db.Create(session).Error
}

func (r *repo) GetLastSessions(limit int) ([]Session, error) {
	var sessions []Session
	err := r.db.Order("created_at desc").Limit(limit).Find(&sessions).Error
	return sessions, err
}

func (r *repo) SaveKlines(klines []Kline) error {
	if len(klines) == 0 {
		return nil
	}
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "symbol"}, {Name: "interval"}, {Name: "open_time"}},
		DoNothing: true,
	}).CreateInBatches(klines, 500).Error
}

func (r *repo) GetKlines(symbol, interval string, limit int) ([]Kline, error) {
	var klines []Kline
	err := r.db.Where("symbol = ? AND interval = ?", symbol, interval).
		Order("open_time asc").Limit(limit).Find(&klines).Error
	return klines, err
}

func (r *repo) GetLatestKlineTime(symbol, interval string) (int64, error) {
	var kline Kline
	err := r.db.Where("symbol = ? AND interval = ?", symbol, interval).
		Order("open_time desc").First(&kline).Error
	if err != nil {
		return 0, err
	}
	return kline.OpenTime, nil
}

func (r *repo) SaveTradeLog(entry *TradeLog) error {
	return r.db.Create(entry).Error
}

func (r *repo) SaveMarketPulse(pulse *MarketPulseLog) error {
	return r.db.Create(pulse).Error
}

func (r *repo) SaveBotLog(entry *BotLog) error {
	return r.db.Create(entry).Error
}
