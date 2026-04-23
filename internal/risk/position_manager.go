package risk

import (
	"errors"
	"log"
	"sync"

	"trading-bot/internal/config"
	"trading-bot/internal/domain"
	"trading-bot/internal/exchange"

	"github.com/shopspring/decimal"
)

var ErrInsufficientFunds = errors.New("insufficient funds")

// PositionManager manages the bot's position and balance.

type PositionManager struct {
	mutex               sync.RWMutex
	balance             decimal.Decimal
	maxRiskPerTrade     decimal.Decimal
	currentPosition     *domain.Position
	leverage            decimal.Decimal
	sessionBudget       float64
	restClient          *exchange.RestClient
	takeProfitPct       decimal.Decimal
	stopLossPct         decimal.Decimal
	breakEvenTriggerPct decimal.Decimal
	trailDistancePct    decimal.Decimal
}

// NewPositionManager creates a new PositionManager.
func NewPositionManager(balance decimal.Decimal, maxRiskPerTrade decimal.Decimal, leverage int, sessionBudget float64, restClient *exchange.RestClient, takeProfitPct, stopLossPct, breakEvenTriggerPct, trailDistancePct float64) *PositionManager {
	return &PositionManager{
		balance:             balance,
		maxRiskPerTrade:     maxRiskPerTrade,
		leverage:            decimal.NewFromInt(int64(leverage)),
		sessionBudget:       sessionBudget,
		restClient:          restClient,
		takeProfitPct:       decimal.NewFromFloat(takeProfitPct),
		stopLossPct:         decimal.NewFromFloat(stopLossPct),
		breakEvenTriggerPct: decimal.NewFromFloat(breakEvenTriggerPct),
		trailDistancePct:    decimal.NewFromFloat(trailDistancePct),
	}
}

// NewBacktestPositionManager creates a new PositionManager for backtesting.
func NewBacktestPositionManager(cfg *config.Config) *PositionManager {
	return &PositionManager{
		balance:             decimal.NewFromFloat(cfg.PaperBalance),
		maxRiskPerTrade:     decimal.NewFromFloat(0.1), // Example value
		leverage:            decimal.NewFromInt(int64(cfg.Leverage)),
		sessionBudget:       cfg.PaperBalance,
		restClient:          &exchange.RestClient{StepSize: decimal.NewFromFloat(0.1)}, // Mock RestClient with default StepSize
		takeProfitPct:       decimal.NewFromFloat(cfg.TakeProfitPct),
		stopLossPct:         decimal.NewFromFloat(cfg.StopLossPct),
		breakEvenTriggerPct: decimal.NewFromFloat(cfg.BreakEvenTriggerPct),
		trailDistancePct:    decimal.NewFromFloat(cfg.TrailDistancePct),
	}
}

// SetLeverage sets the leverage for the position manager.
func (pm *PositionManager) SetLeverage(leverage int64) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	pm.leverage = decimal.NewFromInt(leverage)
}

// CalculatePositionSize calculates the position size based on the 10 USDT budget.
func (pm *PositionManager) CalculatePositionSize(price decimal.Decimal, availableBalance decimal.Decimal) (decimal.Decimal, error) {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	log.Printf("Available balance: %s. Ignoring for budget calculation.", availableBalance)

	// This is the position size in USDT, which is 100 according to rule-8
	positionValue := decimal.NewFromFloat(100.0)

	// Quantity = positionValue / price
	quantity := positionValue.Div(price)

	if quantity.IsZero() {
		return decimal.Zero, errors.New("calculated quantity is zero")
	}

	if pm.restClient.StepSize.IsZero() {
		return quantity, nil
	}
	roundedQuantity := quantity.Div(pm.restClient.StepSize).Floor().Mul(pm.restClient.StepSize)

	return roundedQuantity, nil
}

// Balance returns the current balance.
func (pm *PositionManager) Balance() decimal.Decimal {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	return pm.balance
}

func (pm *PositionManager) UpdateBalance(profit decimal.Decimal) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	pm.balance = pm.balance.Add(profit)
}

func (pm *PositionManager) GetTakeProfitPct() decimal.Decimal {
	return pm.takeProfitPct
}

func (pm *PositionManager) GetStopLossPct() decimal.Decimal {
	return pm.stopLossPct
}

func (pm *PositionManager) SetCurrentPosition(p *domain.Position) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	p.HighestPrice = p.Price
	pm.currentPosition = p
}

func (pm *PositionManager) Evaluate(currentPrice, rsi decimal.Decimal) (exit bool, reason string) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if pm.currentPosition == nil {
		return false, ""
	}

	p := pm.currentPosition

	var currentProfitPct decimal.Decimal
	if p.Side == string(domain.SignalBuy) {
		currentProfitPct = currentPrice.Sub(p.Price).Div(p.Price)
	} else {
		currentProfitPct = p.Price.Sub(currentPrice).Div(p.Price)
	}

	// Break-even logic
	if !p.IsBreakEvenSet && currentProfitPct.GreaterThanOrEqual(pm.breakEvenTriggerPct) {
		p.StopLossPrice = p.Price
		p.IsBreakEvenSet = true
		log.Println("[SAFETY] Move StopLoss to Break-Even")
	}

	// Trailing stop logic
	if p.IsBreakEvenSet {
		if p.Side == string(domain.SignalBuy) {
			if currentPrice.GreaterThan(p.HighestPrice) {
				p.HighestPrice = currentPrice
				newStopLoss := p.HighestPrice.Mul(decimal.NewFromFloat(1).Sub(pm.trailDistancePct))
				if newStopLoss.GreaterThan(p.StopLossPrice) {
					p.StopLossPrice = newStopLoss
				}
			}
		} else { // SignalSell
			if currentPrice.LessThan(p.HighestPrice) {
				p.HighestPrice = currentPrice
				newStopLoss := p.HighestPrice.Mul(decimal.NewFromFloat(1).Add(pm.trailDistancePct))
				if newStopLoss.LessThan(p.StopLossPrice) {
					p.StopLossPrice = newStopLoss
				}
			}
		}
	}

	if p.Side == string(domain.SignalBuy) {
		if currentPrice.GreaterThanOrEqual(p.TakeProfitPrice) {
			return true, "TAKE_PROFIT"
		}
		if currentPrice.LessThanOrEqual(p.StopLossPrice) {
			if p.IsBreakEvenSet {
				return true, "BREAK_EVEN"
			}
			return true, "STOP_LOSS"
		}
	} else if p.Side == string(domain.SignalSell) {
		if currentPrice.GreaterThanOrEqual(p.StopLossPrice) {
			if p.IsBreakEvenSet {
				return true, "BREAK_EVEN"
			}
			return true, "STOP_LOSS"
		}
		if currentPrice.LessThanOrEqual(p.TakeProfitPrice) {
			return true, "TAKE_PROFIT"
		}
	}

	// RSI Exhaustion
	if p.Side == string(domain.SignalBuy) && rsi.GreaterThan(decimal.NewFromInt(70)) {
		return true, "RSI_EXHAUSTION"
	} else if p.Side == string(domain.SignalSell) && rsi.LessThan(decimal.NewFromInt(30)) {
		return true, "RSI_EXHAUSTION"
	}

	return false, ""
}
