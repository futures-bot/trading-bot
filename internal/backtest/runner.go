package backtest

import (
	"bufio"
	"encoding/json"
	"log"
	"os"
	"trading-bot/internal/domain"
	"trading-bot/internal/risk"
	"trading-bot/internal/strategy"

	"github.com/shopspring/decimal"
)

// Runner executes a backtest from a historical data file.
type Runner struct {
	strategy        strategy.Strategy
	positionManager *risk.PositionManager
	currentPosition *domain.Position
}

// NewRunner creates a new backtest runner.
func NewRunner(strategy strategy.Strategy, positionManager *risk.PositionManager) *Runner {
	return &Runner{
		strategy:        strategy,
		positionManager: positionManager,
	}
}

// HistoricalData represents a single data point from the historical data file.
type HistoricalData struct {
	Time  string          `json:"time"`
	Price decimal.Decimal `json:"price"`
}

// Run executes the backtest.
func (r *Runner) Run(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	var candles []domain.Candle

	scanner := bufio.NewScanner(file)
	var totalTicks int
	var lastPrice decimal.Decimal
	for scanner.Scan() {
		totalTicks++
		var data HistoricalData
		if err := json.Unmarshal(scanner.Bytes(), &data); err != nil {
			log.Printf("Failed to unmarshal historical data: %v", err)
			continue
		}
		lastPrice = data.Price

		candle := domain.Candle{
			Open:  data.Price,
			High:  data.Price,
			Low:   data.Price,
			Close: data.Price,
		}
		const maxWindow = 200 // More than enough for RSI/EMA stability
		candles = append(candles, candle)

		if len(candles) > maxWindow {
			// Keep only the most recent 'maxWindow' elements
			candles = candles[1:]
		}

		const warmupPeriod = 20
		if len(candles) < warmupPeriod {
			log.Printf("Warm-up period: %d/%d price points collected", len(candles), warmupPeriod)
			continue
		}

		signal := r.strategy.Calculate(candles)

		if r.currentPosition == nil {
			if signal == domain.SignalBuy || signal == domain.SignalSell {
				qty, err := r.positionManager.CalculatePositionSize(data.Price, r.positionManager.Balance())
				if err != nil {
					log.Printf("Failed to calculate position size: %v", err)
					continue
				}
				r.currentPosition = &domain.Position{
					Symbol:   "XRPUSDT", //TODO: parameterize
					Side:     string(signal),
					Quantity: qty,
					Price:    data.Price,
				}
				log.Printf("Opened %s position at %s", signal, data.Price)
			}
		} else {
			// Only log every 1000 ticks so we don't spam
			if totalTicks%1000 == 0 {
				log.Printf("Tick %d: Price %v | Position %s at %v | Still waiting for exit...",
					totalTicks, data.Price, r.currentPosition.Side, r.currentPosition.Price)
			}
			exit, reason := r.positionManager.Evaluate(data.Price)
			if exit {
				profit := data.Price.Sub(r.currentPosition.Price).Mul(r.currentPosition.Quantity)
				if r.currentPosition.Side == string(domain.SignalSell) {
					profit = r.currentPosition.Price.Sub(data.Price).Mul(r.currentPosition.Quantity)
				}
				log.Printf("Closed position at %s for a profit of %s. Reason: %s", data.Price, profit, reason)
				r.positionManager.UpdateBalance(profit)
				r.currentPosition = nil
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	log.Printf("--- Backtest Results ---")
	log.Printf("Total Ticks Processed: %d", totalTicks)
	log.Printf("Final Balance: %v", r.positionManager.Balance())
	if r.currentPosition != nil {
		log.Printf("Position still OPEN: %s at %v (Current Price: %v)",
			r.currentPosition.Side, r.currentPosition.Price, lastPrice)
	}

	return nil
}
