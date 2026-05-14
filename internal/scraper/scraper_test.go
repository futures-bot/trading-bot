package scraper

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Mock successful server response
func mockServerSuccess(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`[[1, "100", "100", "100", "100", "100", 2, "100", "100", "100", "100", "100"]]`))
}

// Mock server response with an error
func mockServerError(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusBadRequest)
}

// Mock implementation of the events.Publisher interface.
type MockPublisher struct {
	PublishedEvents map[string]interface{}
}

// Publish records the event in the PublishedEvents map.
func (p *MockPublisher) Publish(ctx context.Context, topic string, data interface{}) error {
	if p.PublishedEvents == nil {
		p.PublishedEvents = make(map[string]interface{})
	}
	p.PublishedEvents[topic] = data
	return nil
}

func (p *MockPublisher) Close() {}

func TestScraper(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(mockServerSuccess))
	defer server.Close()

	publisher := &MockPublisher{}
	scraper := newWithClient(publisher, server.Client(), server.URL)

	count, err := scraper.ScrapeSymbols(context.Background(), []string{"BTCUSDT"}, "1m", 1)
	if err != nil {
		t.Errorf("Error scraping symbols: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 kline to be scraped, got %d", count)
	}

	if len(publisher.PublishedEvents) == 0 {
		t.Errorf("Expected events to be published")
	}
}

func TestUnquote(t *testing.T) {
	if unquote(json.RawMessage(`"hello"`)) != "hello" {
		t.Errorf("Expected hello")
	}
	if unquote(json.RawMessage(`hello`)) != "hello" {
		t.Errorf("Expected hello")
	}
}
