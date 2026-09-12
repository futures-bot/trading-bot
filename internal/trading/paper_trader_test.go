package trading

import (
	"context"
	"testing"
	"time"

	"trading-bot/internal/config"
	"trading-bot/internal/marketdata"
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
		Symbols:              []string{"BTCUSDT"},
	}
	publisher := &MockPublisher{}
	trader, err := NewPaperTrader(cfg, publisher, "BTCUSDT", nil)
	if err != nil {
		t.Fatalf("Error creating paper trader: %v", err)
	}

	// Inject custom price channel for the test
	priceCh := make(chan marketdata.PriceUpdate, 100)
	trader.PriceUpdateCh = priceCh

	ctx, cancel := context.WithCancel(context.Background())
	go trader.Start(ctx)

	// Simulate a price trend to trigger a buy signal (EMA crossover)
	price := decimal.NewFromInt(100)
	for i := 0; i < 30; i++ {
		price = price.Add(decimal.NewFromInt(1))
		priceCh <- marketdata.PriceUpdate{
			Symbol: "BTCUSDT",
			Price:  price.String(),
		}
		// Yield slightly to let trader process
		time.Sleep(10 * time.Millisecond)
	}

	// Let it process the remaining queued events
	time.Sleep(500 * time.Millisecond)

	// Safely verify if a position was taken.
	hasPosition := false
	if trader.GetCurrentPosition() != nil {
		hasPosition = true
	}

	if !hasPosition {
		t.Errorf("Expected a position to be opened based on EMA crossover")
	}

	cancel()
}
