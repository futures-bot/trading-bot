# Real-Time Streaming API (SSE)

The bot now supports **Server-Sent Events (SSE)** for real-time data streaming to dashboards.

## Why SSE?

- **One-way streaming**: Perfect for bot → dashboard data flow
- **Built-in to HTTP**: Works in all browsers, no special protocols
- **CORS-friendly**: Works cross-origin with proper headers (firewall-permitting)
- **Simple**: Native browser `EventSource` API
- **Lower overhead** than WebSocket for read-only data

## Endpoint

```
GET /api/stream
```

**Headers**:
- `Content-Type: text/event-stream`
- `Cache-Control: no-cache`
- `Connection: keep-alive`
- `Access-Control-Allow-Origin: *` (CORS)

## Response Format

Server sends events as **SSE data packets**:

```
data: {"type":"stats","timestamp":"2026-05-13T02:42:41Z","data":{...}}

data: {"type":"sessions","timestamp":"2026-05-13T02:42:41Z","data":[...]}

data: {"type":"trades","timestamp":"2026-05-13T02:42:41Z","data":[...]}
```

Each event is a JSON object with:
- `type`: Event type (`stats`, `sessions`, `trades`)
- `timestamp`: ISO 8601 timestamp
- `data`: Payload (varies by type)

## Event Types

### 1. Stats Event
```json
{
  "type": "stats",
  "timestamp": "2026-05-13T02:42:41Z",
  "data": {
    "total_trades": 221,
    "win_rate": 54.75,
    "total_pnl": -0.015
  }
}
```

### 2. Sessions Event
```json
{
  "type": "sessions",
  "timestamp": "2026-05-13T02:42:41Z",
  "data": [
    {
      "ID": 36,
      "mode": "backtest",
      "symbol": "SOLUSDT",
      "total_trades": 3,
      "wins": 1,
      "losses": 2,
      "net_pnl": -0.0728,
      "profit_factor": 0.641,
      "max_drawdown": 0.2028,
      "sharpe_ratio": -0.182,
      "expectancy": -0.0243,
      "status": "completed",
      "CreatedAt": "2026-05-13T02:42:18.046327Z"
    },
    ...
  ]
}
```

### 3. Trades Event
```json
{
  "type": "trades",
  "timestamp": "2026-05-13T02:42:41Z",
  "data": [
    {
      "ID": 261,
      "symbol": "SOLUSDT",
      "side": "SELL",
      "entry_price": "94.48",
      "exit_price": "94.68",
      "profit": "-0.104",
      "exit_reason": "STOP_LOSS",
      "CreatedAt": "2026-05-13T02:42:17.547856Z"
    },
    ...
  ]
}
```

## Browser Usage

### Vanilla JavaScript

```javascript
const eventSource = new EventSource('http://bot-api.example.com/api/stream');

eventSource.addEventListener('message', (event) => {
  const streamEvent = JSON.parse(event.data);
  
  if (streamEvent.type === 'stats') {
    console.log('Updated stats:', streamEvent.data);
    // Update UI with new stats
  }
  
  if (streamEvent.type === 'trades') {
    console.log('New trades:', streamEvent.data);
    // Update trades table
  }
  
  if (streamEvent.type === 'sessions') {
    console.log('Sessions:', streamEvent.data);
    // Update sessions list
  }
});

eventSource.addEventListener('error', () => {
  console.error('Stream connection error');
  eventSource.close();
});
```

### React Hook (useStream)

The dashboard uses a custom `useStream` hook for React integration:

```typescript
import { useStream } from '@/hooks/useStream';

export function Dashboard() {
  const { stats, sessions, trades, connected } = useStream();
  
  return (
    <div>
      <div>{connected ? '🟢 Live' : '🔴 Offline'}</div>
      <div>Total Trades: {stats?.total_trades}</div>
      <SessionsTable data={sessions} />
      <TradesTable data={trades} />
    </div>
  );
}
```

## How It Works

1. **Browser opens connection**: `new EventSource('/api/stream')`
2. **Server streams events**: Sends `stats`, `sessions`, `trades` in one go
3. **Browser receives each event**: Parses JSON and updates UI
4. **Connection stays open**: No polling needed
5. **Auto-reconnect**: Browser automatically reconnects on disconnect

## Performance

- **One HTTP connection** instead of polling 4 endpoints every 5 seconds
- **Instant data** (no polling interval delay)
- **Lower bandwidth**: Single connection, not repeated full requests
- **Battery friendly**: No polling wake-ups

## CORS Support

The API now includes CORS headers for cross-origin requests:

```
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, OPTIONS
Access-Control-Allow-Headers: Content-Type, Authorization
```

This allows dashboards on different origins (e.g., Vercel) to connect directly.

## Fallback: REST Polling

If SSE doesn't work, use the REST endpoints with polling:

```javascript
async function poll() {
  const [stats, sessions, trades] = await Promise.all([
    fetch('/api/stats').then(r => r.json()),
    fetch('/api/sessions?limit=10').then(r => r.json()),
    fetch('/api/trades?limit=10').then(r => r.json()),
  ]);
  
  // Update UI
  
  setTimeout(poll, 5000); // Poll every 5 seconds
}

poll();
```

## Testing

### Local Bot

```bash
# Test streaming from the bot
curl -s http://localhost:8080/api/stream | head -10
```

Output:
```
data: {"type":"stats",...}

data: {"type":"sessions",...}

data: {"type":"trades",...}
```

### With Firewall

If behind firewall, make sure CORS headers are present:

```bash
curl -i http://35.196.16.129:8080/api/stream | head -20
```

Look for:
```
Access-Control-Allow-Origin: *
Content-Type: text/event-stream
```

## Troubleshooting

### "Connection refused" or timeout

1. Check bot is running: `curl http://35.196.16.129:8080/api/health`
2. Check firewall allows port 8080 from your IP
3. Verify CORS headers: `curl -i http://.../api/stream | head -20`

### "Failed to parse event"

Make sure `EventSource` is decoding JSON correctly. Check browser console for parse errors.

### Dashboard shows "Offline"

- Network issue or firewall blocking
- Check browser DevTools Network tab for `/api/stream` request
- Verify CORS headers in response
- Try REST polling as fallback

## Future Improvements

- [ ] Persistent connections with exponential backoff
- [ ] Message compression (gzip for large payloads)
- [ ] Incremental updates (send only changed data)
- [ ] Subscribe/filter by event type
- [ ] Heartbeat pings to detect dead connections
