# Documentation

## Overview

This is a Binance Futures trading bot written in Go. It provides a framework for developing and backtesting trading strategies. The bot is designed to be event-driven and modular, allowing for easy extension and customization.

## Project Structure

The project is organized into the following directories:

- `cmd/`: Contains the main application entrypoint.
- `data/`: Contains historical market data and trade logs.
- `docs/`: Contains architecture and coding standards documentation.
- `internal/`: Contains the core application logic.
  - `backtest/`: The backtesting engine.
  - `config/`: Application configuration.
  - `database/`: Database repository and models.
  - `events/`: NATS event publisher.
  - `marketdata/`: Binance market data client.
  - `notifications/`: Telegram notifications.
  - `scraper/`: Historical data scraper.
  - `strategy/`: Trading strategies.
  - `trading/`: Core trading logic.
- `shared/`: Shared code between services.
  - `eventdef/`: Standardized event definitions.
- `workflows/`: Task-oriented workflows for common development tasks.

## Workflows

The `workflows` directory contains a set of predefined workflows for common development tasks. These workflows are designed to be executed by an AI assistant like Cline to ensure consistency and adherence to the project architecture.

- `add_metric.md`: A workflow for adding a new analytics metric to the `bot-analytics` service.
- `event_schema_workflow.md`: A workflow for standardizing NATS events.
- `persistence_workflow.md`: A workflow for implementing the persistence layer.

## Configuration

The bot is configured through a `config.yaml` file and a `.env` file.

- `config.yaml`: Contains trading parameters such as the symbol to trade, leverage, and strategy parameters.
- `.env`: Contains environment variables such as API keys, NATS URL, and Telegram bot token.

## Usage

The bot can be run in several modes:

- `run`: Runs the scraper, paper trader, testnet trader, and backtester concurrently.
- `backtest`: Runs a backtest on historical data.
- `paper`: Starts paper trading.
- `testnet`: Starts testnet trading.
- `scrape`: Scrapes historical kline data from Binance.
