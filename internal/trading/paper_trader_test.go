package trading

import (
	"context"
	"testing"
	"time"

	"trading-bot/internal/config"
	"trading-bot/internal/trading/domain"
	"trading-bot/shared/eventdef"

	"github.com/shopspring/decimal"
)

// MockPublisher is a mock implementation of the events.Publisher interface.
type MockPublisher struct {
	PublishedEvents map[string]eventdef.Event
}

// Publish records the event in the PublishedEvents map.
func (p *MockPublisher) Publish(ctx context.Context, topic string, event eventdef.Event) error {
	if p.PublishedEvents == nil {
		p.PublishedEvents = make(map[string]eventdef.Event)
	}
	p.PublishedEvents[topic] = event
	return nil
}

func (p *MockPublisher) Close() {}

func TestPaperTrader(t *testing.T) {
	cfg := &config.Config{
		EMAFast:              12,
		EMASlow:              26,
		TakeProfitPct:        0.02,
		StopLossPct:          0.01,
		ConfirmationCount:    3,
		MinProfitForFlipExit: 0.005,
		SessionDurationMin:   1,
		Symbol:               "BTCUSDT",
	}
	publisher := &MockPublisher{}
	trader, err := NewPaperTrader(cfg, publisher)
	if err != nil {
		t.Fatalf("Error creating paper trader: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go trader.Start(ctx)

	// Give the trader time to start
	time.Sleep(2 * time.Second)

	// Simulate a buy signal
	price := decimal.NewFromInt(100)
	trader.candles = make([]domain.Candle, 30)
	for i := 0; i < 30; i++ {
		price = price.Add(decimal.NewFromInt(1))
		trader.candles[i] = domain.Candle{Close: price}
	}

	time.Sleep(2 * time.Second)

	if trader.currentPosition == nil {
		t.Errorf("Expected a position to be opened")
	}

	cancel()
}
