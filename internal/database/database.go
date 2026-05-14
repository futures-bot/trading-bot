package database

import (
	"fmt"

	"trading-bot/internal/config"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Repository defines the interface for database operations.
type Repository interface {
	SaveTrade(trade *Trade) error
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
	err = db.AutoMigrate(&Trade{})
	if err != nil {
		return nil, fmt.Errorf("failed to auto-migrate database: %w", err)
	}

	return &GormRepository{db}, nil
}

// SaveTrade saves a trade to the database.
func (r *GormRepository) SaveTrade(trade *Trade) error {
	return r.Create(trade).Error
}
