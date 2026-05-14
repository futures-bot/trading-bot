# Futures Bot

## Main Characteristics

The futures bot is a trading bot designed to trade cryptocurrency futures on Binance. It uses a strategy based on EMA crossovers to generate trading signals.

- **Strategy-driven:** The bot uses a predefined trading strategy to make decisions. The core strategy is based on the crossover of two Exponential Moving Averages (EMAs).
- **Configuration:** The bot is configured through a `.env` file, where parameters like API keys, trading pair, and strategy settings can be defined.
- **Market Data:** It connects to the Binance API to get real-time market data (klines/candlesticks).
- **Trading:** It can execute trades (buy/sell) on the Binance futures market.
- **Backtesting:** The bot includes a backtesting module to test the trading strategy on historical data.
- **Data Logging:** The bot logs trades and other data to JSON files for analysis.

## Usage

1. **Configuration:** Create a `.env` file with the necessary API keys and configuration parameters.
2. **Run the bot:** `go run main.go`
3. **Backtesting:** `go run internal/backtest/runner.go` (This is a guess based on the file name and may not be accurate)

# Bot Analytics

## Main Characteristics

The bot analytics project seems to be a service that collects and analyzes data from the futures bot.

- **API:** It exposes an API to provide access to the analytics data.
- **Database:** It uses a database to store the data.
- **Event-driven:** It subscribes to events, likely from the futures bot, to update its data.
- **Metrics:** It collects and exposes metrics about the trading activity.
- **Analytics:** It provides analytics on the trading data, such as PnL, win rate, etc.

## Usage

1. **Run the service:** `go run main.go`
2. **Access the API:** The API can be accessed at `http://localhost:<port>` (the port needs to be configured).
