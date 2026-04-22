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
	mutex           sync.RWMutex
	balance         decimal.Decimal
	maxRiskPerTrade decimal.Decimal
	currentPosition *domain.Position
	leverage        decimal.Decimal
	sessionBudget   float64
	restClient      *exchange.RestClient
	takeProfitPct   decimal.Decimal
	stopLossPct     decimal.Decimal
}

// NewPositionManager creates a new PositionManager.
func NewPositionManager(balance decimal.Decimal, maxRiskPerTrade decimal.Decimal, leverage int, sessionBudget float64, restClient *exchange.RestClient, takeProfitPct, stopLossPct float64) *PositionManager {
	return &PositionManager{
		balance:         balance,
		maxRiskPerTrade: maxRiskPerTrade,
		leverage:        decimal.NewFromInt(int64(leverage)),
		sessionBudget:   sessionBudget,
		restClient:      restClient,
		takeProfitPct:   decimal.NewFromFloat(takeProfitPct),
		stopLossPct:     decimal.NewFromFloat(stopLossPct),
	}
}

// NewBacktestPositionManager creates a new PositionManager for backtesting.
func NewBacktestPositionManager(cfg *config.Config) *PositionManager {
	return &PositionManager{
		balance:         decimal.NewFromFloat(cfg.PaperBalance),
		maxRiskPerTrade: decimal.NewFromFloat(0.1), // Example value
		leverage:        decimal.NewFromInt(int64(cfg.Leverage)),
		sessionBudget:   cfg.PaperBalance,
		restClient:      &exchange.RestClient{StepSize: decimal.NewFromFloat(0.1)}, // Mock RestClient with default StepSize
		takeProfitPct:   decimal.NewFromFloat(cfg.TakeProfitPct),
		stopLossPct:     decimal.NewFromFloat(cfg.StopLossPct),
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

func (pm *PositionManager) Evaluate(currentPrice decimal.Decimal) (exit bool, reason string) {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	if pm.currentPosition == nil {
		return false, ""
	}

	profit := currentPrice.Sub(pm.currentPosition.Price)
	if pm.currentPosition.Side == string(domain.SignalSell) {
		profit = pm.currentPosition.Price.Sub(currentPrice)
	}

	pnlRatio := profit.Div(pm.currentPosition.Price)

	takeProfitRatio := pm.takeProfitPct.Div(decimal.NewFromInt(100))
	stopLossRatio := pm.stopLossPct.Div(decimal.NewFromInt(100))

	if pm.currentPosition.Side == string(domain.SignalBuy) {
		if pnlRatio.GreaterThanOrEqual(takeProfitRatio) {
			return true, "TAKE_PROFIT"
		}
		if pnlRatio.LessThanOrEqual(stopLossRatio.Neg()) {
			return true, "STOP_LOSS"
		}
	} else { // SELL
		if pnlRatio.GreaterThanOrEqual(takeProfitRatio) {
			return true, "TAKE_PROFIT"
		}

		if pnlRatio.LessThanOrEqual(stopLossRatio.Neg()) {
			return true, "STOP_LOSS"
		}
	}

	return false, ""
}
