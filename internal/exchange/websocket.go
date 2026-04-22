package exchange

import (
	"context"
	"time"

	"github.com/adshao/go-binance/v2/futures"
	"go.uber.org/zap"
)

// PriceUpdate represents a single price update from the exchange.
// It will be expanded later.
type PriceUpdate struct {
	Symbol string
	Price  string
}

// WebsocketClient handles the connection to the Binance WebSocket API.
type WebsocketClient struct {
	logger *zap.SugaredLogger
}

// NewWebsocketClient creates a new WebsocketClient.
func NewWebsocketClient(logger *zap.SugaredLogger) *WebsocketClient {
	return &WebsocketClient{logger: logger}
}

// Start starts the WebSocket stream for the given symbol.
func (c *WebsocketClient) Start(ctx context.Context, symbol string, priceCh chan<- PriceUpdate) {
	wsAggTradeHandler := func(event *futures.WsAggTradeEvent) {
		c.logger.Infow("Received aggregate trade event", "symbol", event.Symbol, "price", event.Price)
		priceCh <- PriceUpdate{Symbol: event.Symbol, Price: event.Price}
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				errHandler := func(err error) {
					c.logger.Errorw("WebSocket error", "error", err)
				}

				doneC, _, err := futures.WsAggTradeServe(symbol, wsAggTradeHandler, errHandler)
				if err != nil {
					c.logger.Errorw("Failed to connect to WebSocket", "error", err)
					time.Sleep(5 * time.Second) // Wait before trying to reconnect
					continue
				}

				<-doneC
				c.logger.Info("WebSocket disconnected. Attempting to reconnect...")
				time.Sleep(1 * time.Second) // Wait a second before trying to reconnect
			}
		}
	}()
}
