package exchange

import (
	"context"
	"log"
	"time"

	"github.com/adshao/go-binance/v2/futures"
)

// PriceUpdate represents a single price update from the exchange.
// It will be expanded later.
type PriceUpdate struct {
	Symbol string
	Price  string
}

// WebsocketClient handles the connection to the Binance WebSocket API.
type WebsocketClient struct{}

// NewWebsocketClient creates a new WebsocketClient.
func NewWebsocketClient() *WebsocketClient {
	return &WebsocketClient{}
}

// Start starts the WebSocket stream for the given symbol.
func (c *WebsocketClient) Start(ctx context.Context, symbol string, priceCh chan<- PriceUpdate) {
	wsAggTradeHandler := func(event *futures.WsAggTradeEvent) {
		log.Printf("Received aggregate trade event: %+v", event)
		priceCh <- PriceUpdate{Symbol: event.Symbol, Price: event.Price}
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				errHandler := func(err error) {
					log.Printf("WebSocket error: %v", err)
				}

				doneC, _, err := futures.WsAggTradeServe(symbol, wsAggTradeHandler, errHandler)
				if err != nil {
					log.Printf("Failed to connect to WebSocket: %v", err)
					time.Sleep(5 * time.Second) // Wait before trying to reconnect
					continue
				}

				<-doneC
				log.Print("WebSocket disconnected. Attempting to reconnect...")
				time.Sleep(1 * time.Second) // Wait a second before trying to reconnect
			}
		}
	}()
}
