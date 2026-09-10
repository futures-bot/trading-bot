# Product Backlog: Enterprise Futures Trading System

This document outlines high-, medium-, and low-priority milestones targeted for our upcoming engineering cycles, standardizing the trading bot for institutional-grade reliability.

---

## 1. High Priority (Q3-Q4 2026)

### Programmatic Risk & Compliance Engine
*   **Separation of Risk Concerns:** Extract risk management, leverage limits, drawdowns, and operational cool-downs from active trading packages into an isolated `internal/risk` module.
*   **Post-Trade Slippage Checks:** Implement a post-trade slippage validator matching executed prices against target indicator prices.
*   **Drawdown Guardrail:** Establish an institutional-grade maximum daily drawdown tracking index. Auto-suspend execution threads across the entire daemon if the aggregate loss limit is breached.

### Systematic Replay & Backtest Verification Pipeline
*   **Historical Session Replay:** Build a replay pipeline using persisted database market snapshots, allowing developers to trace strategy indicator calculations and trade decisions on historic tick feeds.
*   **Integration Test Harness:** Establish an automated integration test harness that spins up mock NATS and transient PostgreSQL containers using Docker Compose to validate state transitions and event emission schemas.

---

## 2. Medium Priority (2027)

### High-Performance Incremental Analytics
*   **Stateful Incremental Metrics:** Redesign session analytics (Sharpe ratio, max drawdowns, profit factor computations) to use incremental, stateful processing. This reduces memory usage and improves database write performance during 24/7 background tasks.
*   **Indicator Optimization:** Implement pre-calculated indicator vectors using specialized circular ring-buffers to optimize EMA computations.

### Multi-Strategy Pipeline Integration
*   **Abstract Strategy Interface:** Refactor `internal/strategy` to implement a generic interface, allowing developers to configure and run multiple strategies (e.g., Mean Reversion, Grid Trading) concurrently under a single execution daemon.

---

## 3. Low Priority (2027+)

### Unified Multi-Exchange Adapter Interface
*   **Abstract Trader Interfaces:** Decouple exchange-specific API structures from execution layers. This creates abstract adapters, laying the foundation for integrating other platforms (e.g., OKX, Bybit, Coinbase).

### Live Production Broker Integrations
*   **Live Capital Guardrails:** Establish live trading support using multi-party authorization (MPA) checks, secure key vault storage integrations (e.g., HashiCorp Vault), and advanced capital management policies.
