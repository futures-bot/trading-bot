# Architectural Design: Decoupled CLI Algorithmic Engine

This document provides architectural standards for the Enterprise Futures Trading System (EFTS). It defines internal micro-components, cross-package coupling boundaries, and our standardized event-driven data flow.

---

## 1. Modular De-coupling Rules

Corporate stability requires that modules remain strictly separated with unidirected dependencies. We enforce the following separation of concerns:

```
                  +-----------------------------------+
                  |             main.go               |
                  +-----------------+-----------------+
                                    |
                                    v
                  +-----------------------------------+
                  |        internal/cmd (CLI)         |
                  +-----------------+-----------------+
                                    |
                                    v
+------------------+      +-------------------+      +------------------+
|  internal/config | <--- |  internal/trading | ---> | internal/events  |
+------------------+      +---------+---------+      +------------------+
                                    |                         |
                                    v                         v
                          +-------------------+      +------------------+
                          | internal/database |      | shared/eventdef  |
                          +-------------------+      +------------------+
```

1.  **Direct Database Access Isolation:** Only the `internal/database` repository layer interacts directly with the SQL engine or writes to GORM model structures. Core execution routines must access data exclusively through Go interface definitions.
2.  **Order Placement Uniqueness:** The `internal/trading` package acts as the sole authorized sender of market orders. Analytical reporting handlers, notifier layers, or scraper workers are strictly restricted from initiating buy or sell executions.
3.  **Cyclic Reference Prohibition:** Compilation will fail if cyclic imports are introduced. Cross-module data sharing is facilitated through independent domain primitives housed in `internal/trading/domain` and generalized event definitions in `shared/eventdef`.

---

## 2. Event-Driven Communication Pipeline

Rather than tightly binding operational logic to reporting features, EFTS operates on an asynchronous Pub/Sub model mediated by a NATS broker.

1.  **State Telemetry Dispatch:** When a position is liquidated or closed, the executing thread triggers a NATS publish operation containing structural data definitions (e.g., `trade.closed` topic).
2.  **Audit Emitters:** The decoupled event publisher strictly adheres to the predefined schemas under `shared/eventdef`, formatting entries using JSON serialization before streaming them onto the active NATS pipeline.
3.  **Zero-Coupling Subscriptions:** Logging pipelines, corporate analytics services, and automated auditing tasks run as independent background NATS consumers. These systems ingest execution records from NATS in a read-only manner, with zero operational impact on the execution engine.

---

## 3. High-Availability Operational Lifecycles

### Context Life-Cycle Management
Network calls, exchange operations, and database connections must enforce explicit context configurations:
*   Every API, database transaction, and remote connection enforces a standard timeout envelope of **5 seconds**.
*   Standard Go `context.WithCancel` frames are wired through the application's root execution paths. When the parent thread registers terminate signals (`SIGINT`, `SIGTERM`), standard context closures cascade downwards, shutting down active WebSocket price streams, pausing order engines, flushing GORM connections, and releasing resources cleanly.

### Concurrent Thread Synchronization
All operational models rely on concurrent goroutines. Race conditions are programmatically eliminated using Go synchronization controls:
*   **Active State Access:** Shared variables (e.g., simulated account balance in `PaperTrader` or order trackers) are shielded behind explicit `sync.Mutex` structures.
*   **Execution Fences:** A single-position-at-a-time rule is enforced. Multi-threaded signals cannot concurrently spawn order tasks; additional trading opportunities are rejected until the active position mutex is safely released on exit.
