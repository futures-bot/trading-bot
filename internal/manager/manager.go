package manager

import (
	"context"
	"fmt"
	"sync"
	"time"

	"trading-bot/internal/config"
	"trading-bot/internal/database"
	"trading-bot/internal/domain"
	"trading-bot/internal/engine"
	"trading-bot/internal/logging"
	"trading-bot/internal/notifier"

	"github.com/shopspring/decimal"
)

// Manager holds all the running traders.
type Manager struct {
	traders map[string]engine.Trader
	cancels map[string]context.CancelFunc
	mu      sync.RWMutex
	repo    database.Repository
}

// NewManager creates a new Manager.
func NewManager(repo database.Repository) *Manager {
	return &Manager{
		traders: make(map[string]engine.Trader),
		cancels: make(map[string]context.CancelFunc),
		repo:    repo,
	}
}

// StartTrader starts a new trader.
func (m *Manager) StartTrader(ctx context.Context, req domain.StartRequest) (engine.Trader, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.traders[req.UserID]; ok {
		return nil, fmt.Errorf("trader already running for user %s", req.UserID)
	}

	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	cfg.Symbol = req.Symbol

	var trader engine.Trader

	switch req.Mode {
	case "paper":
		pnlLogger, _ := logging.NewPnlLogger("paper.jsonl")
		trader, err = engine.NewPaperTrader(cfg, pnlLogger, m.repo)
	case "testnet":
		pnlLogger, _ := logging.NewPnlLogger("testnet.jsonl")
		marketPulseLogger, _ := logging.NewMarketPulseLogger("market_pulse.jsonl")

		apiKey, apiSecret := "", ""
		if req.ExchangeConfig != nil {
			apiKey = req.ExchangeConfig.APIKey
			apiSecret = req.ExchangeConfig.APISecret
		} else if req.UserID == "fer_admin" {
			// Load from .env or somewhere safe
			apiKey = "your_fallback_api_key"
			apiSecret = "your_fallback_api_secret"
		}

		trader, err = engine.NewBinanceTrader(cfg, pnlLogger, marketPulseLogger, notifier.NewNullNotifier(), m.repo, time.Now(), 0, decimal.Zero, apiKey, apiSecret)
	default:
		return nil, fmt.Errorf("unsupported mode: %s", req.Mode)
	}

	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.traders[req.UserID] = trader
	m.cancels[req.UserID] = cancel

	go trader.Start(ctx)

	return trader, nil
}

// StopTrader stops a running trader.
func (m *Manager) StopTrader(userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	trader, ok := m.traders[userID]
	if !ok {
		return fmt.Errorf("no trader running for user %s", userID)
	}

	cancel, ok := m.cancels[userID]
	if !ok {
		// This should not happen if a trader exists
		return fmt.Errorf("no cancel function for user %s", userID)
	}

	// Signal the trader to stop
	cancel()

	// Call the trader's Stop method for cleanup
	trader.Stop()

	// Remove the trader and its cancel function
	delete(m.traders, userID)
	delete(m.cancels, userID)

	return nil
}

// GetStatus returns the status of all running traders.
func (m *Manager) GetStatus() map[string]engine.Status {
	m.mu.RLock()
	defer m.mu.RUnlock()

	statuses := make(map[string]engine.Status)
	for userID, trader := range m.traders {
		statuses[userID] = trader.GetStatus()
	}

	return statuses
}
