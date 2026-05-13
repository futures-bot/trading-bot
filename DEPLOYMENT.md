# Deployment Guide

Complete guide to deploying the trading bot on Google Cloud Platform with real-time monitoring on Vercel.

## Architecture

```
Your Machine (192.168.1.39)
    ├── npm run dev (localhost:3000)
    │   └── Fetches → GCP Bot API (35.196.16.129:8080)
    │
GitHub
    └── git push
        └── Triggers Vercel deployment
            └── Vercel Edge (64.29.17.195, 216.198.79.195)
                └── Fetches → GCP Bot API (35.196.16.129:8080)

GCP VM (us-east1-c, e2-micro)
    └── trading-bot run
        ├── API Server (0.0.0.0:8080)
        ├── Paper trader
        ├── Testnet trader
        ├── Scraper
        └── Backtest engine
        
        All persist to: Supabase PostgreSQL
```

## Prerequisites

- GCP Project with billing enabled (free tier micro VM)
- Supabase account (free tier PostgreSQL)
- Vercel account (free tier)
- GitHub account with SSH key configured
- Binance Testnet account (free)
- `gcloud` CLI installed and authenticated

## Step 1: Create GCP VM

```bash
# Create VM (e2-micro free tier)
gcloud compute instances create trading-bot \
  --zone=us-east1-c \
  --machine-type=e2-micro \
  --image-family=debian-11 \
  --image-project=debian-cloud \
  --scopes=default,cloud-platform

# Get the external IP
gcloud compute instances describe trading-bot --zone=us-east1-c \
  --format='value(networkInterfaces[0].accessConfigs[0].natIP)'
# Note: Should be something like 35.196.16.129
```

## Step 2: Set Up Firewall Rules

Allow HTTP traffic to port 8080 from your local machine and Vercel edge IPs:

```bash
# Create firewall rule
gcloud compute firewall-rules create allow-dashboard-api \
  --allow=tcp:8080 \
  --source-ranges=192.168.1.39/32,64.29.17.195/32,216.198.79.195/32 \
  --target-tags=trading-bot \
  --description="Allow dashboard API access"

# Apply tag to VM
gcloud compute instances add-tags trading-bot \
  --zone=us-east1-c \
  --tags=trading-bot
```

## Step 3: SSH & Install Go

```bash
# SSH into VM
gcloud compute ssh trading-bot --zone=us-east1-c

# On the VM, install Go
curl -OL https://go.dev/dl/go1.21.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
go version

# Clone bot repo
git clone https://github.com/bercho001-cpu/trading-bot.git ~/trading-bot-repo
cd ~/trading-bot-repo
```

## Step 4: Build & Deploy Bot

```bash
# Build the binary
go build -o trading-bot

# Test it (should show health)
./trading-bot version
# Output: trading-bot 4.0.0
```

## Step 5: Configure Environment

```bash
# Create .env file with your secrets
cat > .env << 'EOF'
DATABASE_URL="host=db.supabase.co port=5432 user=postgres password=YOUR_PASSWORD dbname=postgres sslmode=require"
BINANCE_API_KEY=your_testnet_api_key
BINANCE_SECRET_KEY=your_testnet_secret_key
TELEGRAM_BOT_TOKEN=your_telegram_token
TELEGRAM_CHAT_ID=123456789
EOF

# Verify database connection
./trading-bot stats
# Should either show stats or "No trades found" (not error)
```

## Step 6: Create Systemd Service

```bash
# Create service file
sudo cat > /etc/systemd/system/trading-bot.service << 'EOF'
[Unit]
Description=Trading Bot - Binance Futures Auto-Trader
After=network.target

[Service]
Type=simple
User=fer
WorkingDirectory=/home/fer/trading-bot-repo
Environment="PATH=/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
ExecStart=/home/fer/trading-bot-repo/trading-bot run
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

# Enable & start service
sudo systemctl daemon-reload
sudo systemctl enable trading-bot
sudo systemctl start trading-bot

# Verify it's running
sudo systemctl status trading-bot
# Should show "active (running)"
```

## Step 7: Deploy Dashboard to Vercel

### Option A: Via CLI

```bash
# On your local machine, clone dashboard repo
git clone https://github.com/bercho001-cpu/trading-bot-dashboard.git
cd trading-bot-dashboard

# Install Vercel CLI
npm install -g vercel

# Deploy
vercel --prod
# This will prompt you to link to your Vercel account
```

### Option B: Via Git Push (Recommended)

```bash
# Push to GitHub (already private, Vercel has access)
git push origin main

# Vercel will auto-deploy within 2-3 minutes
```

The dashboard is now available at: `https://trading-bot-dashboard-xxx.vercel.app`

## Step 8: Configure Dashboard Environment Variables

The dashboard needs to know where the bot API is:

```bash
# If using Vercel UI:
# Go to https://vercel.com/dashboard → trading-bot-dashboard → Settings → Environment Variables
# Add:
NEXT_PUBLIC_API_URL=http://35.196.16.129:8080
NEXT_PUBLIC_API_KEY=optional_api_key

# Then redeploy
vercel --prod
```

If already configured in `.env.local`:
```
NEXT_PUBLIC_API_URL=http://35.196.16.129:8080
NEXT_PUBLIC_API_KEY=52cce22cf7617171c180afcc2fb672f693f9b7cce51c3cc0b1659a8bab690850
```

## Step 9: Verify Everything Works

### Test Bot API

```bash
# From your local machine
curl http://35.196.16.129:8080/api/health
# Response: {"status":"ok","version":"4.0.0"}

curl http://35.196.16.129:8080/api/stats
# Response: {"total_trades":0,"win_rate":0,"total_pnl":0,...}
```

### Test Dashboard Locally

```bash
cd ~/Documents/Dev/trading-bot-dashboard
npm run dev
# Visit http://localhost:3000
# Should show "Live" status and data from bot
```

### Test Dashboard Production

Visit your Vercel URL: `https://trading-bot-dashboard-xxx.vercel.app`
- Should show real-time data
- Should fetch from `http://35.196.16.129:8080`

## Step 10: Monitor Bot

### Check Status

```bash
# SSH into VM and check service
gcloud compute ssh trading-bot --zone=us-east1-c --command='sudo systemctl status trading-bot'

# View live logs
gcloud compute ssh trading-bot --zone=us-east1-c --command='sudo journalctl -u trading-bot -f'

# View recent 50 lines
gcloud compute ssh trading-bot --zone=us-east1-c --command='sudo journalctl -u trading-bot -n 50'
```

### Check API Health

```bash
# From anywhere with firewall access
curl http://35.196.16.129:8080/api/health

# From dashboard (automatic 5s polling)
# Visit Vercel URL to see real-time stats
```

### View Database Data

```bash
# In Supabase:
# Tables → sessions (see all trading sessions)
# Tables → trades (see individual trades)
# Tables → bot_logs (see detailed logs)
```

## Maintenance

### Restart Bot (Graceful)

```bash
gcloud compute ssh trading-bot --zone=us-east1-c --command='sudo systemctl restart trading-bot'
```

### Stop Bot

```bash
gcloud compute ssh trading-bot --zone=us-east1-c --command='sudo systemctl stop trading-bot'
```

### Update Bot Code

```bash
# SSH into VM
gcloud compute ssh trading-bot --zone=us-east1-c

# Pull latest code
cd trading-bot-repo
git pull origin master

# Rebuild
go build -o trading-bot

# Restart service
sudo systemctl restart trading-bot
```

### Update Dashboard

```bash
# On local machine
cd trading-bot-dashboard
git add .
git commit -m "Update"
git push

# Vercel auto-deploys in 2-3 minutes
```

## Troubleshooting

### Bot not starting

```bash
# Check logs
sudo journalctl -u trading-bot -n 100 --no-pager

# Check if port 8080 is in use
sudo netstat -tlnp | grep 8080

# Try running manually to see error
/home/fer/trading-bot-repo/trading-bot run
```

### Dashboard shows "Failed to fetch"

```bash
# 1. Check bot is running
gcloud compute ssh trading-bot --zone=us-east1-c --command='curl http://localhost:8080/api/health'

# 2. Check firewall rule
gcloud compute firewall-rules describe allow-dashboard-api

# 3. Check your IP hasn't changed
# Update firewall if needed:
gcloud compute firewall-rules update allow-dashboard-api \
  --source-ranges=YOUR_NEW_IP/32,64.29.17.195/32,216.198.79.195/32
```

### Database connection refused

```bash
# Check DATABASE_URL format
# Should be: host=... port=... user=... password=... dbname=... sslmode=require
# NOT: postgres://...

# Verify Supabase instance is running
# Test connection
psql "postgresql://user:password@host/dbname"
```

## Performance Tips

- **VM**: e2-micro is sufficient (0.5 VCPU, 1GB RAM)—runs everything under 20% CPU
- **Database**: Supabase free tier (500MB storage) is sufficient for months of data
- **Dashboard**: Vercel free tier is sufficient (unlimited requests)
- **Network**: Firewall rules ensure only authorized IPs can access the bot API

## Security

- ✅ Firewall rules restrict API access to known IPs only
- ✅ All credentials in `.env` (never in git)
- ✅ Supabase connection uses SSL
- ✅ Bot API listens on `0.0.0.0:8080` (bind to all, but firewall restricts access)
- ✅ Dashboard will be protected by authentication (future)

## Next Steps

1. ✅ Bot deployed and running 24/7
2. ✅ Dashboard deployed on Vercel
3. ✅ Firewall configured for security
4. 📋 Add authentication to dashboard
5. 📋 Add rate limiting
6. 📋 Add email alerts
7. 📋 Add custom domain for dashboard

---

**Done!** Your bot is now running 24/7 on GCP, with real-time monitoring on Vercel.
