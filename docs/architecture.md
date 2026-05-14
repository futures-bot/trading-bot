# Architecture

## Services

### bot
Responsible for:
- market data
- strategy execution
- order execution

### analytics
Responsible for:
- trade persistence
- metrics
- dashboards
- websocket updates

## Communication

NATS subjects:
- trade.opened
- trade.closed
- signal.generated

## Rules

- bot never reads analytics database
- analytics never sends orders
- all communication event-driven
