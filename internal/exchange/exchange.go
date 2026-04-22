package exchange

import (
	"trading-bot/internal/config"

	"go.uber.org/zap"
)

// Client is a wrapper for the exchange clients.
type Client struct {
	*RestClient
	*WebsocketClient
}

// New creates a new exchange client.
func New(cfg *config.Config, logger *zap.SugaredLogger) (*Client, error) {
	restClient := NewRestClient()
	wsClient := NewWebsocketClient(logger)

	return &Client{
		RestClient:      restClient,
		WebsocketClient: wsClient,
	}, nil
}
