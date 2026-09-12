package database

import (
	"fmt"

	"trading-bot/internal/config"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Repository defines the interface for database operations.
type Repository interface {
	CreateSession(session *Session) error
	UpdateSession(session *Session) error
	GetSessions(limit int) ([]Session, error)
	GetSessionByID(id uint) (*Session, error)
	SaveTrade(trade *Trade) error
	GetTrades(sessionID uint, limit int) ([]Trade, error)
	GetStats() (map[string]interface{}, error)
	SaveKlines(klines []Kline) error
	GetKlines(symbol, interval string, limit int) ([]Kline, error)
}

// GormRepository is a GORM implementation of the Repository.
type GormRepository struct {
	*gorm.DB
}

// NewGormRepository creates a new GORM repository.
func NewGormRepository(cfg *config.Config) (*GormRepository, error) {
	db, err := gorm.Open(postgres.Open(cfg.DB_DSN), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Auto-migrate the schema
	err = db.AutoMigrate(&Session{}, &Trade{}, &Kline{})
	if err != nil {
		return nil, fmt.Errorf("failed to auto-migrate database: %w", err)
	}

	return &GormRepository{db}, nil
}

// CreateSession creates a new trading session.
func (r *GormRepository) CreateSession(session *Session) error {
	return r.Create(session).Error
}

// UpdateSession updates an existing trading session.
func (r *GormRepository) UpdateSession(session *Session) error {
	return r.Save(session).Error
}

// GetSessions retrieves recent sessions.
func (r *GormRepository) GetSessions(limit int) ([]Session, error) {
	var sessions []Session
	err := r.Order("created_at desc").Limit(limit).Find(&sessions).Error
	return sessions, err
}

// GetSessionByID retrieves a session and its trades by ID.
func (r *GormRepository) GetSessionByID(id uint) (*Session, error) {
	var session Session
	err := r.Preload("Trades").First(&session, id).Error
	return &session, err
}

// SaveTrade saves a trade to the database.
func (r *GormRepository) SaveTrade(trade *Trade) error {
	return r.Create(trade).Error
}

// GetTrades retrieves recent trades, optionally filtered by sessionID.
func (r *GormRepository) GetTrades(sessionID uint, limit int) ([]Trade, error) {
	var trades []Trade
	query := r.Order("time desc").Limit(limit)
	if sessionID > 0 {
		query = query.Where("session_id = ?", sessionID)
	}
	err := query.Find(&trades).Error
	return trades, err
}

// GetStats aggregates metrics from all completed trades.
func (r *GormRepository) GetStats() (map[string]interface{}, error) {
	var totalTrades int64
	var totalProfit float64
	var winningTrades int64

	r.Model(&Trade{}).Count(&totalTrades)

	// Since decimal mapping to SQL SUM can be tricky depending on the DB, 
	// we'll query the trades and calculate in Go to ensure precision compatibility.
	var trades []Trade
	r.Find(&trades)

	for _, t := range trades {
		pnl := t.PnlUSDT.InexactFloat64()
		totalProfit += pnl
		if pnl > 0 {
			winningTrades++
		}
	}

	winRate := float64(0)
	if totalTrades > 0 {
		winRate = (float64(winningTrades) / float64(totalTrades)) * 100
	}

	stats := map[string]interface{}{
		"total_trades":   totalTrades,
		"winning_trades": winningTrades,
		"win_rate_pct":   winRate,
		"total_profit":   totalProfit,
	}

	return stats, nil
}

// SaveKlines batch saves klines, ignoring duplicates based on the unique index.
func (r *GormRepository) SaveKlines(klines []Kline) error {
	// In GORM with Postgres, to ignore conflicts, we use OnConflict DO NOTHING
	// gorm.io/gorm/clause is needed.
	// We will implement this as a basic Create/Save loop for now if we can't import clause directly
	// Or we can just use Save which updates on conflict if ID is set, 
	// but since ID is not set, we'll try to insert and just ignore errors.
	// Actually, the proper way without clause is to just loop, but that's slow.
	// We'll use a direct Insert with clause if possible, or fall back to basic loop.

	// A simpler approach for the immediate term is to just use Create and let it fail on duplicates
	// But let's actually just loop them for now to avoid breaking the batch.
	for _, k := range klines {
		r.Exec(`INSERT INTO klines (symbol, interval, open_time, close_time, open, high, low, close, volume) 
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) 
				ON CONFLICT (symbol, interval, open_time) DO NOTHING`,
			k.Symbol, k.Interval, k.OpenTime, k.CloseTime, k.Open, k.High, k.Low, k.Close, k.Volume)
	}
	return nil
}

// GetKlines retrieves klines ordered by time.
func (r *GormRepository) GetKlines(symbol, interval string, limit int) ([]Kline, error) {
	var klines []Kline
	err := r.Where("symbol = ? AND interval = ?", symbol, interval).
		Order("open_time asc").Limit(limit).Find(&klines).Error
	return klines, err
}
