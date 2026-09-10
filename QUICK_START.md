# Operations Runbook: Developer & Operator Quick Start

This guide covers local environment provisioning, project dependency compilation, secrets configuration, and CLI runtime command flows.

---

## 1. System Requirements

*   **Go Compiler:** Version `1.22` or greater (leveraging standard performance profiles, type constraints, and structured logging).
*   **Database:** PostgreSQL 14+ database instance (e.g., Supabase or corporate RDS).
*   **Event Broker:** NATS Server (local or managed instance) for event streaming telemetry.
*   **Exchange Sandbox:** Active Binance Futures Testnet account API credentials.

---

## 2. Setting Up the Local Workspace

### A. Clone and Compile
Clone the repository and build the production-ready CLI binary locally:

```bash
# Clone the repository
git clone https://github.com/bercho001-cpu/trading-bot.git
cd trading-bot

# Synchronize modules and compile the executable
go mod download
go build -o trading-bot main.go
```

### B. Configure System Secrets (`.env`)
Create a custom `.env` file at the root of your project directory. This file is excluded from git tracking to prevent credential leaks.

```bash
# PostgreSQL Connection Configuration (Strict Key-Value Format)
DATABASE_URL="host=your-db-host.supabase.co port=5432 user=postgres password=your_secure_password dbname=postgres sslmode=require"

# NATS Event Broker Configuration
NATS_URL="nats://localhost:4222"
NATS_CREDS_FILE="" # Leave empty if using unauthenticated local NATS; provide file path for TLS credentials in production

# Exchange Sandbox Credentials (from https://testnet.binancefuture.com)
BINANCE_API_KEY="your_binance_testnet_api_key_here"
BINANCE_SECRET_KEY="your_binance_testnet_secret_key_here"

# Operational Notifications (Optional)
TELEGRAM_BOT_TOKEN="your_bot_api_token"
TELEGRAM_CHAT_ID="your_channel_or_group_numeric_id"
```

*Note: In the `DATABASE_URL` format, always use key-value format as complex password strings containing special symbols can disrupt standard URL-parsing utilities.*

### C. Verify Static Strategy Parameters (`config.yaml`)
Validate indicators and risk rules inside `config.yaml`:

```yaml
symbol: "XRPUSDT"              # Trading pair
leverage: 10                  # Target contract leverage
session_budget: 100.0         # USDT margin sizing per trade
ema_fast: 9                   # Fast EMA window
ema_slow: 21                  # Slow EMA window
min_ema_gap: 0.001            # Threshold delta separating EMAs
take_profit_pct: 0.5          # Take profit threshold (%)
stop_loss_pct: 0.2            # Stop loss safety cap (%)
break_even_trigger_pct: 0.1   # Threshold to shift SL to entry (%)
trail_distance_pct: 0.1       # Trailing buffer distance (%)
confirmation_count: 3         # Consecutive ticks needed to confirm entry signals
min_profit_for_flip_exit: 0.1 # Minimum profit to allow signal flip exits
paper_balance: 1000.0         # Virtual balance initialization cap
loss_cooldown: 60             # Operational cooldown ticks after loss
win_cooldown: 60              # Operational cooldown ticks after win
session_duration_min: 60      # Auto-rotation interval length (minutes)
backtest_file: "data/trades.jsonl" # Default backup file path for offline simulation
```

---

## 3. Basic CLI Command Workflows

To ensure proper data flows, complete operations in this sequence:

### Step 1: Historical Data Ingestion (Scraping)
Scrape candle data from public Binance APIs to prep your database for strategic backtests:

```bash
# Scrape the symbol specified in config.yaml and trigger auto-backtesting on completion
./trading-bot scrape

# Scrape specific asset symbols concurrently
./trading-bot scrape BTCUSDT ETHUSDT SOLUSDT
```

### Step 2: Off-line Backtesting Simulation
Test strategic parameters over historical datasets without committing capital:

```bash
# Run backtest using historical kline rows saved in the DB
./trading-bot backtest

# Run backtest using local JSONL files
./trading-bot backtest data/market_pulse.jsonl
```

### Step 3: Real-Time Paper Sandbox Simulation
Validate latency and indicator computations using real-time Binance WebSocket price feeds:

```bash
# Paper trading with automated hourly session rotations and simulated orders
./trading-bot paper
```

### Step 4: Active Staging Execution
Trigger dry-run trading on the actual Binance Testnet using real API connections:

```bash
# Execute dry-run market orders on the Binance Futures Testnet sandbox
./trading-bot testnet
```

### Step 5: High-Resilience Continuous Operation (24/7 Staging)
Launch all components (scraper, auto-backtest scheduler, paper sandbox, testnet execution) concurrently as an enterprise-grade daemon:

```bash
# Runs ingestion, backtesting, simulation, and testnet execution concurrently 24/7
./trading-bot run
```

---

## 4. Operational Monitoring Commands

Query structural logs and metric performance from your terminal:

```bash
# Check the last 20 trade executions and exit triggers
./trading-bot trades

# Retrieve the 10 most recent session records and risk-adjusted metrics
./trading-bot sessions

# Display overall portfolio lifetime performance and PnL totals
./trading-bot stats
```
