package engine

import "context"

// Status represents the status of a trader.
type Status struct {
	IsRunning bool    `json:"is_running"`
	Uptime    string  `json:"uptime"`
	Balance   float64 `json:"balance"`
}

// Trader is the interface for all trading engines.
type Trader interface {
	Start(ctx context.Context)
	Stop()
	GetStatus() Status
}
