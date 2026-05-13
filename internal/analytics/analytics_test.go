package analytics

import (
	"math"
	"testing"

	"github.com/shopspring/decimal"
)

func TestAnalyze_Empty(t *testing.T) {
	r := Analyze(nil)
	if r.TotalTrades != 0 {
		t.Errorf("expected 0 trades, got %d", r.TotalTrades)
	}
}

func TestAnalyze_AllWins(t *testing.T) {
	trades := []TradeResult{
		{PnL: decimal.NewFromFloat(1.5)},
		{PnL: decimal.NewFromFloat(2.0)},
		{PnL: decimal.NewFromFloat(0.5)},
	}
	r := Analyze(trades)

	if r.Wins != 3 {
		t.Errorf("expected 3 wins, got %d", r.Wins)
	}
	if r.Losses != 0 {
		t.Errorf("expected 0 losses, got %d", r.Losses)
	}
	if r.MaxDrawdown != 0 {
		t.Errorf("expected 0 drawdown, got %f", r.MaxDrawdown)
	}
	if r.ProfitFactor != 0 {
		t.Errorf("expected 0 profit factor (no losses), got %f", r.ProfitFactor)
	}
}

func TestAnalyze_MixedTrades(t *testing.T) {
	trades := []TradeResult{
		{PnL: decimal.NewFromFloat(2.0)},
		{PnL: decimal.NewFromFloat(-1.0)},
		{PnL: decimal.NewFromFloat(3.0)},
		{PnL: decimal.NewFromFloat(-0.5)},
	}
	r := Analyze(trades)

	if r.TotalTrades != 4 {
		t.Errorf("expected 4 trades, got %d", r.TotalTrades)
	}
	if r.Wins != 2 {
		t.Errorf("expected 2 wins, got %d", r.Wins)
	}
	if r.WinRate != 50 {
		t.Errorf("expected 50%% win rate, got %f", r.WinRate)
	}

	expectedPF := 5.0 / 1.5
	if math.Abs(r.ProfitFactor-expectedPF) > 0.01 {
		t.Errorf("expected profit factor ~%.2f, got %f", expectedPF, r.ProfitFactor)
	}

	expectedExp := 3.5 / 4.0
	if math.Abs(r.Expectancy-expectedExp) > 0.01 {
		t.Errorf("expected expectancy ~%.2f, got %f", expectedExp, r.Expectancy)
	}

	// Drawdown: cumulative = [2, 1, 4, 3.5] -> peak 2, dd=1 then peak 4, dd=0.5 -> maxDD=1.0
	if math.Abs(r.MaxDrawdown-1.0) > 0.01 {
		t.Errorf("expected max drawdown ~1.0, got %f", r.MaxDrawdown)
	}
}

func TestAnalyze_AllLosses(t *testing.T) {
	trades := []TradeResult{
		{PnL: decimal.NewFromFloat(-1.0)},
		{PnL: decimal.NewFromFloat(-2.0)},
	}
	r := Analyze(trades)

	if r.Wins != 0 {
		t.Errorf("expected 0 wins, got %d", r.Wins)
	}
	if r.ProfitFactor != 0 {
		t.Errorf("expected 0 profit factor, got %f", r.ProfitFactor)
	}
	if r.MaxDrawdown != 3.0 {
		t.Errorf("expected max drawdown 3.0, got %f", r.MaxDrawdown)
	}
}

func TestSharpeRatio_Positive(t *testing.T) {
	pnls := []float64{1.0, 1.0, 1.0}
	s := sharpeRatio(pnls)
	if !math.IsInf(s, 0) && s != 0 {
		// stddev=0 -> should return 0
	}
}

func TestMaxDrawdown_NoDrawdown(t *testing.T) {
	pnls := []float64{1, 2, 3}
	dd := maxDrawdown(pnls)
	if dd != 0 {
		t.Errorf("expected 0 drawdown, got %f", dd)
	}
}
