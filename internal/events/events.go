package events

import (
	"context"
	"encoding/json"
	"time"

	"trading-bot/shared/eventdef"

	"github.com/nats-io/nats.go"
)

// Publisher is a NATS publisher
// It is used to publish events to NATS
// It is a wrapper around the nats.Conn
// It is safe for concurrent use

const (
	// connectWait is the time to wait for a connection to be established
	connectWait = 5 * time.Second
	// maxReconnects is the number of times to reconnect
	maxReconnects = 5
	// reconnectWait is the time to wait between reconnects
	reconnectWait = 1 * time.Second
)

type Publisher interface {
	Publish(ctx context.Context, subject string, event eventdef.Event) error
	Close()
}

type natsPublisher struct {
	nc *nats.Conn
}

func NewPublisher(natsURL string, opts ...nats.Option) (Publisher, error) {
	opts = append(opts, nats.Timeout(connectWait))
	opts = append(opts, nats.MaxReconnects(maxReconnects))
	opts = append(opts, nats.ReconnectWait(reconnectWait))

	nc, err := nats.Connect(natsURL, opts...)
	if err != nil {
		return nil, err
	}

	return &natsPublisher{nc: nc}, nil
}

func (p *natsPublisher) Publish(ctx context.Context, subject string, event eventdef.Event) error {
	bytes, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return p.nc.Publish(subject, bytes)
}

func (p *natsPublisher) Close() {
	p.nc.Close()
}
