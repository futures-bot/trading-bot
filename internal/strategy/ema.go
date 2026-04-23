package strategy

import (
	"log"
	"time"

	"trading-bot/internal/domain"

	"github.com/shopspring/decimal"
)

// EMACrossover represents a strategy based on two EMA crossovers.
type EMACrossover struct {
	fastPeriod    int
	slowPeriod    int
	fastEMA       decimal.Decimal
	slowEMA       decimal.Decimal
	prices        []decimal.Decimal
	rsi           decimal.Decimal
	emadiff       decimal.Decimal
	volatility    decimal.Decimal
	lastTradeTime time.Time
	cooldown      time.Duration
	tradeTracker  *TradeTracker
	lastSignal    domain.Signal
	ticks         int
	previousRSI   decimal.Decimal
	priceHistory  []decimal.Decimal
}

// NewEMACrossover creates a new EMACrossover strategy.
func NewEMACrossover(fastPeriod, slowPeriod int, cooldown time.Duration, tradeTracker *TradeTracker) *EMACrossover {
	return &EMACrossover{
		fastPeriod:   fastPeriod,
		slowPeriod:   slowPeriod,
		cooldown:     cooldown,
		tradeTracker: tradeTracker,
		lastSignal:   domain.SignalHold,
	}
}

// UpdateLastTradeTime updates the last trade time to the current time.
func (s *EMACrossover) UpdateLastTradeTime() {
	s.lastTradeTime = time.Now()
}

func (s *EMACrossover) GetRSI() decimal.Decimal {
	return s.rsi
}

func (s *EMACrossover) GetEMAGap() decimal.Decimal {
	return s.emadiff
}

func (s *EMACrossover) GetVolatility() decimal.Decimal {
	return s.volatility
}

func (s *EMACrossover) GetSignal() domain.Signal {
	return s.lastSignal
}

// Calculate determines a trading signal based on the EMA crossover.
func (s *EMACrossover) Calculate(candles []domain.Candle) (domain.Signal, decimal.Decimal) {
	s.prices = []decimal.Decimal{}
	for _, c := range candles {
		s.prices = append(s.prices, c.Close)
	}

	// We need enough data to calculate the longest EMA.
	if len(s.prices) < s.slowPeriod {
		return domain.SignalHold, s.rsi
	}

	// Trim prices to the required length to avoid memory leaks.
	if len(s.prices) > s.slowPeriod {
		s.prices = s.prices[len(s.prices)-s.slowPeriod:]
	}

	// Calculate the EMAs.
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

	// Check for crossover.
	if s.fastEMA.GreaterThan(s.slowEMA) {

		s.lastSignal = domain.SignalBuy
	} else if s.fastEMA.LessThan(s.slowEMA) {
		s.lastSignal = domain.SignalSell
	}

	s.priceHistory = append(s.priceHistory, s.prices[len(s.prices)-1])
	if len(s.priceHistory) > 10 {
		s.priceHistory = s.priceHistory[1:]
	}

	// Volatility Gate
	if len(s.priceHistory) < 10 {
		return domain.SignalHold, s.rsi
	}
	price10TicksAgo := s.priceHistory[0]
	if s.prices[len(s.prices)-1].Sub(price10TicksAgo).Abs().LessThan(s.prices[len(s.prices)-1].Mul(decimal.NewFromFloat(0.0002))) {
		return domain.SignalHold, s.rsi
	}

	// RSI Cross Logic
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

// calculateEMA calculates the Exponential Moving Average.
func calculateEMA(prices []decimal.Decimal, period int) decimal.Decimal {
	if len(prices) < period {
		return decimal.Zero
	}

	// Start with a simple moving average for the first value.
	sma := calculateSMA(prices[:period])

	k := decimal.NewFromFloat(2.0).Div(decimal.NewFromInt(int64(period + 1)))

	ema := sma
	for i := period; i < len(prices); i++ {
		// EMA = (Close - EMA_prev) * k + EMA_prev
		ema = prices[i].Sub(ema).Mul(k).Add(ema)
	}

	return ema
}

// calculateSMA calculates the Simple Moving Average.
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

// calculateRSI calculates the Relative Strength Index.
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

	rsi := decimal.NewFromInt(100).Sub(decimal.NewFromInt(100).Div(decimal.NewFromInt(1).Add(rs)))

	return rsi
}

// calculateVolatility calculates the standard deviation of prices.
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
	// Babylonian method for square root
	// https://en.wikipedia.org/wiki/Methods_of_computing_square_roots#Babylonian_method
	// This is not a perfect implementation, but it is good enough for our purposes.

	// Use an initial guess.
	guess, _ := decimal.NewFromString("1")

	// Iterate until the guess is good enough.
	for i := 0; i < 10; i++ {
		// guess = (guess + d/guess) / 2
		guess = guess.Add(d.Div(guess)).Div(decimal.NewFromInt(2))
	}

	return guess
}
