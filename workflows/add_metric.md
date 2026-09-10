# Engineering Workflow: Adding a New Analytical Metric

This runbook guides engineers through adding new analytical or risk-adjusted metrics to the platform. Because our system is completely CLI-driven, we focus on GORM database persistence and console-based presentation layers, rather than API endpoints or WebSocket updates.

---

## 1. Goal
Implement, persist, calculate, and display a new performance metric (e.g., Sortino Ratio, Beta, or Calmar Ratio) across backtesting, paper-trading, and testnet sessions.

---

## 2. Requirements

1.  **GORM Model Extension:** Add fields representing the metric to GORM models inside `internal/database/models.go` (e.g., in the `Session` schema).
2.  **Calculus Engine Integration:** Implement the mathematical logic within our analytics routines, ensuring it remains a pure, deterministic calculation.
3.  **Audit Emitter Update:** Include the metric in the corresponding NATS telemetry payload inside `shared/eventdef`.
4.  **CLI Console Display:** Update terminal formatting layers inside `internal/cmd/cmd.go` to print the metric within the `sessions` or `stats` tables.
5.  **Unit & Regression Tests:** Write isolated unit tests with mock data sets, confirming mathematical correctness and checking for division-by-zero errors.

---

## 3. Engineering Constraints

*   **Financial Precision:** Calculations must use `decimal.Decimal`. Converting values to standard float types is strictly prohibited.
*   **Separation of Concerns:** The calculation logic must remain pure and free of side effects. It should not directly execute database writes or order operations.
*   **No Web-Server Hooks:** Do not introduce HTTP API handlers, HTTP routers, SSE controllers, or WebSocket socket events.

---

## 4. Phase-by-Phase Deliverables

### Phase 1: Database Migration Schema
Update `internal/database/models.go` with GORM structures. Since our schema automatically migrates on startup, the GORM engine handles column adjustments seamlessly.

```go
type Session struct {
    gorm.Model
    Mode         string          `gorm:"index;size:16"`
    Symbol       string          `gorm:"size:16"`
    // ... Legacy Metrics ...
    SortinoRatio decimal.Decimal `gorm:"type:decimal(10,4)"` // New Metric Field
}
```

### Phase 2: Core Calculation Method
Implement the calculation inside your strategy or trading logic. Always handle edge cases like empty historical records or zero loss values to prevent division-by-zero panics.

### Phase 3: NATS Event Schema Adjustment
Incorporate the new metric field into the shared NATS JSON payload contract in `shared/eventdef/eventdef.go` to ensure down-stream auditing services receive updated data structure metrics.

### Phase 4: CLI Pretty-Print Addition
Update terminal-rendering tables inside `internal/cmd/cmd.go` to display your newly computed metrics when operators query performance logs:

```go
fmt.Printf("%-4d | %-8s | %-8s | %-6.4f | %-6.4f\n",
    session.ID,
    session.Mode,
    session.Symbol,
    session.NetPnL,
    session.SortinoRatio, // Newly Integrated Console column
)
```

### Phase 5: Verification and Unit Tests
Write robust unit tests verifying the calculation logic over mock series. Execute the testing workflow from the project root:

```bash
# Execute package testing
go test -v ./internal/trading/...
```
