# MASTER WORKFLOW

## Usage

- `go run cmd/bot/main.go live`: Runs the bot in live trading mode.
- `go run cmd/bot/main.go backtest --file <path>`: Runs the bot in backtesting mode, using the specified data file.
- `go run cmd/bot/main.go scrape`: Runs the bot in scrape mode, logging market data without trading.

## PHASE 1: INITIAL SETUP & SCAFFOLDING

1.  **Initialize Go module:** `go mod init trading-bot`.
2.  **Create project structure:**
    -   `/cmd/bot`: Main entry point.
    -   `/internal/config`: Configuration loading and management.
    -   `/internal/engine`: Core trading logic and state machine.
    -   `/internal/exchange`: API/WebSocket communication with the exchange.
    -   `/internal/logging`: PnL, history, and market pulse logging.
    -   `/internal/risk`: Position, margin, and risk management.
    -   `/internal/strategy`: Technical analysis and signal generation.
3.  **Setup Configuration (`config.yaml`):**
    -   Define a schema for: `symbol`, `leverage`, `session_budget`, `ema_fast`, `ema_slow`, `min_ema_gap`, `take_profit_pct`, `stop_loss_pct`, `ConfirmationCount`, `MinProfitForFlipExit`.
    -   Implement a loader using `gopkg.in/yaml.v3` and map to a `Config` struct.
    -   Print the active configuration on startup.
4.  **Setup `.env` support** for API keys.
5.  **Implement a structured logger** (e.g., `uber-go/zap`).

## PHASE 2: CORE ENGINE & DATA FLOW

1.  **Implement Exchange Client:**
    -   Use `adshao/go-binance/v2` for both REST and WebSocket communication.
    -   All base URLs must point to `https://testnet.binancefuture.com`.
    -   Implement `VerifyCredentials()` to check API keys on startup.
2.  **Data Ingestion:**
    -   Stream `AggTrade` data for the selected symbol into a buffered Go channel.
    -   The bot must collect at least `ema_slow` price points before generating signals.
3.  **Risk Management:**
    -   Implement a thread-safe `PositionManager` using `sync.RWMutex`.
    -   Use `shopspring/decimal` for all price, quantity, and balance calculations.
    -   Implement a `CheckRisk` function to validate trade size against the `session_budget`.
4.  **Execution Engine:**
    -   Implement REST API calls for placing `MARKET` orders.
    -   Implement a "Kill Switch" on `os.Interrupt` to cancel all open orders.
    -   Implement a reconciliation loop (e.g., every 30s) to sync local state with the exchange.

## PHASE 3: TRADING LOGIC & WORKFLOW

1.  **Signal Generation:**
    -   Calculate EMAs (`ema_fast`, `ema_slow`) from the price stream.
    -   Generate `BUY` or `SELL` signals based on EMA crossover.
2.  **Entry Logic:**
    -   **Pre-Flight Checks:**
        -   Verify `AvailableBalance` is not zero.
        -   Fetch latest exchange info (`PricePrecision`, `QuantityPrecision`, `StepSize`).
        -   Set leverage via API call.
    -   **Entry Conditions (All must be met):**
        -   EMA crossover signal is present.
        -   The crossover has been confirmed for `ConfirmationCount` consecutive price updates.
        -   `Math.Abs(ema_gap) > MinEmaGap`.
        -   RSI is not in a "no-man's land" (e.g., Buy if > 55, Sell if < 45).
    -   **Order Calculation:**
        -   `Budget = (AvailableBalance * 0.90)` or fixed `session_budget` from config.
        -   `Quantity = (Budget * Leverage) / CurrentPrice`.
        -   Quantity must be formatted correctly using `StepSize` and `QuantityPrecision`.
        -   The final cost (`Quantity * Price / Leverage`) must not exceed the budget.
    -   **Execution:**
        -   Place `MARKET` order.
        -   After placing, wait and call `GetAccountTradeList` to get the actual filled price.
3.  **Position Monitoring & Exit Logic:**
    -   **Priority 1: Hard Exits (Checked First)**
        -   If `PnL >= TakeProfitPct`, close the position.
        -   If `PnL <= StopLossPct`, close the position.
    -   **Priority 2: Strategy Exits (Checked Second)**
        -   If an `INDICATOR_FLIP` occurs (EMAs cross back):
            -   Only exit if `PnL >= MinProfitForFlipExit`.
            -   If PnL is negative, **hold the position** and wait for a hard exit.
    -   **Cooldown:** After any trade closes, wait for a refractory period (e.g., 5 minutes) before accepting new signals.

## PHASE 4: LOGGING & PERSISTENCE

1.  **Session Tracking:**
    -   Use `meta.json` to track `session_number`.
    -   On startup, store `StartingBalance`.
2.  **Trade Journaling (`trades.jsonl`):**
    -   Append a JSON line for every **closed** trade.
    -   The log must include: `time`, `side`, `entry`, `exit`, `pnl_usdt`, `balance`, `exit_reason`, and context (`ema_gap`, `rsi`, `volatility`).
    -   On startup, read this file to calculate session win/loss history.
3.  **Console Dashboard:**
    -   Every 10 seconds, print a summary:
        -   `[SESSION N] | Time Elapsed: HH:MM:SS`
        -   `Current Balance: X.XX USDT | Session PnL: Y.YY%`
        -   `Status: [Collecting/Analyzing/In Position]`
4.  **Session Summary (`summary_N.txt`):**
    -   At the end of a session, create a summary file with total longs/shorts and average hold time.

## PHASE 5: ERROR HANDLING & RECOVERY

1.  **API Errors:**
    -   On 4XX/5XX errors, enter a "Safety Pause" for 60 seconds.
    -   If an `Insufficient Margin` error occurs, reduce quantity/leverage and retry once, or end the session.
2.  **Connection Drops:**
    -   If the WebSocket connection is lost, attempt to reconnect up to 5 times before shutting down.
3.  **Data Parsing:**
    -   Use regex and string trimming (`regexp.MustCompile("[0-9]+(\\.(\\d+)?)")`) to robustly parse `StepSize` from exchange info.
    -   If parsing fails, use a safe, hard-coded fallback (e.g., "0.1") and log a critical warning.
4.  **Hard Shutdown:**
    -   If `AvailableBalance` drops below a minimum threshold (e.g., 2.0 USDT) or a critical error occurs, cancel all orders and exit gracefully, ensuring all logs are written.
### PHASE 6: ARCHITECTURAL REFACTOR (HEXAGONAL LITE)

1. Domain Extraction:

Create internal/domain/types.go to house universal structs: Candle, Signal, Position, and Order.

Ensure all fields use decimal.Decimal.

2. Strategy Isolation:

Refactor internal/strategy/ema.go to implement a Strategy interface.

Ensure it consumes []domain.Candle and returns a domain.Signal.

3. Risk Manager Promotion:

Move all "Hard Exit" (TP/SL) logic from the engine to internal/risk/position_manager.go.

The engine calls positionManager.Evaluate(currentPrice) to decide on an exit.

4. Engine Slimming:

Remove all math and indicator logic from internal/engine/engine.go.

Re-implement the main loop as a simple sequence: Fetch Data → Update Strategy → Check Risk → Execute (if needed).

5. Test Implementation:

Create internal/strategy/ema_test.go.

Use a static slice of candles to verify that a known EMA cross generates the correct signal.

## PHASE 7: BACKTESTING

1.  **Data Cleanup:**
    -   Move all `.jsonl` files to a `/data` directory.
    -   Update logging paths to reflect the new structure.
2.  **Backtest Runner:**
    -   Create a new package `internal/backtest`.
    -   Implement a `Runner` that reads a historical log file (`market_pulse.jsonl`).
    -   The runner should accept the `strategy.Strategy` and `risk.PositionManager` interfaces.
    -   For each historical price point, it should:
        -   Call `strategy.Calculate()`.
        -   If a signal is generated, simulate a trade using the `PositionManager`.
        -   Track "Paper PnL" without executing real orders.
3.  **Backtest Command:**
    -   Create a new command `cmd/backtest/main.go`.
    -   This command should initialize the necessary components (strategy, risk manager) and run the backtester with a specified data file.

## PHASE 8
We are transitioning the bot into a multi-user SaaS-ready architecture. Apply these rules to all future coding:

Data Isolation: All user-specific data (API keys, Configs) must be abstracted into a Store interface. We will transition from config.yaml to a Database (Postgres) soon.

Event-Driven Logging: Implement an EventBus or Internal Notification System so that UI/Telegram can 'subscribe' to trade events without blocking the trading engine.

Safety Interlocks: Any 'Real Money' execution must require an explicit --mode=PROD flag AND an environment variable CONFIRM_EXCHANGE_LIVE=true.

Resource Management: Implement Lumberjack for log rotation immediately to protect the VM disk space.