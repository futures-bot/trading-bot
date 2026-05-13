# Trading Bot Analytics API

The bot exposes a **REST API** (HTTP only, no WebSocket) on `0.0.0.0:8080` for real-time analytics and dashboard integration. All endpoints return JSON. Data retrieval is read-only via HTTP GET requests with optional query parameters.

## Base URL
```
http://localhost:8080
```

When running on GCP VM:
```
http://35.196.16.129:8080  (or your VM's external IP)
```

## API Type & Data Retrieval

- **Protocol**: REST (HTTP only)
- **Methods**: GET only (read-only)
- **Data Format**: JSON
- **Authentication**: Optional API key (header-based, client-side only)
- **Polling**: No WebSocket—dashboard polls endpoints at regular intervals (typically 5 seconds)

## Endpoints

### 1. Health Check
Check if the bot and API are running.

```http
GET /api/health
```

**Response:**
```json
{
  "status": "ok",
  "version": "4.0.0"
}
```

---

### 2. Latest Trades
Get the most recent trades from all sessions.

```http
GET /api/trades?limit=50
```

**Query Parameters:**
- `limit` (optional): Number of trades to return (default: 50, max: 500)

**Response:**
```json
[
  {
    "id": 42,
    "symbol": "SOLUSDT",
    "side": "BUY",
    "entry_price": "94.87",
    "exit_price": "94.65",
    "profit": "-0.1040",
    "exit_reason": "STOP_LOSS",
    "created_at": "2026-05-12T20:56:32Z"
  },
  ...
]
```

---

### 3. Sessions
Get all trading sessions with analytics.

```http
GET /api/sessions?limit=50
```

**Query Parameters:**
- `limit` (optional): Number of sessions to return (default: 50, max: 500)

**Response:**
```json
[
  {
    "id": 1,
    "mode": "paper",
    "symbol": "SOLUSDT",
    "duration_secs": 3600,
    "total_trades": 6,
    "wins": 4,
    "losses": 2,
    "net_pnl": 0.63,
    "final_balance": 1000.63,
    "profit_factor": 4.00,
    "max_drawdown": 0.2,
    "sharpe_ratio": 1.25,
    "expectancy": 0.105,
    "status": "completed",
    "created_at": "2026-05-12T20:21:48Z"
  },
  ...
]
```

---

### 4. Latest Session by Mode
Get the most recent session for each trading mode (testnet, paper, backtest).

```http
GET /api/sessions/latest
```

**Response:**
```json
{
  "testnet": {
    "id": 5,
    "mode": "testnet",
    "symbol": "SOLUSDT",
    "duration_secs": 1200,
    "total_trades": 2,
    "wins": 1,
    "losses": 1,
    "net_pnl": 0.15,
    "final_balance": 4997.57,
    ...
  },
  "paper": {
    "id": 4,
    "mode": "paper",
    ...
  },
  "backtest": {
    "id": 3,
    "mode": "backtest",
    ...
  }
}
```

---

### 5. Overall Statistics
Get aggregate performance stats across all trades.

```http
GET /api/stats
```

**Response:**
```json
{
  "total_trades": 127,
  "win_rate": 52.76,
  "total_pnl": 45.82,
  "paper_sessions": 8,
  "testnet_sessions": 5,
  "backtest_sessions": 12
}
```

---

### 6. Performance Metrics
Get detailed performance analysis for a specific symbol or all trades.

```http
GET /api/performance?mode=all
```

**Query Parameters:**
- `mode` (optional): Filter by symbol prefix (e.g., "SOL" for SOLUSDT, or "all" for all trades)

**Response:**
```json
{
  "mode": "all",
  "symbol": "",
  "total_trades": 127,
  "wins": 67,
  "losses": 60,
  "win_rate": 52.76,
  "net_pnl": 45.82,
  "avg_win": 1.245,
  "avg_loss": -0.892,
  "profit_factor": 1.87,
  "expectancy": 0.361,
  "max_drawdown": 2.15,
  "sharpe_ratio": 0.95,
  "best_trade": 5.20,
  "worst_trade": -1.50
}
```

---

## Usage Examples

### Fetch latest trades from the last hour
```bash
curl http://localhost:8080/api/trades?limit=100
```

### Monitor latest session in real-time
```bash
watch -n 5 'curl -s http://localhost:8080/api/sessions/latest | jq ".paper"'
```

### Get performance metrics every minute
```bash
while true; do
  curl -s http://localhost:8080/api/performance | jq .
  sleep 60
done
```

### JavaScript/fetch example
```javascript
async function getLatestSession() {
  const res = await fetch('http://localhost:8080/api/sessions/latest');
  const data = await res.json();
  console.log('Paper session:', data.paper);
  console.log('Testnet session:', data.testnet);
}

setInterval(getLatestSession, 5000); // Update every 5 seconds
```

---

## Response Codes

| Code | Meaning |
|------|---------|
| 200  | Success |
| 400  | Bad request (invalid parameters) |
| 404  | Not found (no data for query) |
| 500  | Server error |

---

## CORS & Real-Time Streaming

The API now includes **CORS headers** and **SSE (Server-Sent Events)** for real-time data streaming:

### CORS Headers

All endpoints respond with CORS headers for cross-origin requests:

```
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, OPTIONS
Access-Control-Allow-Headers: Content-Type, Authorization
```

This allows dashboards on different domains (e.g., Vercel, localhost) to request data directly.

### Server-Sent Events (SSE)

For real-time updates, use the streaming endpoint:

```http
GET /api/stream
```

Response headers:
```
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
```

**Browser example**:
```javascript
const eventSource = new EventSource('http://35.196.16.129:8080/api/stream');

eventSource.addEventListener('message', (event) => {
  const streamEvent = JSON.parse(event.data);
  console.log('Event type:', streamEvent.type); // 'stats', 'sessions', or 'trades'
  console.log('Data:', streamEvent.data);
});
```

See [STREAMING.md](./STREAMING.md) for full documentation.

### Performance

- **Streaming**: 1 connection, instant updates, lower bandwidth
- **Polling (fallback)**: GET `/api/stats`, `/api/trades`, `/api/sessions` every 5 seconds
- **Dashboard refresh**: 5-second polling interval (can be customized)

---

## Dashboard Implementation

The official Next.js dashboard is available at:
- **Repository**: https://github.com/bercho001-cpu/trading-bot-dashboard
- **Live**: https://trading-bot-dashboard-qnxn9eq7y-fersoria001s-projects.vercel.app

It uses the `useStream` hook for real-time SSE updates:

```typescript
import { useStream } from '@/hooks/useStream';

export function Dashboard() {
  const { stats, sessions, trades, connected } = useStream();
  
  return (
    <div>
      <div>{connected ? '🟢 Live' : '🔴 Offline'}</div>
      <div>Total Trades: {stats?.total_trades}</div>
      <div>Win Rate: {stats?.win_rate.toFixed(2)}%</div>
      <div>PnL: ${stats?.total_pnl.toFixed(2)}</div>
      
      <SessionsTable data={sessions} />
      <TradesTable data={trades} />
    </div>
  );
}
```

### Quick HTML Example (Polling)

For a simple read-only dashboard without SSE:

```html
<!DOCTYPE html>
<html>
<head>
  <title>Trading Bot Dashboard</title>
  <style>
    body { font-family: monospace; margin: 20px; background: #1a1a1a; color: #0f0; }
    .stat { display: inline-block; margin: 10px; padding: 10px; border: 1px solid #0f0; }
    .positive { color: #0f0; }
    .negative { color: #f00; }
  </style>
</head>
<body>
  <h1>Trading Bot Analytics</h1>
  <div id="stats"></div>
  
  <script>
    const API_URL = 'http://35.196.16.129:8080';
    
    async function updateDashboard() {
      try {
        const stats = await fetch(`${API_URL}/api/stats`).then(r => r.json());
        document.getElementById('stats').innerHTML = `
          <div class="stat">
            <b>Trades:</b> ${stats.total_trades}
          </div>
          <div class="stat">
            <b>Win Rate:</b> <span class=${stats.win_rate > 50 ? 'positive' : 'negative'}>${stats.win_rate.toFixed(2)}%</span>
          </div>
          <div class="stat">
            <b>PnL:</b> <span class=${stats.total_pnl > 0 ? 'positive' : 'negative'}>$${stats.total_pnl.toFixed(2)}</span>
          </div>
        `;
      } catch (err) {
        console.error('Update failed:', err);
      }
    }
    
    updateDashboard();
    setInterval(updateDashboard, 5000); // Refresh every 5 seconds
  </script>
</body>
</html>
```

---

## Local Testing

```bash
# Terminal 1: Run the bot
make run

# Terminal 2: Test endpoints
curl http://localhost:8080/api/health
curl http://localhost:8080/api/stats
curl http://localhost:8080/api/sessions/latest
```

---

## Authentication

Currently there is **no authentication**. If you need to secure the API:

1. **Firewall-based**: Restrict bot API access to known IPs (recommended, already in use)
   ```bash
   gcloud compute firewall-rules update allow-dashboard-api \
     --source-ranges=YOUR_IP/32,VERCEL_IP/32
   ```

2. **Token-based**: Add API key validation
   ```http
   GET /api/stats?key=YOUR_SECRET_KEY
   ```

3. **Header-based**: Check `Authorization` header
   ```javascript
   fetch('http://api.example.com/api/stats', {
     headers: { 'Authorization': 'Bearer YOUR_TOKEN' }
   })
   ```

4. **Proxy**: Run behind nginx/reverse-proxy with authentication

---

## Bot Management (Shutdown & Restart)

### Graceful Shutdown

To stop the bot gracefully, send a termination signal:

```bash
# If running locally in foreground
# Press Ctrl+C in the terminal where the bot is running

# If running as a systemd service on GCP VM
gcloud compute ssh trading-bot --zone=us-east1-c --command='sudo systemctl stop trading-bot'

# If running as background process
pkill -SIGTERM trading-bot
# or by PID
kill -SIGTERM <PID>
```

**What happens on graceful shutdown:**
- Bot receives `SIGINT` or `SIGTERM` signal
- Active trading sessions are allowed to complete
- Any open positions are closed
- Current session analytics are saved to database
- API server stops accepting requests
- Process exits cleanly

### Restart the Bot

```bash
# If running as systemd service (recommended on GCP)
gcloud compute ssh trading-bot --zone=us-east1-c --command='sudo systemctl restart trading-bot'

# Or stop and start separately
gcloud compute ssh trading-bot --zone=us-east1-c --command='sudo systemctl stop trading-bot && sudo systemctl start trading-bot'

# If running manually, stop with Ctrl+C and restart the command
./trading-bot run
```

### Check Bot Status

```bash
# Via systemd (on GCP VM)
gcloud compute ssh trading-bot --zone=us-east1-c --command='sudo systemctl status trading-bot'

# Via API health endpoint
curl http://35.196.16.129:8080/api/health

# Get latest session status
curl http://35.196.16.129:8080/api/sessions/latest
```

### View Bot Logs

```bash
# If running as systemd service
gcloud compute ssh trading-bot --zone=us-east1-c --command='sudo journalctl -u trading-bot -f'

# Last 50 lines
gcloud compute ssh trading-bot --zone=us-east1-c --command='sudo journalctl -u trading-bot -n 50'

# Or check database logs (all logs are persisted)
# Query the bot_logs, trade_logs, market_pulse_logs tables in Supabase
```

### Emergency Stop (Hard Shutdown)

If the bot is unresponsive, force kill it:

```bash
# Via SSH
gcloud compute ssh trading-bot --zone=us-east1-c --command='pkill -9 trading-bot'

# Or if it's a systemd service
gcloud compute ssh trading-bot --zone=us-east1-c --command='sudo systemctl kill -s KILL trading-bot'
```

⚠️ **Warning**: Hard shutdown may leave open positions on testnet or incomplete session data in the database. Use graceful shutdown when possible.

