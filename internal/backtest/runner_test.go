package backtest

import (
	"context"
	"os"
	"testing"

	"trading-bot/internal/config"
	"trading-bot/internal/trading"
	"trading-bot/internal/trading/domain"
	"trading-bot/shared/eventdef"

	"github.com/shopspring/decimal"
)

// MockStrategy is a mock implementation of the trading.Strategy interface.
type MockStrategy struct {
	Signal domain.Signal
}

// Calculate returns the mock signal.
func (s *MockStrategy) Calculate(candles []domain.Candle) (domain.Signal, decimal.Decimal) {
	return s.Signal, decimal.Zero
}

func (s *MockStrategy) UpdateLastTradeTime() {}

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

func TestRunner(t *testing.T) {
	// Create a temporary data file
	tmpfile, err := os.CreateTemp("", "test.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	// Write some data to the file
	data := `{"time":"2021-01-01T00:00:00Z","price":"100"}
{"time":"2021-01-01T00:01:00Z","price":"101"}
`
	if _, err := tmpfile.Write([]byte(data)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		BacktestFile: tmpfile.Name(),
		Symbol:       "BTCUSDT",
	}
	strategy := &MockStrategy{Signal: domain.SignalBuy}
	pm := trading.NewBacktestPositionManager(cfg)
	publisher := &MockPublisher{}

	runner := NewRunner(strategy, pm, cfg, publisher)
	if err := runner.Run(tmpfile.Name()); err != nil {
		t.Fatalf("Error running backtest: %v", err)
	}

	if len(publisher.PublishedEvents) == 0 {
		t.Errorf("Expected events to be published")
	}
}
