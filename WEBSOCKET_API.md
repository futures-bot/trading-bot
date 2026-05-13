# WebSocket API

## Connection

The WebSocket server is available at `/ws`.

## Subscribing to Topics

To receive data, you need to subscribe to one or more topics. You can do this by sending a JSON message to the WebSocket server with the following format:

```json
{
  "action": "subscribe",
  "topic": "<topic_name>"
}
```

To unsubscribe, send:

```json
{
  "action": "unsubscribe",
  "topic": "<topic_name>"
}
```

## Available Topics

- `stats`: General performance statistics.
- `trades`: Recent trades.
- `sessions`: Recent sessions.
- `latest-session`: The latest session for each mode (testnet, paper, backtest).

## Message Format

Messages from the server have the following format:

```json
{
  "topic": "<topic_name>",
  "data": { ... }
}
```

- `topic`: The topic of the data.
- `data`: The data payload, which is a JSON object. The structure of this object depends on the topic.

### Topic: `stats`

**Data Structure:** `StatsResponse`

```json
{
  "total_trades": 0,
  "win_rate": 0,
  "total_pnl": 0,
  "paper_sessions": 0,
  "testnet_sessions": 0,
  "backtest_sessions": 0
}
```

### Topic: `trades`

**Data Structure:** `[]TradeResponse`

```json
[
  {
    "id": 0,
    "symbol": "",
    "side": "",
    "entry_price": "0",
    "exit_price": "0",
    "profit": "0",
    "exit_reason": "",
    "created_at": ""
  }
]
```

### Topic: `sessions`

**Data Structure:** `[]SessionResponse`

```json
[
  {
    "id": 0,
    "mode": "",
    "symbol": "",
    "duration_secs": 0,
    "total_trades": 0,
    "wins": 0,
    "losses": 0,
    "net_pnl": 0,
    "final_balance": 0,
    "profit_factor": 0,
    "max_drawdown": 0,
    "sharpe_ratio": 0,
    "expectancy": 0,
    "status": "",
    "created_at": ""
  }
]
```

### Topic: `latest-session`

**Data Structure:** `LatestSessionResponse`

```json
{
  "testnet": { ... }, // SessionResponse
  "paper": { ... },   // SessionResponse
  "backtest": { ... } // SessionResponse
}
```
