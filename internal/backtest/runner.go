package backtest

import (
	"bufio"
	"encoding/json"
	"log"
	"os"
	"trading-bot/internal/config"
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
	cfg             *config.Config
}

// NewRunner creates a new backtest runner.
func NewRunner(strategy strategy.Strategy, positionManager *risk.PositionManager, cfg *config.Config) *Runner {
	return &Runner{
		strategy:        strategy,
		positionManager: positionManager,
		cfg:             cfg,
	}
}

// HistoricalData represents a single data point from the historical data file.
type HistoricalData struct {
	Time  string          `json:"time"`
	Price decimal.Decimal `json:"price"`
}

// Run executes the backtest.
func (r *Runner) Run(filePath string) error {
	log.Printf("VERSION 2.0 - BOOTING")
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	var candles []domain.Candle

	scanner := bufio.NewScanner(file)
	var totalTicks, totalTrades, wins, losses, lastTradeTick int
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
			continue
		}

		signal, rsi := r.strategy.Calculate(candles)

		if r.currentPosition == nil {
			if totalTicks > lastTradeTick && (signal == domain.SignalBuy || signal == domain.SignalSell) {
				qty, err := r.positionManager.CalculatePositionSize(data.Price, r.positionManager.Balance())
				if err != nil {
					log.Printf("Failed to calculate position size: %v", err)
					continue
				}
				stopLossPct := r.positionManager.GetStopLossPct()
				takeProfitPct := r.positionManager.GetTakeProfitPct()

				var stopLossPrice, takeProfitPrice decimal.Decimal
				if signal == domain.SignalBuy {
					stopLossPrice = data.Price.Mul(decimal.NewFromFloat(1).Sub(stopLossPct.Div(decimal.NewFromInt(100))))
					takeProfitPrice = data.Price.Mul(decimal.NewFromFloat(1).Add(takeProfitPct.Div(decimal.NewFromInt(100))))
				} else {
					stopLossPrice = data.Price.Mul(decimal.NewFromFloat(1).Add(stopLossPct.Div(decimal.NewFromInt(100))))
					takeProfitPrice = data.Price.Mul(decimal.NewFromFloat(1).Sub(takeProfitPct.Div(decimal.NewFromInt(100))))
				}

				r.currentPosition = &domain.Position{
					Symbol:          r.cfg.Symbol,
					Side:            string(signal),
					Quantity:        qty,
					Price:           data.Price,
					StopLossPrice:   stopLossPrice,
					TakeProfitPrice: takeProfitPrice,
				}
				r.positionManager.SetCurrentPosition(r.currentPosition)
				log.Printf("Opened %s position at %s", signal, data.Price)
			}
		} else {

			exit, reason := r.positionManager.Evaluate(data.Price, rsi)
			if exit {
				profit := data.Price.Sub(r.currentPosition.Price).Mul(r.currentPosition.Quantity)
				if r.currentPosition.Side == string(domain.SignalSell) {
					profit = r.currentPosition.Price.Sub(data.Price).Mul(r.currentPosition.Quantity)
				}
				log.Printf("[EXIT] Closed at price %s (Reason: %s)", data.Price, reason)
				r.positionManager.UpdateBalance(profit)
				r.currentPosition = nil
				totalTrades++
				if profit.IsPositive() {
					wins++
					lastTradeTick = totalTicks + r.cfg.WinCooldown
				} else if profit.IsNegative() {
					losses++
					lastTradeTick = totalTicks + r.cfg.LossCooldown
				} else {
					lastTradeTick = totalTicks
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	log.Printf("--- Backtest Results ---")
	log.Printf("Total Ticks Processed: %d", totalTicks)
	log.Printf("Total Trades Executed: %d", totalTrades)
	log.Printf("Wins: %d", wins)
	log.Printf("Losses: %d", losses)
	log.Printf("Final Balance: %v", r.positionManager.Balance())
	if losses > 0 {
		log.Printf("Profit Factor: %.2f", float64(wins)/float64(losses))
	} else {
		log.Printf("Profit Factor: N/A (0 Losses)")
	}
	if r.currentPosition != nil {
		log.Printf("Position still OPEN: %s at %v (Current Price: %v)",
			r.currentPosition.Side, r.currentPosition.Price, lastPrice)
	}

	return nil
}
