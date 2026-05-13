# Master Workflow

This document outlines the workflow for the trading bot project.

## Phase 1: Core Domain & Application Refactoring

1.  **Project Structure:** Restructure the project to reflect the Bounded Contexts: `Trading`, `Accounts`, `MarketData`, and `Notifications`.
2.  **Domain Layer:** Define the core domain models and business logic for each context.
3.  **Application Layer:** Create application services to orchestrate the use cases (e.g., `StartTradingSession`, `PlaceOrder`, `RegisterUser`).
4.  **Infrastructure Layer:** Implement the infrastructure components (e.g., Binance API client, database repositories).

## Phase 2: Multi-Tenancy & API

1.  **User Authentication:** Implement a secure authentication system for users.
2.  **API Key Management:** Create a system for users to store and manage their encrypted Binance API keys.
3.  **Dynamic Configuration API:** Build an HTTP API to allow users to dynamically configure their trading strategies and bot settings.

## Phase 3: Advanced Features & CLI

1.  **Trading Modes:** Implement the `trade`, `scrape`, and `backtest` modes.
2.  **Real-time Logging:** Add WebSocket support to provide real-time logs of prices and positions.
3.  **Admin CLI:** Create a CLI for the admin to manage the application.

## Documentation

1.  **`.clinerules`:** Document the project's rules and workflow in the `.clinerules` directory.
2.  **Docstrings:** Add extensive `godoc` compatible docstrings to all public functions and types.
