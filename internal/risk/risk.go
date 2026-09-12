package risk

import (
	"errors"
	"log"
	"sync"

	"trading-bot/internal/config"
	"trading-bot/internal/trading/domain"

	"github.com/shopspring/decimal"
)

// SharedBudget tracks allocated trade budgets dynamically across concurrent symbols.
type SharedBudget struct {
	mu    sync.Mutex
	total decimal.Decimal
	used  decimal.Decimal
}

func NewSharedBudget(total decimal.Decimal) *SharedBudget {
	return &SharedBudget{
		total: total,
	}
}

func (sb *SharedBudget) Allocate(amount decimal.Decimal) (decimal.Decimal, bool) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	remaining := sb.total.Sub(sb.used)
	if remaining.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, false
	}

	allocated := amount
	if allocated.GreaterThan(remaining) {
		allocated = remaining
	}

	sb.used = sb.used.Add(allocated)
	return allocated, true
}

func (sb *SharedBudget) Release(amount decimal.Decimal) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	sb.used = sb.used.Sub(amount)
	if sb.used.LessThan(decimal.Zero) {
		sb.used = decimal.Zero
	}
}

// Engine evaluates trade safety, sizing, and exits.
type Engine struct {
	MaxRiskPerTrade     decimal.Decimal
	Leverage            decimal.Decimal
	SessionBudget       decimal.Decimal
	TakeProfitPct       decimal.Decimal
	StopLossPct         decimal.Decimal
	BreakEvenTriggerPct decimal.Decimal
	TrailDistancePct    decimal.Decimal
	StepSize            decimal.Decimal

	SharedBudget *SharedBudget
}

// NewEngine constructs a new risk engine from configuration.
func NewEngine(cfg *config.Config, stepSize decimal.Decimal) *Engine {
	if stepSize.IsZero() {
		stepSize = decimal.NewFromFloat(0.01) // safe fallback
	}
	return &Engine{
		MaxRiskPerTrade:     decimal.NewFromFloat(0.1),
		Leverage:            decimal.NewFromInt(int64(cfg.Leverage)),
		SessionBudget:       decimal.NewFromFloat(cfg.SessionBudget),
		TakeProfitPct:       decimal.NewFromFloat(cfg.TakeProfitPct),
		StopLossPct:         decimal.NewFromFloat(cfg.StopLossPct),
		BreakEvenTriggerPct: decimal.NewFromFloat(cfg.BreakEvenTriggerPct),
		TrailDistancePct:    decimal.NewFromFloat(cfg.TrailDistancePct),
		StepSize:            stepSize,
	}
}

// CalculatePositionSize determines the safe order quantity based on budget, leverage, and step size.
func (e *Engine) CalculatePositionSize(price decimal.Decimal, availableBalance decimal.Decimal, trendStrength decimal.Decimal) (decimal.Decimal, error) {
	// Modified to use 100% of session budget as requested by user to avoid small orders.
	factor := decimal.NewFromFloat(1.0)

	targetBudget := e.SessionBudget.Mul(factor)

	budget := targetBudget
	if e.SharedBudget != nil {
		allocated, ok := e.SharedBudget.Allocate(targetBudget)
		if !ok || allocated.IsZero() {
			return decimal.Zero, errors.New("no shared budget available")
		}
		budget = allocated
	}

	// Sizing: Use the allocated budget multiplied by leverage (divided by price)
	leverage := e.Leverage
	if leverage.IsZero() {
		leverage = decimal.NewFromInt(1)
	}
	quantity := budget.Mul(leverage).Div(price)

	if quantity.IsZero() {
		if e.SharedBudget != nil {
			e.SharedBudget.Release(budget)
		}
		return decimal.Zero, errors.New("calculated quantity is zero")
	}

	// Apply step size rounding (e.g. floored to step size)
	finalQty := quantity.Div(e.StepSize).Floor().Mul(e.StepSize)

	// Ensure the floored quantity is not zero after step size rounding
	if finalQty.IsZero() {
		if e.SharedBudget != nil {
			e.SharedBudget.Release(budget)
		}
		return decimal.Zero, errors.New("calculated quantity is zero after step size rounding")
	}

	// Adjust the allocated budget to match actual quantity cost (margin used)
	if e.SharedBudget != nil {
		actualCost := finalQty.Mul(price).Div(leverage)
		diff := budget.Sub(actualCost)
		if diff.IsPositive() {
			e.SharedBudget.Release(diff) // release the unused portion of the allocated block
		}
	}

	return finalQty, nil
}

// EvaluateExit determines if an active position should be closed.
func (e *Engine) EvaluateExit(p *domain.Position, currentPrice, rsi decimal.Decimal) (exit bool, reason string) {
	if p == nil {
		return false, ""
	}

	var currentProfitPct decimal.Decimal
	if p.Side == string(domain.SignalBuy) {
		currentProfitPct = currentPrice.Sub(p.Price).Div(p.Price)
	} else {
		currentProfitPct = p.Price.Sub(currentPrice).Div(p.Price)
	}

	// Trailing Stop-Loss Logic: Move SL to break-even once target triggered
	if !p.IsBreakEvenSet && currentProfitPct.GreaterThanOrEqual(e.BreakEvenTriggerPct) {
		p.StopLossPrice = p.Price
		p.IsBreakEvenSet = true
		log.Println("[SAFETY] Move StopLoss to Break-Even")
	}

	// Trail SL up (for Longs) or down (for Shorts)
	if p.IsBreakEvenSet {
		if p.Side == string(domain.SignalBuy) {
			if currentPrice.GreaterThan(p.HighestPrice) {
				p.HighestPrice = currentPrice
				newSL := p.HighestPrice.Mul(decimal.NewFromFloat(1).Sub(e.TrailDistancePct))
				if newSL.GreaterThan(p.StopLossPrice) {
					p.StopLossPrice = newSL
				}
			}
		} else {
			if currentPrice.LessThan(p.HighestPrice) {
				p.HighestPrice = currentPrice
				newSL := p.HighestPrice.Mul(decimal.NewFromFloat(1).Add(e.TrailDistancePct))
				if newSL.LessThan(p.StopLossPrice) {
					p.StopLossPrice = newSL
				}
			}
		}
	}

	// Hard Exits
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

	// RSI Exhaustion Exits
	if p.Side == string(domain.SignalBuy) && rsi.GreaterThan(decimal.NewFromInt(70)) {
		return true, "RSI_EXHAUSTION"
	} else if p.Side == string(domain.SignalSell) && rsi.LessThan(decimal.NewFromInt(30)) {
		return true, "RSI_EXHAUSTION"
	}

	return false, ""
}
