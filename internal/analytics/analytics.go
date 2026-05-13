package analytics

import (
	"math"

	"github.com/shopspring/decimal"
)

type TradeResult struct {
	PnL      decimal.Decimal
	Side     string
	Duration int
}

type Report struct {
	TotalTrades  int
	Wins         int
	Losses       int
	WinRate      float64
	ProfitFactor float64
	Expectancy   float64
	MaxDrawdown  float64
	SharpeRatio  float64
	NetPnL       float64
	GrossProfit  float64
	GrossLoss    float64
}

func Analyze(trades []TradeResult) Report {
	if len(trades) == 0 {
		return Report{}
	}

	var wins, losses int
	var grossProfit, grossLoss float64
	pnls := make([]float64, len(trades))

	for i, t := range trades {
		pnl, _ := t.PnL.Float64()
		pnls[i] = pnl
		if pnl > 0 {
			wins++
			grossProfit += pnl
		} else {
			losses++
			grossLoss += math.Abs(pnl)
		}
	}

	total := len(trades)
	winRate := float64(wins) / float64(total) * 100

	var profitFactor float64
	if grossLoss > 0 {
		profitFactor = grossProfit / grossLoss
	}

	netPnL := grossProfit - grossLoss
	expectancy := netPnL / float64(total)
	maxDD := maxDrawdown(pnls)
	sharpe := sharpeRatio(pnls)

	return Report{
		TotalTrades:  total,
		Wins:         wins,
		Losses:       losses,
		WinRate:      winRate,
		ProfitFactor: profitFactor,
		Expectancy:   expectancy,
		MaxDrawdown:  maxDD,
		SharpeRatio:  sharpe,
		NetPnL:       netPnL,
		GrossProfit:  grossProfit,
		GrossLoss:    grossLoss,
	}
}

func maxDrawdown(pnls []float64) float64 {
	if len(pnls) == 0 {
		return 0
	}

	var cumulative float64
	peak := 0.0
	maxDD := 0.0

	for _, pnl := range pnls {
		cumulative += pnl
		if cumulative > peak {
			peak = cumulative
		}
		dd := peak - cumulative
		if dd > maxDD {
			maxDD = dd
		}
	}

	return maxDD
}

func sharpeRatio(pnls []float64) float64 {
	if len(pnls) < 2 {
		return 0
	}

	var sum float64
	for _, p := range pnls {
		sum += p
	}
	mean := sum / float64(len(pnls))

	var variance float64
	for _, p := range pnls {
		diff := p - mean
		variance += diff * diff
	}
	variance /= float64(len(pnls) - 1)
	stdDev := math.Sqrt(variance)

	if stdDev == 0 {
		return 0
	}

	return mean / stdDev
}
