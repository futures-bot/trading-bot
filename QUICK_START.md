# Quick Start Guide

Get the trading bot running in 5 minutes.

## Prerequisites

- Go 1.21+
- PostgreSQL database (Supabase recommended for free tier)
- Binance Futures Testnet API keys (for testnet trading)
- Telegram Bot token (optional, for notifications)

## Local Setup

### 1. Clone & Install

```bash
git clone https://github.com/bercho001-cpu/trading-bot.git
cd trading-bot
go mod download
go build -o trading-bot
```

### 2. Configure .env

```bash
cat > .env << EOF
DATABASE_URL="host=db.supabase.co port=5432 user=postgres password=YOUR_PASSWORD dbname=postgres sslmode=require"
BINANCE_API_KEY=your_testnet_api_key
BINANCE_SECRET_KEY=your_testnet_secret_key
TELEGRAM_BOT_TOKEN=your_telegram_bot_token    # Optional
TELEGRAM_CHAT_ID=your_telegram_chat_id        # Optional
EOF
```

### 3. Configure config.yaml (optional)

Default settings are in `config.yaml`. Adjust if needed:
- `symbol`: Trading pair (default: XRPUSDT)
- `leverage`: 1-10x (default: 10)
- `session_budget`: USDT per trade (default: 100)
- `ema_fast`, `ema_slow`: EMA periods (default: 9, 21)

### 4. Run Commands

**Backtest on historical data:**
```bash
./trading-bot scrape              # Download 24h of klines + auto-backtest
./trading-bot backtest            # Run backtest on downloaded data
```

**Paper trading (simulated, with real prices):**
```bash
./trading-bot paper               # Auto-rotating 1hr sessions
```

**Testnet trading (real orders, no real money):**
```bash
./trading-bot testnet             # Auto-rotating 1hr sessions on Binance testnet
```

**Run everything (24/7 on GCP):**
```bash
./trading-bot run                 # Scrape + paper + testnet + backtest concurrently
```

## Cloud Deployment (GCP)

### 1. Create GCP VM

```bash
gcloud compute instances create trading-bot \
  --zone=us-east1-c \
  --machine-type=e2-micro \
  --image=debian-11
```

### 2. SSH & Install Bot

```bash
gcloud compute ssh trading-bot --zone=us-east1-c
# On the VM:
curl -OL https://go.dev/dl/go1.21.linux-amd64.tar.gz
tar -C /usr/local -xzf go1.21.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

git clone https://github.com/bercho001-cpu/trading-bot.git
cd trading-bot
go build -o trading-bot
```

### 3. Configure & Run as Service

```bash
# Upload .env file
gcloud compute scp .env trading-bot:~

# Create systemd service
gcloud compute ssh trading-bot --zone=us-east1-c --command='
cat | sudo tee /etc/systemd/system/trading-bot.service << EOFSERVICE
[Unit]
Description=Trading Bot
After=network.target

[Service]
User=fer
WorkingDirectory=/home/fer
ExecStart=/home/fer/trading-bot run
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOFSERVICE
'

# Start the service
gcloud compute ssh trading-bot --zone=us-east1-c --command='sudo systemctl enable trading-bot && sudo systemctl start trading-bot'
```

### 4. Monitor

```bash
# Check status
gcloud compute ssh trading-bot --zone=us-east1-c --command='sudo systemctl status trading-bot'

# View logs
gcloud compute ssh trading-bot --zone=us-east1-c --command='sudo journalctl -u trading-bot -f'

# Check API health
curl http://35.196.16.129:8080/api/health
```

## Dashboard

Deploy the dashboard on Vercel for real-time monitoring:

```bash
git clone https://github.com/bercho001-cpu/trading-bot-dashboard.git
cd trading-bot-dashboard

# Configure .env.local
cat > .env.local << EOF
NEXT_PUBLIC_API_URL=http://YOUR_BOT_IP:8080
NEXT_PUBLIC_API_KEY=optional_api_key
EOF

# Run locally
npm run dev
# Visit http://localhost:3000

# Deploy to Vercel
vercel --prod
```

## API Endpoints

All endpoints are **REST** (HTTP GET only), return JSON:

- `GET /api/health` — Check if bot is running
- `GET /api/stats` — Overall statistics
- `GET /api/trades?limit=50` — Latest trades
- `GET /api/sessions?limit=50` — All sessions
- `GET /api/sessions/latest` — Latest session per mode
- `GET /api/performance?mode=all` — Performance metrics

See [API.md](./API.md) for full documentation.

## Troubleshooting

### "Connection timeout" to Binance

Make sure your `.env` has:
- `BINANCE_API_KEY` and `BINANCE_SECRET_KEY` (testnet keys from https://testnet.binancefuture.com)
- Testnet API endpoint is used by default (no firewall issues)

### Database connection error

1. Check `DATABASE_URL` format: `host=... port=... user=... password=... dbname=... sslmode=require`
2. Make sure Supabase instance is running
3. Verify firewall allows your IP

### Bot crashes with "panic: invalid memory address"

This usually means the database connection failed or a required API key is missing. Check logs:

```bash
gcloud compute ssh trading-bot --zone=us-east1-c --command='sudo journalctl -u trading-bot -n 100'
```

## Next Steps

1. ✅ Backtest the strategy
2. ✅ Run paper trading for validation
3. ✅ Deploy to GCP VM
4. ✅ Monitor via dashboard on Vercel
5. 📋 Later: Add authentication to dashboard
6. 📋 Later: Add rate limiting
7. 📋 Later: Add more indicators/strategies

## Resources

- [API Documentation](./API.md) — Full REST API reference
- [README](./README.md) — Strategy details & architecture
- [Deployment Guide](./DEPLOYMENT_GUIDE.md) — GCP setup & firewall config
- [Dashboard Deployment](./VERCEL_SETUP.md) — Vercel setup guide

## Support

For issues, check:
1. Bot logs: `journalctl -u trading-bot`
2. Database logs: Query `bot_logs`, `trade_logs` tables in Supabase
3. API health: `curl http://localhost:8080/api/health`
4. GitHub Issues: https://github.com/bercho001-cpu/trading-bot/issues
