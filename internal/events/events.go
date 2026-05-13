package events

import (
	"context"
	"encoding/json"
	"time"

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

type Publisher struct {
	nc *nats.Conn
}

func NewPublisher(natsURL string, opts ...nats.Option) (*Publisher, error) {
	opts = append(opts, nats.Timeout(connectWait))
	opts = append(opts, nats.MaxReconnects(maxReconnects))
	opts = append(opts, nats.ReconnectWait(reconnectWait))

	nc, err := nats.Connect(natsURL, opts...)
	if err != nil {
		return nil, err
	}

	return &Publisher{nc: nc}, nil
}

func (p *Publisher) Publish(ctx context.Context, subject string, data interface{}) error {
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return p.nc.Publish(subject, bytes)
}

func (p *Publisher) Close() {
	p.nc.Close()
}
