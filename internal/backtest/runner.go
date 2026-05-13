package backtest

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"trading-bot/internal/analytics"
	"trading-bot/internal/config"
	"trading-bot/internal/database"
	"trading-bot/internal/trading"
	"trading-bot/internal/trading/domain"

	"github.com/shopspring/decimal"
)

type Runner struct {
	strategy        trading.Strategy
	positionManager *trading.PositionManager
	currentPosition *domain.Position
	cfg             *config.Config
	repo            database.Repository
}

func NewRunner(strategy trading.Strategy, positionManager *trading.PositionManager, cfg *config.Config, repo database.Repository) *Runner {
	return &Runner{
		strategy:        strategy,
		positionManager: positionManager,
		cfg:             cfg,
		repo:            repo,
	}
}

type HistoricalData struct {
	Time  string          `json:"time"`
	Price decimal.Decimal `json:"price"`
}

func (r *Runner) Start(ctx context.Context) {
	go r.Run(r.cfg.BacktestFile)
}

func (r *Runner) Stop() {}

func (r *Runner) GetStatus() domain.Status {
	return domain.Status{}
}

func (r *Runner) Run(filePath string) error {
	log.Printf("Backtest starting: file=%s symbol=%s", filePath, r.cfg.Symbol)

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

	return r.RunFromCandles(candles)
}

func (r *Runner) RunFromCandles(candles []domain.Candle) error {
	var tradeResults []analytics.TradeResult
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
				qty, err := r.positionManager.CalculatePositionSize(candle.Close, r.positionManager.Balance())
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
					Symbol:          r.cfg.Symbol,
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
					Symbol:     r.cfg.Symbol,
					Side:       r.currentPosition.Side,
					EntryPrice: r.currentPosition.Price,
					ExitPrice:  candle.Close,
					Profit:     profit,
					ExitReason: reason,
				}
				r.repo.SaveTrade(dbTrade)

				tradeResults = append(tradeResults, analytics.TradeResult{
					PnL:  profit,
					Side: r.currentPosition.Side,
				})

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

	report := analytics.Analyze(tradeResults)

	session := &database.Session{
		Mode:         "backtest",
		Symbol:       r.cfg.Symbol,
		TotalTrades:  report.TotalTrades,
		Wins:         report.Wins,
		Losses:       report.Losses,
		NetPnL:       report.NetPnL,
		FinalBalance: r.positionManager.Balance().InexactFloat64(),
		ProfitFactor: report.ProfitFactor,
		MaxDrawdown:  report.MaxDrawdown,
		SharpeRatio:  report.SharpeRatio,
		Expectancy:   report.Expectancy,
		Status:       "completed",
	}
	r.repo.SaveSession(session)

	fmt.Println()
	fmt.Println("==================== BACKTEST RESULTS ====================")
	fmt.Printf("  Symbol:          %s\n", r.cfg.Symbol)
	fmt.Printf("  Ticks Processed: %d\n", totalTicks)
	fmt.Println("----------------------------------------------------------")
	fmt.Printf("  Total Trades:    %d\n", report.TotalTrades)
	fmt.Printf("  Wins:            %d\n", report.Wins)
	fmt.Printf("  Losses:          %d\n", report.Losses)
	fmt.Printf("  Win Rate:        %.2f%%\n", report.WinRate)
	fmt.Println("----------------------------------------------------------")
	fmt.Printf("  Net PnL:         %.4f USDT\n", report.NetPnL)
	fmt.Printf("  Gross Profit:    %.4f USDT\n", report.GrossProfit)
	fmt.Printf("  Gross Loss:      %.4f USDT\n", report.GrossLoss)
	fmt.Printf("  Final Balance:   %s USDT\n", r.positionManager.Balance().StringFixed(4))
	fmt.Println("----------------------------------------------------------")
	fmt.Printf("  Profit Factor:   %.2f\n", report.ProfitFactor)
	fmt.Printf("  Expectancy:      %.4f USDT/trade\n", report.Expectancy)
	fmt.Printf("  Max Drawdown:    %.4f USDT\n", report.MaxDrawdown)
	fmt.Printf("  Sharpe Ratio:    %.4f\n", report.SharpeRatio)
	fmt.Println("==========================================================")

	if r.currentPosition != nil {
		fmt.Printf("\n  [WARN] Position still OPEN: %s @ %s (Last Price: %s)\n",
			r.currentPosition.Side, r.currentPosition.Price, lastPrice)
	}

	return nil
}
