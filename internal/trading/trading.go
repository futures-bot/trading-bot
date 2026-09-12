package trading

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"trading-bot/internal/config"
	"trading-bot/internal/events"
	"trading-bot/internal/marketdata"
	"trading-bot/internal/notifications"
	"trading-bot/internal/risk"
	"trading-bot/internal/trading/domain"
	"trading-bot/shared/eventdef"

	"github.com/adshao/go-binance/v2/common"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/shopspring/decimal"
)

var ErrBudgetExhausted = errors.New("budget exhausted")

var ErrInsufficientFunds = errors.New("insufficient funds")

type Trader interface {
	Start(ctx context.Context)
	Stop()
	GetStatus() domain.Status
	GetTradeLogs() []*domain.TradeLog
}

// PositionManager is responsible for managing the current position.
type PositionManager struct {
	mutex           sync.RWMutex
	balance         decimal.Decimal
	currentPosition *domain.Position
	riskEngine      *risk.Engine
}

func NewPositionManager(balance decimal.Decimal, maxRiskPerTrade decimal.Decimal, leverage int, sessionBudget float64, restClient *marketdata.RestClient, cfg *config.Config, sharedBudget *risk.SharedBudget) *PositionManager {
	engine := risk.NewEngine(cfg, restClient.StepSize)
	engine.SharedBudget = sharedBudget
	return &PositionManager{
		balance:    balance,
		riskEngine: engine,
	}
}

func NewBacktestPositionManager(cfg *config.Config, sharedBudget *risk.SharedBudget) *PositionManager {
	engine := risk.NewEngine(cfg, decimal.NewFromFloat(0.01))
	engine.SharedBudget = sharedBudget
	return &PositionManager{
		balance:    decimal.NewFromFloat(cfg.PaperBalance),
		riskEngine: engine,
	}
}

func (pm *PositionManager) ReleaseBudget(p *domain.Position) {
	if pm.riskEngine.SharedBudget != nil && p != nil {
		leverage := pm.riskEngine.Leverage
		if leverage.IsZero() {
			leverage = decimal.NewFromInt(1)
		}
		cost := p.Quantity.Mul(p.Price).Div(leverage)
		pm.riskEngine.SharedBudget.Release(cost)
		log.Printf("[RISK] Released %s USDT back to shared budget", cost.StringFixed(2))
	}
}

func (pm *PositionManager) ReleaseMargin(amount decimal.Decimal) {
	if pm.riskEngine.SharedBudget != nil && !amount.IsZero() {
		pm.riskEngine.SharedBudget.Release(amount)
		log.Printf("[RISK] Released failed order margin %s USDT back to shared budget", amount.StringFixed(2))
	}
}

func (pm *PositionManager) CalculatePositionSize(price decimal.Decimal, availableBalance decimal.Decimal, trendStrength decimal.Decimal) (decimal.Decimal, error) {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	return pm.riskEngine.CalculatePositionSize(price, availableBalance, trendStrength)
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
	return pm.riskEngine.TakeProfitPct
}

func (pm *PositionManager) GetStopLossPct() decimal.Decimal {
	return pm.riskEngine.StopLossPct
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
	return pm.riskEngine.EvaluateExit(pm.currentPosition, currentPrice, rsi)
}

// Strategy defines the interface for a trading strategy.
type Strategy interface {
	Calculate([]domain.Candle) (domain.Signal, decimal.Decimal)
	UpdateLastTradeTime()
	TrendStrength() decimal.Decimal
}

// EMACrossover is a trading strategy based on the EMA crossover.
type EMACrossover struct {
	Strategy
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

func (s *EMACrossover) TrendStrength() decimal.Decimal {
	if s.slowEMA.IsZero() {
		return decimal.Zero
	}
	return s.emadiff.Abs().Div(s.slowEMA)
}

func (s *EMACrossover) GetRSI() decimal.Decimal        { return s.rsi }
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

	log.Printf("fastEMA: %s, slowEMA: %s, rsi: %s", s.fastEMA, s.slowEMA, s.rsi)

	s.ticks++
	if s.ticks%500 == 0 {
		log.Printf("RSI: %s | Price: %s | EMA: %s", s.rsi, s.prices[len(s.prices)-1], s.slowEMA)
	}

	if s.fastEMA.IsZero() || s.slowEMA.IsZero() {
		return domain.SignalHold, s.rsi
	}

	if s.fastEMA.GreaterThan(s.slowEMA) {
		return domain.SignalBuy, s.rsi
	} else if s.fastEMA.LessThan(s.slowEMA) {
		return domain.SignalSell, s.rsi
	}

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

// TradeTracker is responsible for tracking the state of a trade.
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

// BinanceTrader is a trader that connects to the Binance API.
type BinanceTrader struct {
	symbol      string
	config      *config.Config
	tracker          *TradeTracker
	pm               *PositionManager
	emaStrategy      Strategy
	client           *marketdata.Client
	notifier         notifications.Notifier
	publisher        events.Publisher
	startTime        time.Time
	sessionNumber    int
	lastPrice        decimal.Decimal
	availableBalance decimal.Decimal
	candles          []domain.Candle
	shutdownStatus   string
	tradeLogs        []*domain.TradeLog

	mutex   sync.RWMutex
	Running bool
	cancel  context.CancelFunc
}

func (e *BinanceTrader) GetTradeLogs() []*domain.TradeLog {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	return e.tradeLogs
}

func (e *BinanceTrader) GetStatus() domain.Status {
	return domain.Status{
		IsRunning: e.IsRunning(),
		Uptime:    time.Since(e.startTime).String(),
		Balance:   e.availableBalance.InexactFloat64(),
	}
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
	go e.client.Start(ctx, e.symbol, priceCh)

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
					quantity, err := e.pm.CalculatePositionSize(e.lastPrice, e.availableBalance, decimal.Zero)
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
		quantity, err := e.pm.CalculatePositionSize(e.lastPrice, e.availableBalance, e.emaStrategy.TrendStrength())
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
		quantity, err := e.pm.CalculatePositionSize(e.lastPrice, e.availableBalance, e.emaStrategy.TrendStrength())
		if err != nil {
			log.Printf("Failed to calculate position size: %v", err)
		} else {
			e.openPosition(ctx, futures.SideTypeSell, quantity)
		}
	}
}

func (e *BinanceTrader) openPosition(ctx context.Context, side futures.SideType, quantity decimal.Decimal) {
	formattedQuantity := quantity.StringFixed(e.client.QuantityPrecision)
	order, err := e.client.PlaceOrder(ctx, e.symbol, side, formattedQuantity)
	if err != nil {
		// Release failed order budget back to shared budget
		leverage := e.pm.riskEngine.Leverage
		if leverage.IsZero() {
			leverage = decimal.NewFromInt(1)
		}
		marginCost := quantity.Mul(e.lastPrice).Div(leverage)
		e.pm.ReleaseMargin(marginCost)

		if apiErr, ok := err.(*common.APIError); ok && apiErr.Code == -2019 {
			e.hardShutdown("MARGIN_CALL")
		} else {
			log.Printf("Failed to place order: %v", err)
		}
		return
	}

	log.Printf("Placed order: %+v", order)
	go e.notifier.Notify(fmt.Sprintf("Position opened: %s %s @ %s", side, quantity.StringFixed(e.client.QuantityPrecision), e.lastPrice.StringFixed(e.client.PricePrecision)))

	entryPrice := decimal.Zero
	time.Sleep(1 * time.Second)
	trades, err := e.client.GetAccountTradeList(ctx, e.symbol, order.OrderID)
	if err != nil {
		log.Printf("Failed to get account trade list for entry price: %v", err)
	} else if len(trades) > 0 {
		entryPrice, _ = decimal.NewFromString(trades[0].Price)
	}

	if entryPrice.IsZero() {
		time.Sleep(2 * time.Second)
		trades, err = e.client.GetAccountTradeList(ctx, e.symbol, order.OrderID)
		if err != nil {
			log.Printf("Failed to get account trade list for entry price on retry: %v", err)
		} else if len(trades) > 0 {
			entryPrice, _ = decimal.NewFromString(trades[0].Price)
		}
	}

	if entryPrice.IsZero() {
		log.Print("Could not determine entry price, aborting trade")
		// Release allocated margin cost back and reset tracker state
		leverage := e.pm.riskEngine.Leverage
		if leverage.IsZero() {
			leverage = decimal.NewFromInt(1)
		}
		marginCost := quantity.Mul(e.lastPrice).Div(leverage)
		e.pm.ReleaseMargin(marginCost)
		e.tracker.EndTrade()
		return
	}

	log.Printf("Execution Verified: Entry @ %s", entryPrice.StringFixed(e.client.PricePrecision))

	var signalSide domain.Signal
	if side == futures.SideTypeBuy {
		signalSide = domain.SignalBuy
	} else {
		signalSide = domain.SignalSell
	}

	e.tracker.StartTrade(entryPrice, signalSide)

	// Set current position in PositionManager so that it can be correctly released at close
	position := &domain.Position{
		Symbol:          e.symbol,
		Side:            string(signalSide),
		Price:           entryPrice,
		Quantity:        quantity,
		StopLossPrice:   e.tracker.stopLoss,
		TakeProfitPrice: e.tracker.takeProfit,
	}
	e.pm.SetCurrentPosition(position)
}

func (e *BinanceTrader) closePosition(ctx context.Context, side futures.SideType, quantity decimal.Decimal, exitReason string) {
	formattedQuantity := quantity.StringFixed(e.client.QuantityPrecision)
	order, err := e.client.PlaceOrder(ctx, e.symbol, side, formattedQuantity)
	if err != nil {
		if apiErr, ok := err.(*common.APIError); ok && apiErr.Code == -2019 {
			e.hardShutdown("MARGIN_CALL")
		} else {
			log.Printf("Failed to place order: %v", err)
		}
		return
	}

	log.Printf("Placed order: %+v", order)
	go e.notifier.Notify(fmt.Sprintf("Position closed: %s %s @ %s. Reason: %s", side, quantity.StringFixed(e.client.QuantityPrecision), e.lastPrice.StringFixed(e.client.PricePrecision), exitReason))
	time.Sleep(1500 * time.Millisecond)

	trades, err := e.client.GetAccountTradeList(ctx, e.symbol, order.OrderID)
	if err != nil {
		log.Printf("Failed to get account trade list for exit price: %v", err)
		return
	}

	if len(trades) == 0 {
		log.Print("Could not determine exit price, trade will not be logged")
		return
	}

	exitPrice, _ := decimal.NewFromString(trades[0].Price)
	log.Printf("Execution Verified: Exit @ %s", exitPrice.StringFixed(e.client.PricePrecision))

	quantity, _ = decimal.NewFromString(order.OrigQuantity)
	var pnl decimal.Decimal
	if e.tracker.GetSide() == domain.SignalBuy {
		pnl = exitPrice.Sub(e.tracker.GetEntryPrice()).Mul(quantity)
	} else {
		pnl = e.tracker.GetEntryPrice().Sub(exitPrice).Mul(quantity)
	}

	if pnl.Abs().GreaterThan(decimal.NewFromFloat(e.config.SessionBudget)) {
		log.Printf("Logic Error: PNL exceeds session budget. PNL: %s", pnl.StringFixed(2))
	} else if pnl.LessThan(decimal.NewFromFloat(-1.5)) {
		log.Printf("Panic Protocol Triggered! Loss > 1.5 USDT. PNL: %s", pnl.StringFixed(2))
		e.hardShutdown("PANIC")
	}

	dbTrade := map[string]interface{}{
		"symbol":      e.symbol,
		"side":        string(e.tracker.Side),
		"entry_price": e.tracker.GetEntryPrice(),
		"exit_price":  exitPrice,
		"profit":      pnl,
		"exit_reason": exitReason,
	}
	e.publisher.Publish(context.Background(), "trades", eventdef.NewEvent("trade.closed", "binance-trader", 1, dbTrade))

	e.mutex.Lock()
	e.tradeLogs = append(e.tradeLogs, &domain.TradeLog{
		Mode:       "testnet",
		Symbol:     e.symbol,
		Side:       string(e.tracker.Side),
		Entry:      e.tracker.GetEntryPrice().InexactFloat64(),
		Exit:       exitPrice.InexactFloat64(),
		PnlUSDT:    pnl.InexactFloat64(),
		ExitReason: exitReason,
	})
	e.mutex.Unlock()

	e.pm.ReleaseBudget(e.pm.currentPosition)
	e.pm.SetCurrentPosition(nil)
	e.tracker.EndTrade()
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

	_ = e.client.RestClient.CancelAllOpenOrders(context.Background(), e.symbol)

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

	log.Println("--------------------------------------------------")
}

func NewBinanceTrader(cfg *config.Config, notifier notifications.Notifier, publisher events.Publisher, startTime time.Time, sessionNumber int, availableBalance decimal.Decimal, apiKey, apiSecret, symbol string, sharedBudget *risk.SharedBudget) (*BinanceTrader, error) {
	client, err := marketdata.New(cfg, apiKey, apiSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to create exchange client: %w", err)
	}

	// Retrieve exchange info to populate PricePrecision, QuantityPrecision, and StepSize
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.RestClient.GetExchangeInfo(ctx, symbol); err != nil {
		return nil, fmt.Errorf("failed to get exchange info for %s: %w", symbol, err)
	}

	pm := NewPositionManager(
		decimal.NewFromFloat(cfg.SessionBudget),
		decimal.NewFromFloat(0.1),
		cfg.Leverage,
		cfg.SessionBudget,
		client.RestClient,
		cfg,
		sharedBudget,
	)

	tracker := NewTradeTracker(cfg.TakeProfitPct, cfg.StopLossPct, cfg.ConfirmationCount, cfg.MinProfitForFlipExit)
	emaStrategy := NewEMACrossover(cfg.EMAFast, cfg.EMASlow, 5*time.Minute, tracker)

	return &BinanceTrader{
		symbol:      symbol,
		config:      cfg,
		tracker:     tracker,
		pm:          pm,
		emaStrategy: emaStrategy,
		client:      client,

		notifier:         notifier,
		publisher:        publisher,
		startTime:        startTime,
		sessionNumber:    sessionNumber,
		availableBalance: availableBalance,
	}, nil
}

// PaperTrader is a trader that simulates trades without connecting to an exchange.
type PaperTrader struct {
	symbol          string
	cfg             *config.Config
	positionManager *PositionManager
	strategy        Strategy
	balance         decimal.Decimal
	currentPosition *domain.Position
	candles         []domain.Candle
	publisher       events.Publisher
	startTime       time.Time
	cancel          context.CancelFunc
	tradeLogs       []*domain.TradeLog
	mu              sync.RWMutex
	// PriceUpdateCh can be injected in tests to mock live exchange feed.
	PriceUpdateCh chan marketdata.PriceUpdate
}

func (pt *PaperTrader) GetTradeLogs() []*domain.TradeLog {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	return pt.tradeLogs
}

// GetCurrentPosition safely returns the current position for testing or status checks.
func (pt *PaperTrader) GetCurrentPosition() *domain.Position {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	return pt.currentPosition
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

	var priceCh chan marketdata.PriceUpdate
	if pt.PriceUpdateCh != nil {
		priceCh = pt.PriceUpdateCh
	} else {
		priceCh = make(chan marketdata.PriceUpdate)
		client, err := marketdata.New(pt.cfg, "", "")
		if err != nil {
			log.Fatalf("Failed to create exchange client: %v", err)
		}
		go client.Start(ctx, pt.symbol, priceCh)
	}

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

			pt.mu.Lock()
			if pt.currentPosition == nil {
				if signal == domain.SignalBuy || signal == domain.SignalSell {
					qty, _ := pt.positionManager.CalculatePositionSize(price, pt.balance, pt.strategy.TrendStrength())
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
						Symbol:          pt.symbol,
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

					dbTrade := map[string]interface{}{
						"symbol":      pt.symbol,
						"side":        pt.currentPosition.Side,
						"entry_price": pt.currentPosition.Price,
						"exit_price":  price,
						"profit":      profit,
						"exit_reason": reason,
					}
					pt.publisher.Publish(context.Background(), "trades", eventdef.NewEvent("trade.closed", "paper-trader", 1, dbTrade))

					pt.tradeLogs = append(pt.tradeLogs, &domain.TradeLog{
						Mode:       "paper",
						Symbol:     pt.symbol,
						Side:       pt.currentPosition.Side,
						Entry:      pt.currentPosition.Price.InexactFloat64(),
						Exit:       price.InexactFloat64(),
						PnlUSDT:    profit.InexactFloat64(),
						ExitReason: reason,
					})

					log.Printf("[PAPER] Closed at %s | PnL: %s | Reason: %s", price, profit, reason)
					pt.positionManager.ReleaseBudget(pt.currentPosition)
					pt.currentPosition = nil
				}
			}
			pt.mu.Unlock()

		case <-sessionTimer.C:
			log.Printf("Session timeout reached (%d min). Paper trader stopping.", pt.cfg.SessionDurationMin)
			return

		case <-ctx.Done():
			return
		}
	}
}

func NewPaperTrader(cfg *config.Config, publisher events.Publisher, symbol string, sharedBudget *risk.SharedBudget) (*PaperTrader, error) {
	pm := NewBacktestPositionManager(cfg, sharedBudget)
	tradeTracker := NewTradeTracker(cfg.TakeProfitPct, cfg.StopLossPct, cfg.ConfirmationCount, cfg.MinProfitForFlipExit)
	strategy := NewEMACrossover(cfg.EMAFast, cfg.EMASlow, 0, tradeTracker)

	return &PaperTrader{
		symbol:          symbol,
		cfg:             cfg,
		positionManager: pm,
		strategy:        strategy,
		balance:         decimal.NewFromFloat(1000.0),
		publisher:       publisher,
	}, nil
}
