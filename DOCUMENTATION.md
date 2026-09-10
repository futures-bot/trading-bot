# Corporate Technical Documentation: Enterprise Futures Trading System (EFTS)

This guide provides deep technical details regarding the system's package structures, internal modular architecture, database mapping, NATS event contract, and software maintenance guidelines.

---

## 1. System Package Architecture

The EFTS codebase is organized as a structured, modular monorepo containing decoupled backend packages. Cyclic dependencies are strictly prohibited, and interface-driven design is utilized for easy mocking and unit testing.

```
/trading-bot/
├── main.go                     # Unified Application entry point (CMD Router)
├── config.yaml                 # Static system & strategic parameter definitions
├── .env                        # Cryptographic secrets & environmental variables
├── go.mod                      # Dependency management
├── internal/
│   ├── cmd/                    # CLI commands implementation & execution loops
│   ├── config/                 # YAML & ENV parser/config validator
│   ├── database/               # GORM PostgreSQL repository, models, and transactions
│   ├── events/                 # NATS publisher wrappers (schema-enforcing)
│   ├── logger/                 # Production file & console logger (Lumberjack)
│   ├── marketdata/             # REST/WebSocket bindings for Binance Exchange
│   ├── notifications/          # Resilient Telegram/Null dispatchers
│   ├── scraper/                # Concurrent historical kline harvester
│   ├── strategy/               # Technical indicator calculations & trade signal logic
│   └── trading/                # Position management, trade tracker, & order engines
│       ├── domain/             # Unified entity types (Trade, Position, Signal, etc.)
│       └── ...
└── shared/
    └── eventdef/               # Enterprise NATS telemetry schemas (shared payload definitions)
```

---

## 2. Component Design & Responsibility Matrix

### `internal/cmd` (CLI Executive Interface)
Acts as the central router and lifecycle coordinator. Handles POSIX system signals (`SIGINT`, `SIGTERM`) to gracefully close exchange WebSocket streams, flush buffered database logs, publish final session stats to NATS, and notify developers of normal or emergency shutdowns.

### `internal/trading` (Execution and Position Management)
Maintains thread-safe in-memory state of active positions and orders. It supports two main transaction executors:
1. **PaperTrader:** Simulates execution locally using real-time price feeds. All metrics are computed and stored exactly as if real-money execution took place.
2. **BinanceTrader:** Communicates with the Binance Futures Testnet REST and WebSocket APIs. Performs automated position size formatting, leverage checks, and margin monitoring.

### `internal/database` (Data Integrity & Storage)
Encapsulates PostgreSQL transaction layers. GORM is configured with:
* Direct index creations on critical query paths (e.g., `symbol`, `created_at`).
* Safe connection pool bounds (`MaxOpenConns = 10`, `MaxIdleConns = 5`, `ConnMaxLifetime = 1h`).
* Soft-delete capability and transaction-wrapped mutations for audit logs and account states.

### `internal/scraper` (Concurrent Ingestion Channel)
A high-throughput harvester designed to fetch historical candle data (klines) from public market endpoints. Implements:
* Goroutine worker pools grouping parallel symbol downloads.
* Auto-backtesting loop activation upon completion of symbol historical streams.
* Transient error retries utilizing exponential backoff algorithms.

---

## 3. Communication Model: NATS Event Schema

To decouple reporting and operations, EFTS publishes standardized structural messages onto a centralized NATS bus. All events follow the core envelope design defined in `shared/eventdef`.

### Standard Event Envelope Schema
```json
{
  "event_id": "uuid-v4-identifier",
  "event_type": "trade.closed",
  "version": 1,
  "timestamp": "2026-09-10T14:32:00.123Z",
  "source": "paper-trader",
  "payload": {}
}
```

### Event Topics and Subjects

*   **`trades`** (Subject: `trade.closed` / `trade.opened`):
    Fires when an executor takes action. The payload contains final performance indicators (prices, profits, exact exit reason trigger like `STOP_LOSS`, `TAKE_PROFIT`, or `RSI_EXHAUSTION`).
*   **`sessions`** (Subject: `session.completed`):
    Dispatched when a rotating session (e.g., hourly cycle) ends. Transmits analytical performance summaries (Win/Loss numbers, Sharpe Ratio, Profit Factor, Drawdowns).
*   **`klines.<symbol>`** (Subject: `kline.new`):
    Published by the concurrent scraper when new historical chunks are integrated into the DB.

---

## 4. Operational Safety Controls

The platform implements programmatic protective controls:
1. **Precision Assurance:** All financial quantities use the `decimal.Decimal` arbitrary-precision framework. Conversions to/from floats are strictly prohibited during runtime calculations.
2. **State Cooldowns:** The trade executor enforces a state-lock during the `loss_cooldown` or `win_cooldown` tick limits, preventing rapid order loops.
3. **Budget Bounds:** The system validates that each order matches the specified `session_budget` constraint before dispatching orders to the exchange.
4. **Panic Protections:** Recovery wrappers run on all background threads, catching panics, logging stack traces to database records, publishing a failure event on NATS, and issuing a high-priority alert via Telegram.
