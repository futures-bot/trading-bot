package trading

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"trading-bot/internal/analytics"
	"trading-bot/internal/config"
	"trading-bot/internal/database"
	"trading-bot/internal/logging"
	"trading-bot/internal/marketdata"
	"trading-bot/internal/notifications"
	"trading-bot/internal/trading/domain"

	"github.com/adshao/go-binance/v2/common"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/shopspring/decimal"
)

var ErrBudgetExhausted = errors.New("budget exhausted")

var ErrInsufficientFunds = errors.New("insufficient funds")

type PositionManager struct {
	mutex               sync.RWMutex
	balance             decimal.Decimal
	maxRiskPerTrade     decimal.Decimal
	currentPosition     *domain.Position
	leverage            decimal.Decimal
	sessionBudget       float64
	restClient          *marketdata.RestClient
	takeProfitPct       decimal.Decimal
	stopLossPct         decimal.Decimal
	breakEvenTriggerPct decimal.Decimal
	trailDistancePct    decimal.Decimal
}

func NewPositionManager(balance decimal.Decimal, maxRiskPerTrade decimal.Decimal, leverage int, sessionBudget float64, restClient *marketdata.RestClient, takeProfitPct, stopLossPct, breakEvenTriggerPct, trailDistancePct float64) *PositionManager {
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

func NewBacktestPositionManager(cfg *config.Config) *PositionManager {
	return &PositionManager{
		balance:             decimal.NewFromFloat(cfg.PaperBalance),
		maxRiskPerTrade:     decimal.NewFromFloat(0.1),
		leverage:            decimal.NewFromInt(int64(cfg.Leverage)),
		sessionBudget:       cfg.SessionBudget,
		restClient:          &marketdata.RestClient{StepSize: decimal.NewFromFloat(0.01)},
		takeProfitPct:       decimal.NewFromFloat(cfg.TakeProfitPct),
		stopLossPct:         decimal.NewFromFloat(cfg.StopLossPct),
		breakEvenTriggerPct: decimal.NewFromFloat(cfg.BreakEvenTriggerPct),
		trailDistancePct:    decimal.NewFromFloat(cfg.TrailDistancePct),
	}
}

func (pm *PositionManager) CalculatePositionSize(price decimal.Decimal, availableBalance decimal.Decimal) (decimal.Decimal, error) {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	positionValue := decimal.NewFromFloat(pm.sessionBudget)
	quantity := positionValue.Div(price)

	if quantity.IsZero() {
		return decimal.Zero, errors.New("calculated quantity is zero")
	}

	if pm.restClient.StepSize.IsZero() {
		return quantity, nil
	}
	return quantity.Div(pm.restClient.StepSize).Floor().Mul(pm.restClient.StepSize), nil
}

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

	if !p.IsBreakEvenSet && currentProfitPct.GreaterThanOrEqual(pm.breakEvenTriggerPct) {
		p.StopLossPrice = p.Price
		p.IsBreakEvenSet = true
		log.Println("[SAFETY] Move StopLoss to Break-Even")
	}

	if p.IsBreakEvenSet {
		if p.Side == string(domain.SignalBuy) {
			if currentPrice.GreaterThan(p.HighestPrice) {
				p.HighestPrice = currentPrice
				newSL := p.HighestPrice.Mul(decimal.NewFromFloat(1).Sub(pm.trailDistancePct))
				if newSL.GreaterThan(p.StopLossPrice) {
					p.StopLossPrice = newSL
				}
			}
		} else {
			if currentPrice.LessThan(p.HighestPrice) {
				p.HighestPrice = currentPrice
				newSL := p.HighestPrice.Mul(decimal.NewFromFloat(1).Add(pm.trailDistancePct))
				if newSL.LessThan(p.StopLossPrice) {
					p.StopLossPrice = newSL
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

	if p.Side == string(domain.SignalBuy) && rsi.GreaterThan(decimal.NewFromInt(70)) {
		return true, "RSI_EXHAUSTION"
	} else if p.Side == string(domain.SignalSell) && rsi.LessThan(decimal.NewFromInt(30)) {
		return true, "RSI_EXHAUSTION"
	}

	return false, ""
}

// Strategy defines the interface for a trading strategy.
type Strategy interface {
	Calculate([]domain.Candle) (domain.Signal, decimal.Decimal)
	UpdateLastTradeTime()
}

type EMACrossover struct {
	fastPeriod   int
	slowPeriod   int
	fastEMA      decimal.Decimal
	slowEMA      decimal.Decimal
	prices       []decimal.Decimal
	rsi          decimal.Decimal
	emadiff      decimal.Decimal
	volatility   decimal.Decimal
	tradeTracker *TradeTracker
	lastSignal   domain.Signal
	ticks        int
	previousRSI  decimal.Decimal
	priceHistory []decimal.Decimal
}

func NewEMACrossover(fastPeriod, slowPeriod int, cooldown time.Duration, tradeTracker *TradeTracker) *EMACrossover {
	return &EMACrossover{
		fastPeriod:   fastPeriod,
		slowPeriod:   slowPeriod,
		tradeTracker: tradeTracker,
		lastSignal:   domain.SignalHold,
	}
}

func (s *EMACrossover) UpdateLastTradeTime() {}

func (s *EMACrossover) GetRSI() decimal.Decimal       { return s.rsi }
func (s *EMACrossover) GetEMAGap() decimal.Decimal     { return s.emadiff }
func (s *EMACrossover) GetVolatility() decimal.Decimal { return s.volatility }
func (s *EMACrossover) GetSignal() domain.Signal       { return s.lastSignal }

func (s *EMACrossover) Calculate(candles []domain.Candle) (domain.Signal, decimal.Decimal) {
	s.prices = []decimal.Decimal{}
	for _, c := range candles {
		s.prices = append(s.prices, c.Close)
	}

	if len(s.prices) < s.slowPeriod {
		return domain.SignalHold, s.rsi
	}

	if len(s.prices) > s.slowPeriod {
		s.prices = s.prices[len(s.prices)-s.slowPeriod:]
	}

	s.fastEMA = calculateEMA(s.prices, s.fastPeriod)
	s.slowEMA = calculateEMA(s.prices, s.slowPeriod)
	s.rsi = calculateRSI(s.prices, 14)
	s.emadiff = s.fastEMA.Sub(s.slowEMA)
	s.volatility = calculateVolatility(s.prices, 20)

	s.ticks++
	if s.ticks%500 == 0 {
		log.Printf("RSI: %s | Price: %s | EMA: %s", s.rsi, s.prices[len(s.prices)-1], s.slowEMA)
	}

	if s.fastEMA.IsZero() || s.slowEMA.IsZero() {
		return domain.SignalHold, s.rsi
	}

	if s.fastEMA.GreaterThan(s.slowEMA) {
		s.lastSignal = domain.SignalBuy
	} else if s.fastEMA.LessThan(s.slowEMA) {
		s.lastSignal = domain.SignalSell
	}

	s.priceHistory = append(s.priceHistory, s.prices[len(s.prices)-1])
	if len(s.priceHistory) > 10 {
		s.priceHistory = s.priceHistory[1:]
	}

	if len(s.priceHistory) < 10 {
		return domain.SignalHold, s.rsi
	}
	price10TicksAgo := s.priceHistory[0]
	if s.prices[len(s.prices)-1].Sub(price10TicksAgo).Abs().LessThan(s.prices[len(s.prices)-1].Mul(decimal.NewFromFloat(0.0002))) {
		return domain.SignalHold, s.rsi
	}

	if s.previousRSI.GreaterThan(decimal.NewFromInt(60)) && s.rsi.LessThan(decimal.NewFromInt(60)) {
		s.previousRSI = s.rsi
		return domain.SignalSell, s.rsi
	}

	if s.previousRSI.LessThan(decimal.NewFromInt(40)) && s.rsi.GreaterThan(decimal.NewFromInt(40)) {
		s.previousRSI = s.rsi
		return domain.SignalBuy, s.rsi
	}

	s.previousRSI = s.rsi
	return domain.SignalHold, s.rsi
}

func calculateEMA(prices []decimal.Decimal, period int) decimal.Decimal {
	if len(prices) < period {
		return decimal.Zero
	}
	sma := calculateSMA(prices[:period])
	k := decimal.NewFromFloat(2.0).Div(decimal.NewFromInt(int64(period + 1)))
	ema := sma
	for i := period; i < len(prices); i++ {
		ema = prices[i].Sub(ema).Mul(k).Add(ema)
	}
	return ema
}

func calculateSMA(prices []decimal.Decimal) decimal.Decimal {
	if len(prices) == 0 {
		return decimal.Zero
	}
	sum := decimal.Zero
	for _, p := range prices {
		sum = sum.Add(p)
	}
	return sum.Div(decimal.NewFromInt(int64(len(prices))))
}

func calculateRSI(prices []decimal.Decimal, period int) decimal.Decimal {
	if len(prices) < period+1 {
		return decimal.Zero
	}
	gains := decimal.Zero
	losses := decimal.Zero
	for i := 1; i < len(prices); i++ {
		diff := prices[i].Sub(prices[i-1])
		if diff.IsPositive() {
			gains = gains.Add(diff)
		} else {
			losses = losses.Add(diff.Abs())
		}
	}
	if losses.IsZero() {
		return decimal.NewFromInt(100)
	}
	avgGain := gains.Div(decimal.NewFromInt(int64(period)))
	avgLoss := losses.Div(decimal.NewFromInt(int64(period)))
	rs := avgGain.Div(avgLoss)
	return decimal.NewFromInt(100).Sub(decimal.NewFromInt(100).Div(decimal.NewFromInt(1).Add(rs)))
}

func calculateVolatility(prices []decimal.Decimal, period int) decimal.Decimal {
	if len(prices) < period {
		return decimal.Zero
	}
	mean := calculateSMA(prices)
	sum := decimal.Zero
	for _, p := range prices {
		sum = sum.Add(p.Sub(mean).Pow(decimal.NewFromInt(2)))
	}
	variance := sum.Div(decimal.NewFromInt(int64(len(prices))))
	return sqrt(variance)
}

func sqrt(d decimal.Decimal) decimal.Decimal {
	guess, _ := decimal.NewFromString("1")
	for i := 0; i < 10; i++ {
		guess = guess.Add(d.Div(guess)).Div(decimal.NewFromInt(2))
	}
	return guess
}

type TradeTracker struct {
	mutex                sync.RWMutex
	inTrade              bool
	IsClosed             bool
	entryPrice           decimal.Decimal
	takeProfit           decimal.Decimal
	stopLoss             decimal.Decimal
	Side                 domain.Signal
	lastTradeAttempt     time.Time
	minProfitBuffer      decimal.Decimal
	ExitReason           string
	cooldown             time.Duration
	takeProfitPct        float64
	stopLossPct          float64
	confirmationCount    int
	confirmationSignal   domain.Signal
	confirmationCounter  int
	minProfitForFlipExit float64
}

func NewTradeTracker(takeProfitPct, stopLossPct float64, confirmationCount int, minProfitForFlipExit float64) *TradeTracker {
	return &TradeTracker{
		minProfitBuffer:      decimal.NewFromFloat(0.0015),
		cooldown:             5 * time.Minute,
		takeProfitPct:        takeProfitPct,
		stopLossPct:          stopLossPct,
		confirmationCount:    confirmationCount,
		minProfitForFlipExit: minProfitForFlipExit,
	}
}

func (t *TradeTracker) ConfirmSignal(signal domain.Signal) bool {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	if signal == t.confirmationSignal {
		t.confirmationCounter++
	} else {
		t.confirmationSignal = signal
		t.confirmationCounter = 1
	}
	return t.confirmationCounter >= t.confirmationCount
}

func (t *TradeTracker) GetEntryPrice() decimal.Decimal {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	return t.entryPrice
}

func (t *TradeTracker) GetSide() domain.Signal {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	return t.Side
}

func (t *TradeTracker) StartTrade(entryPrice decimal.Decimal, side domain.Signal) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.inTrade = true
	t.IsClosed = false
	t.entryPrice = entryPrice
	t.Side = side
	if side == domain.SignalBuy {
		t.takeProfit = entryPrice.Mul(decimal.NewFromFloat(1).Add(decimal.NewFromFloat(t.takeProfitPct)))
		t.stopLoss = entryPrice.Mul(decimal.NewFromFloat(1).Sub(decimal.NewFromFloat(t.stopLossPct)))
	} else {
		t.takeProfit = entryPrice.Mul(decimal.NewFromFloat(1).Sub(decimal.NewFromFloat(t.takeProfitPct)))
		t.stopLoss = entryPrice.Mul(decimal.NewFromFloat(1).Add(decimal.NewFromFloat(t.stopLossPct)))
	}
}

func (t *TradeTracker) CanTrade() bool {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	return time.Since(t.lastTradeAttempt) > t.cooldown
}

func (t *TradeTracker) SetLastTradeAttempt() {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.lastTradeAttempt = time.Now()
}

func (t *TradeTracker) EndTrade() {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.inTrade = false
	t.IsClosed = true
	t.lastTradeAttempt = time.Now()
	t.confirmationCounter = 0
	t.confirmationSignal = domain.SignalHold
}

func (t *TradeTracker) InTrade() bool {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	return t.inTrade
}

func (t *TradeTracker) ShouldExit(currentPrice decimal.Decimal, exitReason string) bool {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	if !t.inTrade {
		return false
	}
	if exitReason == "INDICATOR_FLIP" {
		profit := currentPrice.Sub(t.entryPrice)
		if t.Side == domain.SignalSell {
			profit = t.entryPrice.Sub(currentPrice)
		}
		pnlPct := profit.Div(t.entryPrice)
		return pnlPct.GreaterThanOrEqual(decimal.NewFromFloat(t.minProfitForFlipExit))
	}
	return true
}

type BinanceTrader struct {
	config           *config.Config
	tracker          *TradeTracker
	pm               *PositionManager
	emaStrategy      Strategy
	client           *marketdata.Client
	counter          *logging.TradeCounter
	notifier         notifications.Notifier
	repo             database.Repository
	startTime        time.Time
	sessionNumber    int
	lastPrice        decimal.Decimal
	availableBalance decimal.Decimal
	currentTrade     *logging.TradeLog
	candles          []domain.Candle
	tradeResults     []analytics.TradeResult
	shutdownStatus   string

	mutex   sync.RWMutex
	Running bool
	cancel  context.CancelFunc
}

func (e *BinanceTrader) GetStatus() domain.Status {
	return domain.Status{
		IsRunning: e.IsRunning(),
		Uptime:    time.Since(e.startTime).String(),
		Balance:   e.availableBalance.InexactFloat64(),
	}
}

func (e *BinanceTrader) GetTradeResults() []analytics.TradeResult {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	result := make([]analytics.TradeResult, len(e.tradeResults))
	copy(result, e.tradeResults)
	return result
}

func (e *BinanceTrader) GetShutdownStatus() string {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	return e.shutdownStatus
}

func (e *BinanceTrader) IsRunning() bool {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	return e.Running
}

func (e *BinanceTrader) Start(ctx context.Context) {
	ctx, e.cancel = context.WithCancel(ctx)
	e.Run(ctx)
}

func (e *BinanceTrader) Stop() {
	if e.cancel != nil {
		e.cancel()
	}
}

func (e *BinanceTrader) Run(ctx context.Context) {
	e.mutex.Lock()
	e.Running = true
	e.mutex.Unlock()

	defer func() {
		e.mutex.Lock()
		e.Running = false
		e.mutex.Unlock()
	}()

	sessionTimeout := time.Duration(e.config.SessionDurationMin) * time.Minute
	sessionTimer := time.NewTimer(sessionTimeout)
	defer sessionTimer.Stop()

	priceCh := make(chan marketdata.PriceUpdate)
	go e.client.Start(ctx, e.config.Symbol, priceCh)

	dashboardTicker := time.NewTicker(10 * time.Second)
	defer dashboardTicker.Stop()

	for {
		select {
		case priceUpdate := <-priceCh:
			price, _ := decimal.NewFromString(priceUpdate.Price)
			e.lastPrice = price

			newCandle := domain.Candle{Open: price, High: price, Low: price, Close: price}
			e.candles = append(e.candles, newCandle)

			if len(e.candles) < e.config.EMASlow {
				log.Printf("Collecting price points, %d/%d...", len(e.candles), e.config.EMASlow)
				continue
			}

			signal, rsi := e.emaStrategy.Calculate(e.candles)

			if e.tracker.InTrade() {
				if exit, reason := e.pm.Evaluate(e.lastPrice, rsi); exit {
					log.Printf("EXIT TRIGGER: %s reached", reason)
					quantity, err := e.pm.CalculatePositionSize(e.lastPrice, e.availableBalance)
					if err != nil {
						log.Printf("Failed to calculate position size for exit: %v", err)
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

		case <-dashboardTicker.C:
			e.updateDashboard(ctx)

		case <-sessionTimer.C:
			log.Printf("Session timeout reached (%d min). Shutting down.", e.config.SessionDurationMin)
			go e.notifier.Notify(fmt.Sprintf("Session timeout reached (%d min). Shutting down.", e.config.SessionDurationMin))
			e.hardShutdown("SESSION_TIMEOUT")
			return

		case <-ctx.Done():
			return
		}
	}
}

func (e *BinanceTrader) executeBuy(ctx context.Context) {
	if !e.tracker.InTrade() && e.tracker.CanTrade() {
		e.tracker.SetLastTradeAttempt()
		log.Print("Buy signal")
		quantity, err := e.pm.CalculatePositionSize(e.lastPrice, e.availableBalance)
		if err != nil {
			log.Printf("Failed to calculate position size: %v", err)
		} else {
			e.openPosition(ctx, futures.SideTypeBuy, quantity)
		}
	}
}

func (e *BinanceTrader) executeSell(ctx context.Context) {
	if !e.tracker.InTrade() && e.tracker.CanTrade() {
		e.tracker.SetLastTradeAttempt()
		log.Print("Sell signal")
		quantity, err := e.pm.CalculatePositionSize(e.lastPrice, e.availableBalance)
		if err != nil {
			log.Printf("Failed to calculate position size: %v", err)
		} else {
			e.openPosition(ctx, futures.SideTypeSell, quantity)
		}
	}
}

func (e *BinanceTrader) openPosition(ctx context.Context, side futures.SideType, quantity decimal.Decimal) {
	formattedQuantity := quantity.StringFixed(2)
	order, err := e.client.PlaceOrder(ctx, e.config.Symbol, side, formattedQuantity)
	if err != nil {
		if apiErr, ok := err.(*common.APIError); ok && apiErr.Code == -2019 {
			e.hardShutdown("MARGIN_CALL")
		} else {
			log.Printf("Failed to place order: %v", err)
		}
		return
	}

	log.Printf("Placed order: %+v", order)
	go e.notifier.Notify(fmt.Sprintf("Position opened: %s %s @ %s", side, quantity.StringFixed(2), e.lastPrice.StringFixed(2)))

	entryPrice := decimal.Zero
	time.Sleep(1 * time.Second)
	trades, err := e.client.GetAccountTradeList(ctx, e.config.Symbol, order.OrderID)
	if err != nil {
		log.Printf("Failed to get account trade list for entry price: %v", err)
	} else if len(trades) > 0 {
		entryPrice, _ = decimal.NewFromString(trades[0].Price)
	}

	if entryPrice.IsZero() {
		time.Sleep(2 * time.Second)
		trades, err = e.client.GetAccountTradeList(ctx, e.config.Symbol, order.OrderID)
		if err != nil {
			log.Printf("Failed to get account trade list for entry price on retry: %v", err)
		} else if len(trades) > 0 {
			entryPrice, _ = decimal.NewFromString(trades[0].Price)
		}
	}

	if entryPrice.IsZero() {
		log.Print("Could not determine entry price, aborting trade")
		return
	}

	log.Printf("Execution Verified: Entry @ %s", entryPrice.StringFixed(2))

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

func (e *BinanceTrader) closePosition(ctx context.Context, side futures.SideType, quantity decimal.Decimal, exitReason string) {
	formattedQuantity := quantity.StringFixed(2)
	order, err := e.client.PlaceOrder(ctx, e.config.Symbol, side, formattedQuantity)
	if err != nil {
		if apiErr, ok := err.(*common.APIError); ok && apiErr.Code == -2019 {
			e.hardShutdown("MARGIN_CALL")
		} else {
			log.Printf("Failed to place order: %v", err)
		}
		return
	}

	log.Printf("Placed order: %+v", order)
	go e.notifier.Notify(fmt.Sprintf("Position closed: %s %s @ %s. Reason: %s", side, quantity.StringFixed(2), e.lastPrice.StringFixed(2), exitReason))
	time.Sleep(1500 * time.Millisecond)

	trades, err := e.client.GetAccountTradeList(ctx, e.config.Symbol, order.OrderID)
	if err != nil {
		log.Printf("Failed to get account trade list for exit price: %v", err)
		return
	}

	if len(trades) == 0 {
		log.Print("Could not determine exit price, trade will not be logged")
		return
	}

	exitPrice, _ := decimal.NewFromString(trades[0].Price)
	log.Printf("Execution Verified: Exit @ %s", exitPrice.StringFixed(2))

	quantity, _ = decimal.NewFromString(order.OrigQuantity)
	var pnl decimal.Decimal
	if e.currentTrade.Side == "Buy" || e.currentTrade.Side == "BUY" {
		pnl = exitPrice.Sub(e.currentTrade.Entry).Mul(quantity)
	} else {
		pnl = e.currentTrade.Entry.Sub(exitPrice).Mul(quantity)
	}

	if pnl.Abs().GreaterThan(decimal.NewFromFloat(e.config.SessionBudget)) {
		log.Printf("Logic Error: PNL exceeds session budget. PNL: %s", pnl.StringFixed(2))
	} else if pnl.LessThan(decimal.NewFromFloat(-1.5)) {
		log.Printf("Panic Protocol Triggered! Loss > 1.5 USDT. PNL: %s", pnl.StringFixed(2))
		e.hardShutdown("PANIC")
	}

	e.currentTrade.Exit = exitPrice
	e.currentTrade.PnlUSDT = pnl
	e.currentTrade.ExitReason = exitReason
	e.counter.Record(*e.currentTrade)

	dbTrade := &domain.Trade{
		Symbol:     e.config.Symbol,
		Side:       e.currentTrade.Side,
		EntryPrice: e.currentTrade.Entry,
		ExitPrice:  e.currentTrade.Exit,
		Profit:     e.currentTrade.PnlUSDT,
		ExitReason: e.currentTrade.ExitReason,
	}
	e.repo.SaveTrade(dbTrade)

	e.repo.SaveTradeLog(&database.TradeLog{
		Mode:       "testnet",
		Symbol:     e.config.Symbol,
		Side:       e.currentTrade.Side,
		Entry:      e.currentTrade.Entry.InexactFloat64(),
		Exit:       exitPrice.InexactFloat64(),
		PnlUSDT:   pnl.InexactFloat64(),
		ExitReason: exitReason,
	})

	e.mutex.Lock()
	e.tradeResults = append(e.tradeResults, analytics.TradeResult{
		PnL:  pnl,
		Side: e.currentTrade.Side,
	})
	e.mutex.Unlock()

	e.tracker.EndTrade()
	e.currentTrade = nil
}

func (e *BinanceTrader) updateDashboard(ctx context.Context) {
	balance, err := e.client.VerifyCredentialsAndGetBalance(ctx)
	if err != nil {
		log.Printf("Failed to get account in dashboard: %v", err)
	} else {
		availableBalance, err := decimal.NewFromString(balance.AvailableBalance)
		if err != nil {
			log.Printf("Failed to parse available balance: %v", err)
		} else {
			if availableBalance.LessThan(decimal.NewFromInt(2)) {
				e.hardShutdown("LIQUIDATED_OR_EMPTY")
			}
			e.availableBalance = availableBalance
		}
	}
	e.printDashboard()
}

func (e *BinanceTrader) hardShutdown(status string) {
	log.Printf("CRITICAL: Shutting down. Status: %s", status)
	e.notifier.Notify(fmt.Sprintf("CRITICAL: Shutting down. Status: %s", status))

	_ = e.client.RestClient.CancelAllOpenOrders(context.Background(), e.config.Symbol)

	e.mutex.Lock()
	e.shutdownStatus = status
	e.mutex.Unlock()

	if e.cancel != nil {
		e.cancel()
	}
}

func (e *BinanceTrader) printDashboard() {
	log.Println("--------------------------------------------------")
	log.Printf("Time: %s | Uptime: %s", time.Now().Format("15:04:05"), time.Since(e.startTime).Round(time.Second))
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
	log.Printf("[SESSION] Trades: %d | Wins: %d | Losses: %d | Win Rate: %.2f%% | PnL: %s USDT",
		e.counter.GetTotalTrades(),
		e.counter.GetWins(),
		e.counter.GetLosses(),
		e.counter.GetWinRate(),
		e.counter.GetTotalProfit().StringFixed(2),
	)
	log.Println("--------------------------------------------------")
}

func NewBinanceTrader(cfg *config.Config, notifier notifications.Notifier, repo database.Repository, startTime time.Time, sessionNumber int, availableBalance decimal.Decimal, apiKey, apiSecret string) (*BinanceTrader, error) {
	client, err := marketdata.New(cfg, apiKey, apiSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to create exchange client: %w", err)
	}

	pm := NewPositionManager(
		decimal.NewFromFloat(cfg.SessionBudget),
		decimal.NewFromFloat(0.1),
		cfg.Leverage,
		cfg.SessionBudget,
		client.RestClient,
		cfg.TakeProfitPct,
		cfg.StopLossPct,
		cfg.BreakEvenTriggerPct,
		cfg.TrailDistancePct,
	)

	tracker := NewTradeTracker(cfg.TakeProfitPct, cfg.StopLossPct, cfg.ConfirmationCount, cfg.MinProfitForFlipExit)
	emaStrategy := NewEMACrossover(cfg.EMAFast, cfg.EMASlow, 5*time.Minute, tracker)

	return &BinanceTrader{
		config:           cfg,
		tracker:          tracker,
		pm:               pm,
		emaStrategy:      emaStrategy,
		client:           client,
		counter:          logging.NewTradeCounter(),
		notifier:         notifier,
		repo:             repo,
		startTime:        startTime,
		sessionNumber:    sessionNumber,
		availableBalance: availableBalance,
	}, nil
}

type PaperTrader struct {
	cfg             *config.Config
	counter         *logging.TradeCounter
	positionManager *PositionManager
	strategy        Strategy
	balance         decimal.Decimal
	currentPosition *domain.Position
	candles         []domain.Candle
	repo            database.Repository
	startTime       time.Time
	cancel          context.CancelFunc
	tradeResults    []analytics.TradeResult
	mu              sync.RWMutex
}

func (pt *PaperTrader) GetTradeResults() []analytics.TradeResult {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	result := make([]analytics.TradeResult, len(pt.tradeResults))
	copy(result, pt.tradeResults)
	return result
}

func (pt *PaperTrader) GetStatus() domain.Status {
	return domain.Status{
		IsRunning: true,
		Uptime:    time.Since(pt.startTime).String(),
		Balance:   pt.balance.InexactFloat64(),
	}
}

func (pt *PaperTrader) Start(ctx context.Context) {
	pt.startTime = time.Now()
	ctx, pt.cancel = context.WithCancel(ctx)
	pt.Run(ctx)
}

func (pt *PaperTrader) Stop() {
	if pt.cancel != nil {
		pt.cancel()
	}
}

func (pt *PaperTrader) Run(ctx context.Context) {
	log.Println("Starting paper trader...")

	sessionTimeout := time.Duration(pt.cfg.SessionDurationMin) * time.Minute
	sessionTimer := time.NewTimer(sessionTimeout)
	defer sessionTimer.Stop()

	priceCh := make(chan marketdata.PriceUpdate)
	client, err := marketdata.New(pt.cfg, "", "")
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

					pt.counter.Record(logging.TradeLog{
						Entry:      pt.currentPosition.Price,
						Exit:       price,
						Side:       pt.currentPosition.Side,
						PnlUSDT:    profit,
						ExitReason: reason,
					})

					pt.repo.SaveTradeLog(&database.TradeLog{
						Mode:       "paper",
						Symbol:     pt.cfg.Symbol,
						Side:       pt.currentPosition.Side,
						Entry:      pt.currentPosition.Price.InexactFloat64(),
						Exit:       price.InexactFloat64(),
						PnlUSDT:   profit.InexactFloat64(),
						ExitReason: reason,
					})

					dbTrade := &domain.Trade{
						Symbol:     pt.cfg.Symbol,
						Side:       pt.currentPosition.Side,
						EntryPrice: pt.currentPosition.Price,
						ExitPrice:  price,
						Profit:     profit,
						ExitReason: reason,
					}
					pt.repo.SaveTrade(dbTrade)
					pt.repo.SavePaperBalance(pt.balance)

					pt.mu.Lock()
					pt.tradeResults = append(pt.tradeResults, analytics.TradeResult{
						PnL:  profit,
						Side: pt.currentPosition.Side,
					})
					pt.mu.Unlock()

					log.Printf("[PAPER] Closed at %s | PnL: %s | Reason: %s", price, profit, reason)
					pt.currentPosition = nil
				}
			}

		case <-sessionTimer.C:
			log.Printf("Session timeout reached (%d min). Paper trader stopping.", pt.cfg.SessionDurationMin)
			return

		case <-ctx.Done():
			return
		}
	}
}

func NewPaperTrader(cfg *config.Config, repo database.Repository) (*PaperTrader, error) {
	pm := NewBacktestPositionManager(cfg)
	tradeTracker := NewTradeTracker(cfg.TakeProfitPct, cfg.StopLossPct, cfg.ConfirmationCount, cfg.MinProfitForFlipExit)
	strategy := NewEMACrossover(cfg.EMAFast, cfg.EMASlow, 0, tradeTracker)

	balance, err := repo.GetPaperBalance()
	if err != nil {
		balance = decimal.NewFromFloat(1000.0)
	}

	return &PaperTrader{
		cfg:             cfg,
		counter:         logging.NewTradeCounter(),
		positionManager: pm,
		strategy:        strategy,
		balance:         balance,
		repo:            repo,
	}, nil
}
