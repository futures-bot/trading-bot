package backtest

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"trading-bot/internal/config"
	"trading-bot/internal/events"
	"trading-bot/internal/trading"
	"trading-bot/internal/trading/domain"
	"trading-bot/shared/eventdef"

	"github.com/shopspring/decimal"
)

// Runner is a backtesting engine that reads historical data from a file and simulates trades.
type Runner struct {
	strategy        trading.Strategy
	positionManager *trading.PositionManager
	currentPosition *domain.Position
	cfg             *config.Config
	publisher       events.Publisher
}

func NewRunner(strategy trading.Strategy, positionManager *trading.PositionManager, cfg *config.Config, publisher events.Publisher) *Runner {
	return &Runner{
		strategy:        strategy,
		positionManager: positionManager,
		cfg:             cfg,
		publisher:       publisher,
	}
}

type HistoricalData struct {
	Time  string          `json:"time"`
	Price decimal.Decimal `json:"price"`
}

func (r *Runner) Start(ctx context.Context) {
	// If Start is used generically, we might need a default symbol, assuming first symbol for now.
	var sym string
	if len(r.cfg.Symbols) > 0 {
		sym = r.cfg.Symbols[0]
	}
	go r.Run(r.cfg.BacktestFile, sym)
}

func (r *Runner) Stop() {}

func (r *Runner) GetStatus() domain.Status {
	return domain.Status{}
}

func (r *Runner) Run(filePath string, symbol string) error {
	log.Printf("Backtest starting: file=%s symbol=%s", filePath, symbol)

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open backtest file: %w", err)
	}
	defer file.Close()

	var candles []domain.Candle
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var data HistoricalData
		if err := json.Unmarshal(scanner.Bytes(), &data); err != nil {
			continue
		}
		candles = append(candles, domain.Candle{
			Open: data.Price, High: data.Price, Low: data.Price, Close: data.Price,
		})
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	return r.RunFromCandles(candles, symbol)
}

func (r *Runner) RunFromCandles(candles []domain.Candle, symbol string) error {
	var window []domain.Candle

	totalTicks := len(candles)
	var totalTrades, wins, losses, lastTradeTick int
	var lastPrice decimal.Decimal

	for tick, candle := range candles {
		lastPrice = candle.Close

		const maxWindow = 200
		window = append(window, candle)
		if len(window) > maxWindow {
			window = window[1:]
		}

		const warmupPeriod = 20
		if len(window) < warmupPeriod {
			continue
		}

		signal, rsi := r.strategy.Calculate(window)

		if r.currentPosition == nil {
			if tick > lastTradeTick && (signal == domain.SignalBuy || signal == domain.SignalSell) {
				qty, err := r.positionManager.CalculatePositionSize(candle.Close, r.positionManager.Balance(), r.strategy.TrendStrength())
				if err != nil {
					continue
				}
				stopLossPct := r.positionManager.GetStopLossPct()
				takeProfitPct := r.positionManager.GetTakeProfitPct()

				var stopLossPrice, takeProfitPrice decimal.Decimal
				if signal == domain.SignalBuy {
					stopLossPrice = candle.Close.Mul(decimal.NewFromFloat(1).Sub(stopLossPct.Div(decimal.NewFromInt(100))))
					takeProfitPrice = candle.Close.Mul(decimal.NewFromFloat(1).Add(takeProfitPct.Div(decimal.NewFromInt(100))))
				} else {
					stopLossPrice = candle.Close.Mul(decimal.NewFromFloat(1).Add(stopLossPct.Div(decimal.NewFromInt(100))))
					takeProfitPrice = candle.Close.Mul(decimal.NewFromFloat(1).Sub(takeProfitPct.Div(decimal.NewFromInt(100))))
				}

				r.currentPosition = &domain.Position{
					Symbol:          symbol,
					Side:            string(signal),
					Quantity:        qty,
					Price:           candle.Close,
					StopLossPrice:   stopLossPrice,
					TakeProfitPrice: takeProfitPrice,
				}
				r.positionManager.SetCurrentPosition(r.currentPosition)
				log.Printf("[OPEN] %s @ %s", signal, candle.Close)
			}
		} else {
			exit, reason := r.positionManager.Evaluate(candle.Close, rsi)
			if exit {
				profit := candle.Close.Sub(r.currentPosition.Price).Mul(r.currentPosition.Quantity)
				if r.currentPosition.Side == string(domain.SignalSell) {
					profit = r.currentPosition.Price.Sub(candle.Close).Mul(r.currentPosition.Quantity)
				}
				log.Printf("[EXIT] @ %s | PnL: %s | Reason: %s", candle.Close, profit.StringFixed(4), reason)
				r.positionManager.UpdateBalance(profit)

				dbTrade := &domain.Trade{
					Symbol:     symbol,
					Side:       r.currentPosition.Side,
					EntryPrice: r.currentPosition.Price,
					ExitPrice:  candle.Close,
					Profit:     profit,
					ExitReason: reason,
				}

				r.publisher.Publish(context.Background(), "trades", eventdef.NewEvent("trade.closed", "backtest", 1, dbTrade))

				r.currentPosition = nil
				totalTrades++
				if profit.IsPositive() {
					wins++
					lastTradeTick = tick + r.cfg.WinCooldown
				} else if profit.IsNegative() {
					losses++
					lastTradeTick = tick + r.cfg.LossCooldown
				} else {
					lastTradeTick = tick
				}
			}
		}
	}

	session := map[string]interface{}{
		"mode":          "backtest",
		"symbol":        symbol,
		"total_trades":  totalTrades,
		"wins":          wins,
		"losses":        losses,
		"net_pnl":       r.positionManager.Balance().InexactFloat64(),
		"final_balance": r.positionManager.Balance().InexactFloat64(),
		"profit_factor": 0.0,
		"max_drawdown":  0.0,
		"sharpe_ratio":  0.0,
		"expectancy":    0.0,
		"status":        "completed",
	}
	r.publisher.Publish(context.Background(), "sessions", eventdef.NewEvent("session.completed", "backtest", 1, session))

	fmt.Println()
	fmt.Println("==================== BACKTEST RESULTS ====================")
	fmt.Printf("  Symbol:          %s\n", symbol)
	fmt.Printf("  Ticks Processed: %d\n", totalTicks)
	fmt.Println("----------------------------------------------------------")
	fmt.Printf("  Total Trades:    %d\n", totalTrades)
	fmt.Printf("  Wins:            %d\n", wins)
	fmt.Printf("  Losses:          %d\n", losses)
	fmt.Printf("  Win Rate:        %.2f%%\n", float64(wins)/float64(totalTrades)*100)
	fmt.Println("----------------------------------------------------------")
	fmt.Printf("  Net PnL:         %s USDT\n", r.positionManager.Balance().StringFixed(4))
	fmt.Printf("  Gross Profit:    %s USDT\n", r.positionManager.Balance().StringFixed(4))
	fmt.Printf("  Gross Loss:      %s USDT\n", "0.0000")
	fmt.Printf("  Final Balance:   %s USDT\n", r.positionManager.Balance().StringFixed(4))
	fmt.Println("----------------------------------------------------------")
	fmt.Printf("  Profit Factor:   %.2f\n", 0.0)
	fmt.Printf("  Expectancy:      %.4f USDT/trade\n", 0.0)
	fmt.Printf("  Max Drawdown:    %.4f USDT\n", 0.0)
	fmt.Printf("  Sharpe Ratio:    %.4f\n", 0.0)
	fmt.Println("==========================================================")

	if r.currentPosition != nil {
		fmt.Printf("\n  [WARN] Position still OPEN: %s @ %s (Last Price: %s)\n",
			r.currentPosition.Side, r.currentPosition.Price, lastPrice)
	}

	return nil
}
