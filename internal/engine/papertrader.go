package engine

import (
	"context"
	"log"

	"trading-bot/internal/config"
	"trading-bot/internal/domain"
	"trading-bot/internal/exchange"
	"trading-bot/internal/logging"
	"trading-bot/internal/risk"
	"trading-bot/internal/strategy"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// PaperTrader simulates live trading without executing real orders.
type PaperTrader struct {
	cfg             *config.Config
	sugar           *zap.SugaredLogger
	pnlLogger       *logging.PnlLogger
	positionManager *risk.PositionManager
	strategy        strategy.Strategy
	balance         decimal.Decimal
	currentPosition *domain.Position
	candles         []domain.Candle
}

// NewPaperTrader creates a new PaperTrader.
func NewPaperTrader(cfg *config.Config, sugar *zap.SugaredLogger, pnlLogger *logging.PnlLogger) (*PaperTrader, error) {
	pm := risk.NewBacktestPositionManager(cfg)
	tradeTracker := strategy.NewTradeTracker(cfg.TakeProfitPct, cfg.StopLossPct, cfg.ConfirmationCount, cfg.MinProfitForFlipExit)
	strategy := strategy.NewEMACrossover(cfg.EMAFast, cfg.EMASlow, 0, tradeTracker)

	return &PaperTrader{
		cfg:             cfg,
		sugar:           sugar,
		pnlLogger:       pnlLogger,
		positionManager: pm,
		strategy:        strategy,
		balance:         decimal.NewFromFloat(1000.0),
	}, nil
}

// Run starts the paper trading loop.
func (pt *PaperTrader) Run(ctx context.Context) {
	log.Println("Starting paper trader...")

	priceCh := make(chan exchange.PriceUpdate)
	client, err := exchange.New(pt.cfg, pt.sugar)
	if err != nil {
		log.Fatalf("Failed to create exchange client: %v", err)
	}

	go client.Start(ctx, pt.cfg.Symbol, priceCh)

	for {
		select {
		case priceUpdate := <-priceCh:
			price, _ := decimal.NewFromString(priceUpdate.Price)

			newCandle := domain.Candle{Open: price, High: price, Low: price, Close: price}
			pt.candles = append(pt.candles, newCandle)
			if len(pt.candles) > pt.cfg.EMASlow {
				pt.candles = pt.candles[1:]
			}

			if len(pt.candles) < pt.cfg.EMASlow {
				continue
			}

			signal, rsi := pt.strategy.Calculate(pt.candles)

			if pt.currentPosition == nil {
				if signal == domain.SignalBuy || signal == domain.SignalSell {
					qty, _ := pt.positionManager.CalculatePositionSize(price, pt.balance)
					stopLossPct := pt.positionManager.GetStopLossPct()
					takeProfitPct := pt.positionManager.GetTakeProfitPct()

					var stopLossPrice, takeProfitPrice decimal.Decimal
					if signal == domain.SignalBuy {
						stopLossPrice = price.Mul(decimal.NewFromFloat(1).Sub(stopLossPct.Div(decimal.NewFromInt(100))))
						takeProfitPrice = price.Mul(decimal.NewFromFloat(1).Add(takeProfitPct.Div(decimal.NewFromInt(100))))
					} else {
						stopLossPrice = price.Mul(decimal.NewFromFloat(1).Add(stopLossPct.Div(decimal.NewFromInt(100))))
						takeProfitPrice = price.Mul(decimal.NewFromFloat(1).Sub(takeProfitPct.Div(decimal.NewFromInt(100))))
					}

					pt.currentPosition = &domain.Position{
						Symbol:          pt.cfg.Symbol,
						Side:            string(signal),
						Quantity:        qty,
						Price:           price,
						StopLossPrice:   stopLossPrice,
						TakeProfitPrice: takeProfitPrice,
					}
					pt.positionManager.SetCurrentPosition(pt.currentPosition)
					log.Printf("[PAPER] Opened %s position at %s", signal, price)
				}
			} else {
				exit, reason := pt.positionManager.Evaluate(price, rsi)
				if exit {
					profit := price.Sub(pt.currentPosition.Price).Mul(pt.currentPosition.Quantity)
					if pt.currentPosition.Side == string(domain.SignalSell) {
						profit = pt.currentPosition.Price.Sub(price).Mul(pt.currentPosition.Quantity)
					}
					pt.balance = pt.balance.Add(profit)

					tradeLog := logging.TradeLog{
						Entry:      pt.currentPosition.Price,
						Exit:       price,
						Side:       pt.currentPosition.Side,
						PnlUSDT:    profit,
						ExitReason: reason,
					}
					pt.pnlLogger.LogTrade(tradeLog)

					log.Printf("[PAPER] Closed position at %s for a profit of %s. Reason: %s", price, profit, reason)
					pt.currentPosition = nil
				}
			}

		case <-ctx.Done():
			return
		}
	}
}
