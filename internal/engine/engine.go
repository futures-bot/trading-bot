package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"trading-bot/internal/config"
	"trading-bot/internal/domain"
	"trading-bot/internal/exchange"
	"trading-bot/internal/logging"
	"trading-bot/internal/risk"
	"trading-bot/internal/strategy"

	"github.com/adshao/go-binance/v2/common"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// Engine orchestrates the trading bot's operations.
type Engine struct {
	config            *config.Config
	tracker           *strategy.TradeTracker
	pm                *risk.PositionManager
	emaStrategy       strategy.Strategy
	client            *exchange.Client
	pnlLogger         *logging.PnlLogger
	marketPulseLogger *logging.MarketPulseLogger
	sugar             *zap.SugaredLogger
	startTime         time.Time
	sessionNumber     int
	lastPrice         decimal.Decimal
	availableBalance  decimal.Decimal
	currentTrade      *logging.TradeLog
	candles           []domain.Candle

	mutex sync.RWMutex
}

// New creates a new Engine instance.
func New(cfg *config.Config, sugar *zap.SugaredLogger, pnlLogger *logging.PnlLogger, marketPulseLogger *logging.MarketPulseLogger, startTime time.Time, sessionNumber int, availableBalance decimal.Decimal) (*Engine, error) {
	client, err := exchange.New(cfg, sugar)
	if err != nil {
		return nil, fmt.Errorf("failed to create exchange client: %w", err)
	}

	pm := risk.NewPositionManager(
		decimal.NewFromFloat(cfg.SessionBudget),
		decimal.NewFromFloat(0.1), // 10% risk per trade
		cfg.Leverage,
		cfg.SessionBudget,
		client.RestClient,
		cfg.TakeProfitPct,
		cfg.StopLossPct,
	)

	tracker := strategy.NewTradeTracker(
		cfg.TakeProfitPct,
		cfg.StopLossPct,
		cfg.ConfirmationCount,
		cfg.MinProfitForFlipExit,
	)

	emaStrategy := strategy.NewEMACrossover(
		cfg.EMAFast,
		cfg.EMASlow,
		5*time.Minute, // Cooldown
		tracker,
	)

	return &Engine{
		config:            cfg,
		tracker:           tracker,
		pm:                pm,
		emaStrategy:       emaStrategy,
		client:            client,
		pnlLogger:         pnlLogger,
		marketPulseLogger: marketPulseLogger,
		sugar:             sugar,
		startTime:         startTime,
		sessionNumber:     sessionNumber,
		availableBalance:  availableBalance,
	}, nil
}

// Run starts the trading bot's main loop.
func (e *Engine) Run(ctx context.Context) {
	priceCh := make(chan exchange.PriceUpdate)
	go e.client.Start(ctx, e.config.Symbol, priceCh)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	dashboardTicker := time.NewTicker(10 * time.Second)
	defer dashboardTicker.Stop()

	marketPulseTicker := time.NewTicker(5 * time.Minute)
	defer marketPulseTicker.Stop()

	for {
		select {
		case priceUpdate := <-priceCh:
			price, _ := decimal.NewFromString(priceUpdate.Price)
			e.lastPrice = price

			// Add to candles
			newCandle := domain.Candle{Open: price, High: price, Low: price, Close: price}
			e.candles = append(e.candles, newCandle)

			if len(e.candles) < e.config.EMASlow {
				e.sugar.Infof("Collecting price points, %d/%d...", len(e.candles), e.config.EMASlow)
				continue
			}

			signal := e.emaStrategy.Calculate(e.candles)

			if e.tracker.InTrade() {
				if exit, reason := e.pm.Evaluate(e.lastPrice); exit {
					log.Printf("EXIT TRIGGER: %s reached", reason)
					quantity, err := e.pm.CalculatePositionSize(e.lastPrice, e.availableBalance)
					if err != nil {
						e.sugar.Errorw("Failed to calculate position size for exit", "error", err)
						return
					}
					side := futures.SideTypeSell
					if e.tracker.GetSide() == domain.SignalSell {
						side = futures.SideTypeBuy
					}
					e.closePosition(ctx, side, quantity, reason)
					continue
				}
			}

			switch signal {
			case domain.SignalBuy:
				e.executeBuy(ctx)
			case domain.SignalSell:
				e.executeSell(ctx)
			}

		case <-ticker.C:
			e.sugar.Info("Reconciliation loop triggered")
		case <-dashboardTicker.C:
			e.updateDashboard(ctx)
		case <-marketPulseTicker.C:
			if !e.tracker.InTrade() {
				// e.marketPulseLogger.Log(logging.MarketPulseLog{
				// 	Price:  e.lastPrice,
				// 	RSI:    e.emaStrategy.GetRSI(),
				// 	EMAGap: e.emaStrategy.GetEMAGap(),
				// })
			}
		case <-ctx.Done():
			return
		}
	}
}

func (e *Engine) executeBuy(ctx context.Context) {
	if !e.tracker.InTrade() && e.tracker.CanTrade() {
		// if e.emaStrategy.GetEMAGap().Abs().GreaterThan(decimal.NewFromFloat(e.config.MinEMAGap)) {
		e.tracker.SetLastTradeAttempt()
		e.sugar.Info("Buy signal")
		quantity, err := e.pm.CalculatePositionSize(e.lastPrice, e.availableBalance)
		if err != nil {
			e.sugar.Errorw("Failed to calculate position size", "error", err)
		} else {
			e.openPosition(ctx, futures.SideTypeBuy, quantity)
		}
		// } else {
		// 	e.sugar.Info("EMA gap too small, skipping trade")
		// }
	}
}

func (e *Engine) executeSell(ctx context.Context) {
	if !e.tracker.InTrade() && e.tracker.CanTrade() {
		// if e.emaStrategy.GetEMAGap().Abs().GreaterThan(decimal.NewFromFloat(e.config.MinEMAGap)) {
		e.tracker.SetLastTradeAttempt()
		e.sugar.Info("Sell signal")
		quantity, err := e.pm.CalculatePositionSize(e.lastPrice, e.availableBalance)
		if err != nil {
			e.sugar.Errorw("Failed to calculate position size", "error", err)
		} else {
			e.openPosition(ctx, futures.SideTypeSell, quantity)
		}
		// } else {
		// 	e.sugar.Info("EMA gap too small, skipping trade")
		// }
	}
}

func (e *Engine) openPosition(ctx context.Context, side futures.SideType, quantity decimal.Decimal) {
	budget := decimal.NewFromFloat(e.config.SessionBudget)

	log.Printf("CRITICAL DEBUG: Type of qty variable is %T and value is %v", quantity, quantity)
	log.Printf("DEBUG: Budget=%v, Price=%v, Calculated Qty=%v", budget, e.lastPrice, quantity)
	formattedQuantity := quantity.StringFixed(2)
	order, err := e.client.PlaceOrder(ctx, e.config.Symbol, side, formattedQuantity)
	if err != nil {
		if apiErr, ok := err.(*common.APIError); ok && apiErr.Code == -2019 {
			e.hardShutdown("MARGIN_CALL")
		} else {
			e.sugar.Errorw("Failed to place order", "error", err)
		}
		return
	}

	e.sugar.Infow("Placed order", "order", order)
	entryPrice := decimal.Zero
	time.Sleep(1 * time.Second)
	trades, err := e.client.GetAccountTradeList(ctx, e.config.Symbol, order.OrderID)
	if err != nil {
		e.sugar.Errorw("Failed to get account trade list for entry price", "error", err)
	} else if len(trades) > 0 {
		entryPrice, _ = decimal.NewFromString(trades[0].Price)
	}

	if entryPrice.IsZero() {
		time.Sleep(2 * time.Second)
		trades, err := e.client.GetAccountTradeList(ctx, e.config.Symbol, order.OrderID)
		if err != nil {
			e.sugar.Errorw("Failed to get account trade list for entry price on retry", "error", err)
		} else if len(trades) > 0 {
			entryPrice, _ = decimal.NewFromString(trades[0].Price)
		}
	}

	if entryPrice.IsZero() {
		e.sugar.Warnw("Could not determine entry price, aborting trade")
		return
	}

	e.sugar.Infow("Execution Verified:", "Entry", entryPrice)

	var signalSide domain.Signal
	if side == futures.SideTypeBuy {
		signalSide = domain.SignalBuy
	} else {
		signalSide = domain.SignalSell
	}

	e.tracker.StartTrade(entryPrice, signalSide)
	e.currentTrade = &logging.TradeLog{
		Entry: entryPrice,
		Side:  string(signalSide),
	}
}

func (e *Engine) closePosition(ctx context.Context, side futures.SideType, quantity decimal.Decimal, exitReason string) {
	budget := decimal.NewFromFloat(e.config.SessionBudget)
	log.Printf("DEBUG: Budget=%v, Price=%v, Calculated Qty=%v", budget, e.lastPrice, quantity)
	formattedQuantity := quantity.StringFixed(2)
	order, err := e.client.PlaceOrder(ctx, e.config.Symbol, side, formattedQuantity)
	if err != nil {
		if apiErr, ok := err.(*common.APIError); ok && apiErr.Code == -2019 {
			e.hardShutdown("MARGIN_CALL")
		} else {
			e.sugar.Errorw("Failed to place order", "error", err)
		}
		return
	}

	e.sugar.Infow("Placed order", "order", order)
	time.Sleep(1500 * time.Millisecond)

	trades, err := e.client.GetAccountTradeList(ctx, e.config.Symbol, order.OrderID)
	if err != nil {
		e.sugar.Errorw("Failed to get account trade list for exit price", "error", err)
		return
	}

	if len(trades) == 0 {
		e.sugar.Errorw("Could not determine exit price, trade will not be logged")
		return
	}

	exitPrice, _ := decimal.NewFromString(trades[0].Price)
	e.sugar.Infow("Execution Verified:", "Exit", exitPrice)

	quantity, _ = decimal.NewFromString(order.OrigQuantity)
	var pnl decimal.Decimal

	if e.currentTrade.Side == "Buy" { // This was a long position
		pnl = exitPrice.Sub(e.currentTrade.Entry).Mul(quantity)
	} else { // This was a short position
		pnl = e.currentTrade.Entry.Sub(exitPrice).Mul(quantity)
	}

	if pnl.Abs().GreaterThan(decimal.NewFromFloat(e.config.SessionBudget)) {
		e.sugar.Errorw("Logic Error: PNL exceeds session budget, ignoring.", "pnl", pnl)
	} else if pnl.LessThan(decimal.NewFromFloat(-1.5)) {
		e.sugar.Errorw("Panic Protocol Triggered! Loss > 1.5 USDT", "pnl", pnl)
		e.hardShutdown("PANIC")
	}

	e.currentTrade.Exit = exitPrice
	e.currentTrade.PnlUSDT = pnl
	// e.currentTrade.RSI = e.emaStrategy.GetRSI()
	// e.currentTrade.EMAGap = e.emaStrategy.GetEMAGap()
	// e.currentTrade.Volatility = e.emaStrategy.GetVolatility()
	e.currentTrade.ExitReason = exitReason

	e.pnlLogger.LogTrade(*e.currentTrade)
	e.tracker.EndTrade()
	e.currentTrade = nil
}

func (e *Engine) updateDashboard(ctx context.Context) {
	balance, err := e.client.VerifyCredentialsAndGetBalance(ctx)
	if err != nil {
		e.sugar.Errorw("Failed to get account in dashboard", "error", err)
	} else {
		availableBalance, err := decimal.NewFromString(balance.AvailableBalance)
		if err != nil {
			e.sugar.Errorw("Failed to parse available balance in dashboard", "error", err)
		} else {
			if availableBalance.LessThan(decimal.NewFromInt(2)) {
				e.hardShutdown("LIQUIDATED_OR_EMPTY")
			}
			e.availableBalance = availableBalance
		}
	}
	e.printDashboard()
}

func (e *Engine) hardShutdown(status string) {
	e.sugar.Info("CRITICAL: Margin Depleted or Panic Triggered. Ending Session.")

	err := e.client.RestClient.CancelAllOpenOrders(context.Background(), e.config.Symbol)
	if err != nil {
		e.sugar.Errorw("Failed to cancel open orders", "error", err)
	}

	balance, err := e.client.RestClient.VerifyCredentialsAndGetBalance(context.Background())
	if err != nil {
		e.sugar.Errorw("Failed to get final balance", "error", err)
	}
	finalBalance, _ := decimal.NewFromString(balance.Balance)

	duration := time.Since(e.startTime)
	totalTrades := e.pnlLogger.GetTotalTrades()
	netPnL := e.pnlLogger.GetTotalProfit()

	history := History{
		SessionNumber: e.sessionNumber,
		Duration:      duration,
		TotalTrades:   totalTrades,
		NetPnL:        netPnL,
		FinalBalance:  finalBalance,
		Status:        status,
	}

	f, err := os.OpenFile("history.jsonl", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open history.jsonl: %v", err)
	}
	defer f.Close()

	jsonHistory, err := json.Marshal(history)
	if err != nil {
		log.Fatalf("Failed to marshal history: %v", err)
	}

	if _, err := f.Write(jsonHistory); err != nil {
		log.Fatalf("Failed to write to history.jsonl: %v", err)
	}

	os.Exit(0)
}

func (e *Engine) printDashboard() {
	log.Println("--------------------------------------------------")
	log.Printf("Time: %s", time.Now().Format("15:04:05"))
	log.Printf("[BUDGET] Session Budget: %s USDT", decimal.NewFromFloat(e.config.SessionBudget).StringFixed(2))
	if e.tracker.InTrade() {
		entryPrice := e.tracker.GetEntryPrice()
		pnl := e.lastPrice.Sub(entryPrice).Div(entryPrice).Mul(decimal.NewFromInt(100))
		if e.tracker.GetSide() == domain.SignalSell {
			pnl = pnl.Neg()
		}
		log.Printf("[ACTIVE TRADE] PnL: %s%%", pnl.StringFixed(2))
	} else {
		log.Println("[ACTIVE TRADE] No active trade")
	}

	log.Printf("[SESSION STATS] Total Trades: %d | Wins: %d | Losses: %d | Win Rate: %.2f%% | Total Profit: %s USDT",
		e.pnlLogger.GetTotalTrades(),
		e.pnlLogger.GetWins(),
		e.pnlLogger.GetLosses(),
		e.pnlLogger.GetWinRate(),
		e.pnlLogger.GetTotalProfit().StringFixed(2),
	)
	log.Println("--------------------------------------------------")
}

type History struct {
	SessionNumber int             `json:"session_number"`
	Duration      time.Duration   `json:"duration"`
	TotalTrades   int             `json:"total_trades"`
	NetPnL        decimal.Decimal `json:"net_pnl"`
	FinalBalance  decimal.Decimal `json:"final_balance"`
	Status        string          `json:"status,omitempty"`
}
