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
	for scanner.Scan() {
		var data HistoricalData
		if err := json.Unmarshal(scanner.Bytes(), &data); err != nil {
			log.Printf("Failed to unmarshal historical data: %v", err)
			continue
		}

		candle := domain.Candle{
			Open:  data.Price,
			High:  data.Price,
			Low:   data.Price,
			Close: data.Price,
		}
		candles = append(candles, candle)

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
			exit, reason := r.positionManager.Evaluate(data.Price)
			if exit {
				profit := data.Price.Sub(r.currentPosition.Price).Mul(r.currentPosition.Quantity)
				if r.currentPosition.Side == string(domain.SignalSell) {
					profit = r.currentPosition.Price.Sub(data.Price).Mul(r.currentPosition.Quantity)
				}
				log.Printf("Closed position at %s for a profit of %s. Reason: %s", data.Price, profit, reason)
				r.currentPosition = nil
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}
