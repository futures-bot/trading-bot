# MASTER RULES

## AGENT CODING RULES & CONSTRAINTS

1.  **NO FLOATS:** You are strictly forbidden from using `float64` for price, quantity, or balance. Use `github.com/shopspring/decimal`.
2.  **CONCURRENCY SAFETY:** Every shared resource (Position, Balance, Config) must be protected by a `sync.Mutex` or `sync.RWMutex`. Check for data races.
3.  **CONTEXT TIMEOUTS:** Every network request (REST or WebSocket) must include a `context.Context` with a timeout of 5 seconds.
4.  **ERROR HANDLING:** If an exchange API returns a 4XX or 5XX error, the bot must log the error and enter a "Safety Pause" state for 60 seconds.
5.  **TESTNET FIRST:** All API base URLs must default to the Binance Futures Testnet. Explicitly log a warning if Production URLs are ever used.
6.  **ATOMICITY:** Every 'Open Position' order must include a 'Stop Loss' order in the same execution logic.
7.  **RACE FLAG:** Remind the user to run the bot with `go run -race cmd/bot/main.go` to verify concurrency.
8.  **The 100-Line Rule:** No single function or loop should exceed 100 lines; logic must be extracted into specific "Handlers".
9.  **Strategy Decoupling:** Indicators (Calculate) must only return data, while a StrategyManager evaluates that data to return a Signal.
10. **Concurrency Patterns:** Use channels for data flow (Price → Strategy → Executor) and mutexes for shared state like AvailableBalance.
11. **Clean Initialization:** Avoid global state and init() functions; pass a Config object to all constructors.
12. **Error Context:** Every error must be wrapped (e.g., fmt.Errorf) to provide immediate context on which component failed.
13. DECOUPLING REQUIREMENT: No exchange-specific code (e.g., Binance-specific structs) is allowed inside internal/strategy or internal/risk. These packages must only depend on internal/domain.
14. STRATEGY PURITY: The Strategy must be a pure function or a stateless object that accepts a Candle slice and returns a Signal. It must not trigger orders directly.
15. SINGLE RESPONSIBILITY: The Engine is strictly a "Coordinator." It is forbidden from performing PnL math, indicator calculation, or direct API formatting.
16. INTERFACE-DRIVEN: The Engine must interact with the Exchange via an Exchange interface, allowing for a MockExchange or CSVExchange in tests.
## XRP STRATEGY & LOGGING RULES

### 1. Analysis Logic

-   **Indicator:** Use a 9-period and 21-period EMA (Exponential Moving Average) crossover.
-   **Signal:**
    -   LONG if EMA(9) crosses ABOVE EMA(21).
    -   SHORT if EMA(9) crosses BELOW EMA(21).
-   **Timeframe:** Pull 1-minute klines (candlesticks) for XRPUSDT to generate signals.
-   **Trend Confirmation:** Only enter a trade if the EMA gap (EMA9 - EMA21) is increasing for 3 consecutive price points. This prevents "choppy" entries.
-   **The Anti-Chop Filter:** Only enter if the EMA9 has been above/below the EMA21 for at least 3 consecutive candles.
-   **Confirmation Rule:** The EMA crossover must hold for **5 consecutive price updates** (instead of 3) before an order is placed.

### 2. Operation Execution

-   **Position Size:** Use exactly 10% of available Testnet USDT balance per trade.
-   **Leverage:** Force `SetLeverage(3)` on startup for XRPUSDT. Later rules override this.
-   **Leverage Lock:** Keep leverage at **10x**. This means your total position size will always be approximately **100 USDT worth of XRP**.
-   **Atomic Exit:** Every order must include a hard Stop Loss at 2% and a Take Profit at 4%.
-   **ATR-Based Stops:** Use the Average True Range (ATR) to set the Stop Loss. If the market is very jumpy, the Stop Loss should be wider; if it's calm, it should be tighter.
-   **The 10 USDT Ceiling:** The cost of opening any position (Quantity * Price / Leverage) must never exceed **10.0 USDT**.
-   **Buffer Rule:** Never use more than 90% of the `AvailableBalance` as margin for a single trade. This reserves 10% for fees.
-   **Dynamic Scaling:** If the calculated `Quantity` results in an "Insufficient Margin" error, the bot must automatically reduce the `Leverage` or `Quantity` by 20% and retry once before giving up on that signal.
-   **Account Snapshot:** Before every `NewOrder` call, fetch the latest balance. Do not rely on a cached balance variable, as it may be stale.
-   **Precision Mastery:** Never send an order without first passing the price/quantity through the rounding utility based on Exchange Info.
-   **Precision Rounding:** When calculating the trade quantity for XRP, always use `math.Floor(quantity * 10) / 10` to ensure exactly 1 decimal place.
-   **Pre-Flight Balance Check:** The bot must never start "Collecting price points" if the initial balance check returns 0. It must stop immediately.

### 3. The "Earnings Log" (PnL Tracking)

-   **Local Database:** Create a `trades.jsonl` file.
-   **Log Format:** For every closed trade, append a JSON object:
    `{"time": "...", "side": "...", "entry": 0.00, "exit": 0.00, "pnl_usdt": 0.00, "balance": 0.00}`
-   **Success Metrics:** At the end of every hour, log a summary: "Total Trades: X | Win Rate: Y% | Total Profit: Z USDT".
-   **Realized PnL Formula:**
    -   Use: `(ExitPrice - EntryPrice) * Quantity * Leverage` (for Longs).
    -   All calculations must use `shopspring/decimal`.
-   **Persistent Journaling:**
    -   Append every closed trade to `trades.jsonl`.
    -   On startup, the bot should read `trades.jsonl` to "remember" the session's previous win/loss record so the success rate is accurate even if the bot restarts.
-   **Analysis Validation:**
    -   Before proceeding with an operation, the bot must log the specific reason:
        -   *Ex: "Analysis: EMA9 (1.4166) crossed above EMA21 (1.4138). Executing Long..."*
-   **Detailed Trade Metadata:**
    -   For every trade, log: `EMA9`, `EMA21`, `RSI` (if available), and `Volatility` at the time of entry.
    -   This data is for the "Analyzer" to determine which market conditions lead to wins.
-   **The Success Dashboard:**
    -   Update the console summary to show: "Current Wallet Balance" vs "Starting Wallet Balance".
-   **Dashboard Requirement:** The 10-second console summary must show:
    -   [SESSION N] | Time Elapsed: HH:MM:SS
    -   Current Balance: X.XX USDT | Session PnL: Y.YY%
    -   Status: [Collecting/Analyzing/In Position]
-   **Reasoned Logging:** Every trade entry must be logged with the exact indicator values that triggered it (e.g., "EMA9: 1.4304, EMA21: 1.4302").

## RISK & TRADING RULES

-   **Cooldown Period:** After a trade closes (win or loss), the bot must enter a "Refractory Period" of 300 seconds (5 minutes) where it ignores all signals.
-   **Spread Protection:** If the difference between `entry` and `exit` signal is less than the exchange fee (0.04%), the bot must stay in the trade.
-   **Session Persistence:** Keep logging the `ema_gap` and `rsi`. This data is proving that the current "flat" market is the bot's biggest enemy.
-   **One-Shot Failure Rule:** If an order fails for any API reason, do not retry immediately. Log the error, wait 60 seconds, and if it's a Margin error, trigger the Hard Shutdown.
-   **No Virtual Mode:** If the budget is gone, the session is over. Do not simulate trades; stop the process so the user can review and reset.
-   **Session Post-Mortem:** If a session ends with a balance of 0, the `history.jsonl` entry must be flagged with `"status": "LIQUIDATED_OR_EMPTY"`.
-   **Market Order Handling:** Since we use MARKET orders, the bot must wait for the "Filled" status before recording the `entry` price in `trades.jsonl`. If the price is 0, use the last known ticker price as a fallback.
-   **Anti-Spam:** After a successful order placement, the bot must wait for the position to **CLOSE** before it is allowed to open a new one. No "layering" positions.
-   **Fee Awareness:** The bot must calculate the estimated fee before entering. If the projected profit doesn't cover 2x the fee, skip the trade.
-   **Session Summary:** At the end of a session, create a `summary_N.txt` that counts:
    -   Total Longs vs Total Shorts.
    -   Average hold time per trade.

