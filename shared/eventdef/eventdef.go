package eventdef

import "time"

// Event represents a standard event structure.
type Event struct {
	EventType string      `json:"event_type"`
	Version   int         `json:"version"`
	Timestamp time.Time   `json:"timestamp"`
	Source    string      `json:"source"`
	Payload   interface{} `json:"payload"`
}

// TradePayload defines the data for a trade event.
// This is a placeholder and should be expanded with actual trade data.
type TradePayload struct {
	Symbol    string    `json:"symbol"`
	Price     float64   `json:"price"`
	Quantity  float64   `json:"quantity"`
	Timestamp time.Time `json:"timestamp"`
}

// NewTradeOpenedEvent creates a new TradeOpenedEvent.
func NewTradeOpenedEvent(source string, payload TradePayload) Event {
	return Event{
		EventType: "trade.opened",
		Version:   1,
		Timestamp: time.Now(),
		Source:    source,
		Payload:   payload,
	}
}

// NewEvent creates a new generic event.
func NewEvent(eventType, source string, version int, payload interface{}) Event {
	return Event{
		EventType: eventType,
		Version:   version,
		Timestamp: time.Now(),
		Source:    source,
		Payload:   payload,
	}
}
