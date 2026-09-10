# Enterprise Futures Trading System (EFTS) v4.0 (Pure-CLI)

An institutional-grade, high-performance, concurrent, and modular algorithmic futures trading platform developed in Go. Built explicitly to execute and validate systematic strategies on the Binance Futures Testnet.

This platform is engineered to meet corporate production standards, prioritizing mathematical rigor, strict risk boundaries, thread safety, and transparent audit trails. The system is designed to be **100% CLI-driven** and daemon-operated, with zero dependencies on web servers, WebSockets for frontend clients, or graphic user interfaces.

---

## 1. System Overview

EFTS runs as a high-performance background daemon or on-demand task processor. It implements an event-driven systematic trading model:

```
                  +-----------------------------------+
                  |      Binance WebSocket API        |
                  +-----------------+-----------------+
                                    | Real-time Price Ticks
                                    v
+------------------+      +-------------------+      +------------------+
|                  |      |                   |      |                  |
|   CLI commands   | ---> |  Execution Engine | ---> |  PostgreSQL DB   |
| (run, testnet,   |      |   (Paper/Testnet) |      | (Audit Trail &   |
|   paper, etc.)   |      +---------+---------+      |   Trade Logs)    |
|                  |                |                +------------------+
+------------------+                |                        ^
                                    | Publishes Events       | Persists States
                                    v                        |
                          +-------------------+              |
                          |  NATS Event Bus   | -------------+
                          +-------------------+
```

### Core Architecture Principles
* **Pure Command Line Interface (CLI):** Designed for shell execution, cron orchestrations, and `systemd` process supervision.
* **Asynchronous & Thread-Safe:** Fully non-blocking event-driven structure leveraging Go's native goroutines, channels, and synchronization primitives.
* **Deterministic Risk Management:** Built-in programmatic guardrails, state-enforced cooldowns, and a global circuit-breaker ("Panic Protocol").
* **Unified Audit Trails:** All analytical metrics, system logs, market pulse ticks, and execution history are written directly to PostgreSQL via GORM.

---

## 2. CLI Command Specification

The application binary exposes a highly cohesive, standard Unix-style command interface.

| Command | Category | Execution Mode | Description |
|:---|:---|:---|:---|
| `run` | Orchestration | Daemon | **Staging Daemon:** Concurrently launches background scraper, auto-backtesting loop, simulated paper-trading, and testnet execution tasks. |
| `backtest [file]` | Evaluation | On-Demand | **Historical Simulator:** Runs the systematic strategy on historical candle data in-memory or from local JSONL datasets. Outputs institutional analytics. |
| `paper` | Simulation | Daemon | **Real-Time Sandbox:** Evaluates strategies on real-time market feeds from Binance WebSocket using simulated accounts and balances. |
| `testnet` | Execution | Daemon | **Dry Run Execution:** Connects directly to Binance Futures Testnet, executing actual exchange market orders with paper capital. |
| `scrape [symbols]` | Data Utility | On-Demand | **Data Pipeline:** Concurrently scrapes historical klines from the public Binance API, updates the DB, and triggers backtesting. |
| `trades` | Auditing | Query | **Audit Log:** Queries the database and pretty-prints the 20 most recent trade execution records. |
| `sessions` | Auditing | Query | **Session Inspector:** Displays the 10 most recent execution sessions along with calculated risk/return metrics. |
| `stats` | Portfolio | Query | **Portfolio Reporter:** Evaluates and aggregates system-wide lifetime performance metrics (overall PnL, Win/Loss ratios). |
| `version` | Metadata | Immediate | **Build Info:** Prints current compilation version and engine details. |
| `help` | Metadata | Immediate | **CLI Help:** Outputs complete command usage reference. |

---

## 3. Deep-Dive: Core Functionalities

### A. Core Strategy Engine: EMA Crossover with RSI Gate
EFTS executes a systematic, trend-following strategy designed for high-volatility futures contracts:
1. **Indicator Calculus:** Computes Fast EMA (period: 9) and Slow EMA (period: 21) dynamically along with RSI (period: 14) over moving tick/kline frames.
2. **Long Entry Signal:** Fast EMA crosses above Slow EMA by at least `min_ema_gap` AND RSI crosses above the oversold boundary (default: 40), verified across a configurable consecutive confirmation threshold (`confirmation_count`).
3. **Short Entry Signal:** Fast EMA crosses below Slow EMA by at least `min_ema_gap` AND RSI crosses below the overbought boundary (default: 60), verified across a consecutive confirmation threshold.

### B. Institutional Risk Engine
To prevent catastrophic drawdowns and secure capital, the engine enforces strict runtime safeguards:
* **Stop Loss (SL):** Programmatically liquidates positions if price moves `0.2%` against entry.
* **Take Profit (TP):** Liquidates positions if profit targets reach `0.5%`.
* **Break-Even (BE) Trigger:** Automatically slides the stop-loss level to the exact entry price once positive price variance matches `0.1%`.
* **Trailing Stop-Loss:** Activates post-BE, locking in gains by tracking peak positive variance at a strict trailing distance of `0.1%`.
* **RSI Exhaustion Gate:** Immediate position termination if RSI exceeds `70` (for Longs) or falls below `30` (for Shorts).
* **Execution Cooldowns:** Mandates a strict `60-tick` operational pause immediately after any trade exit, mitigating "revenge trading" or high-frequency oscillation loops.
* **Circuit-Breaker ("Panic Protocol"):** Triggers an immediate hard-shutdown of the execution daemon if a realized loss exceeds a hard cap of `1.50 USDT` within a session.
* **Inviolable Position Bounds:** Enforces exactly *one active position at a time* per-symbol via thread-safe lock-states.

### C. Analytical Risk Metrics
Every completed session (simulation, paper-trading, or testnet) computes a standard risk-return dossier:
* **Win Rate:** $\frac{\text{Winning Trades}}{\text{Total Trades}} \times 100$.
* **Profit Factor (PF):** $\frac{\text{Gross Profit}}{\text{Gross Loss}}$. Critical baseline where $\text{PF} > 1.0$ indicates strategy viability.
* **Expectancy:** Average profit or loss expected per single trade action.
* **Max Drawdown (MDD):** Peak-to-trough value reduction representing worst-case historical risk exposure.
* **Sharpe Ratio:** Volatility-adjusted return metric evaluating strategy efficiency.

---

## 4. Operational & Database Schema

The system uses GORM for PostgreSQL relational persistence. All transactional, system state, and market records are persisted instantly to ensure compliance and prevent telemetry loss.

```sql
-- Core schemas automatically migrated on startup:
CREATE TABLE trades (
    id SERIAL PRIMARY KEY,
    symbol VARCHAR(16) NOT NULL,
    side VARCHAR(8) NOT NULL,
    entry_price DECIMAL(18, 8) NOT NULL,
    exit_price DECIMAL(18, 8) NOT NULL,
    profit DECIMAL(18, 8) NOT NULL,
    exit_reason VARCHAR(32) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE sessions (
    id SERIAL PRIMARY KEY,
    mode VARCHAR(16) NOT NULL,
    symbol VARCHAR(16) NOT NULL,
    total_trades INT NOT NULL,
    wins INT NOT NULL,
    losses INT NOT NULL,
    net_pnl DECIMAL(18, 8) NOT NULL,
    profit_factor DECIMAL(10, 4) NOT NULL,
    max_drawdown DECIMAL(10, 4) NOT NULL,
    sharpe_ratio DECIMAL(10, 4) NOT NULL,
    expectancy DECIMAL(18, 8) NOT NULL,
    status VARCHAR(16) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
```

---

## 5. Security & Configuration Compliance

Institutional-grade security protocols are strictly observed:
1. **Zero hardcoded credentials:** Absolutely no private API keys, database DSNs, or chat tokens exist in `config.yaml`.
2. **DSN Enforcements:** The PostgreSQL connection string uses strict key-value pairs (`host=... sslmode=require`) in `.env` to prevent URI parser vulnerabilities with complex passwords.
3. **Financial Precision:** Floating-point arithmetic is strictly forbidden for currency operations. The codebase utilizes `shopspring/decimal` for all margin, capital, and indicators logic.

For deployment configuration, see `QUICK_START.md` and `GCP_DEPLOYMENT.md`.
