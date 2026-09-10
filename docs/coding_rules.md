# Corporate Coding Standards: Go Development Guidelines

These rules establish strict requirements for software engineering quality, type-safety, concurrent correctness, and precision arithmetic within the Enterprise Futures Trading System.

---

## 1. Type Safety & Warnings Compliance

1.  **Zero-Tolerance Policy for Suppressions:** Do not use temporary comment hacks or compile directives to suppress or bypass linter warnings or type checks.
2.  **Explicit Structural Management:** Avoid reflection or generic interfaces (`interface{}`) when representing structured domain data. Always instantiate explicit struct models. Type assertions on interface returns must employ two-value check patterns:
    ```go
    // Compliant Type Assertion Pattern
    if concrete, ok := genericVar.(domain.Trade); ok {
        // Use concrete safely
    } else {
        // Log descriptive error and escalate/bubble up
    }
    ```
3.  **Strict Compile-time Assertions:** Verify interface implementation at compile time using standard Go patterns:
    ```go
    var _ trading.Trader = (*BinanceTrader)(nil)
    ```

---

## 2. Robust Error Handling & Wrapping

1.  **Descriptive Error Isolation:** Functions returning errors must wrap root issues to retain complete trace contexts as they propagate up the execution chain. Avoid raw string errors:
    ```go
    // Compliant Wrapping Pattern
    if err != nil {
         return fmt.Errorf("failed to fetch order book state for %s: %w", symbol, err)
    }
    ```
2.  **Zero-Panic Execution:** Application code must not execute `panic()` in production paths. When catching critical errors or exceptions, catch the error, write logs to GORM audit stores, send urgent notifications, and trigger a graceful shutdown process.
3.  **Lumberjack Structured Logging:** Use our structured logger, providing key-value pairs (e.g., symbol, orderID, profit value) to ensure clarity. Do not print unformatted text to stdout.

---

## 3. Explicit Context Lifetime Propagation

1.  **Context Injection:** Every function performing database operations, external API calls, NATS events, or network interactions MUST accept a `context.Context` variable as its first parameter.
    ```go
    func (b *BinanceTrader) PlaceOrder(ctx context.Context, order domain.Order) error
    ```
2.  **Graceful Cancel Propagations:** Goroutine workers must actively monitor context cancellation signals inside selection loops:
    ```go
    select {
    case <-ctx.Done():
        log.Println("Terminating concurrent worker: Context canceled")
        return ctx.Err()
    case tick := <-tickCh:
        // Execute operational tick safely
    }
    ```

---

## 4. Concurrency & Mutex Standards

1.  **Shielding Shared Variables:** Any struct field modified across different goroutines (e.g., system states, paper balance caches, indicator buffers) must be protected using a dedicated `sync.Mutex` or `sync.RWMutex`.
2.  **Avoid Deadlocks:** Lock-holding operations should be minimized. Always use `defer` to release mutexes immediately after acquiring them:
    ```go
    m.mu.Lock()
    defer m.mu.Unlock()
    ```
3.  **Safe Channel Boundaries:** Do not close channels from the receiver side. Ensure sender channels employ robust recovery wrappers to intercept panic states in high-concurrency environments.

---

## 5. Absolute Financial Precision

1.  **Double-Precision Prohibitions:** Floating-point numbers (`float32`, `float64`) are strictly banned for executing asset allocations, pricing calculations, margin computations, or profit-and-loss calculations.
2.  **Shopspring Decimal Enforcement:** Use `decimal.Decimal` objects for all financial logic. Convert to string representations or explicit integers when preparing payloads for API calls:
    ```go
    // Compliant Decimal Pricing Pattern
    entryPrice := decimal.NewFromFloat(54220.50)
    quantity := decimal.NewFromFloat(0.015)
    totalCost := entryPrice.Mul(quantity) // Explicitly safe multiplication
    ```
