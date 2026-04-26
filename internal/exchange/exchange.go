package exchange

import (
	"trading-bot/internal/config"
)

// Client is a wrapper for the exchange clients.
type Client struct {
	*RestClient
	*WebsocketClient
}

// New creates a new exchange client.
func New(cfg *config.Config, apiKey, apiSecret string) (*Client, error) {
	restClient := NewRestClient(apiKey, apiSecret)
	wsClient := NewWebsocketClient()

	return &Client{
		RestClient:      restClient,
		WebsocketClient: wsClient,
	}, nil
}
