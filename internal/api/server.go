package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"trading-bot/internal/database"
	tradingv1 "trading-bot/internal/trading/domain"

	"github.com/shopspring/decimal"
)

type Server struct {
	repo   database.Repository
	addr   string
	server *http.Server
}

func NewServer(repo database.Repository, addr string) *Server {
	return &Server{
		repo: repo,
		addr: addr,
	}
}

func (s *Server) corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/health", s.corsMiddleware(s.handleHealth))
	mux.HandleFunc("/api/stats", s.corsMiddleware(s.handleStats))
	mux.HandleFunc("/api/trades", s.corsMiddleware(s.handleTrades))
	mux.HandleFunc("/api/sessions", s.corsMiddleware(s.handleSessions))
	mux.HandleFunc("/api/sessions/latest", s.corsMiddleware(s.handleLatestSession))
	mux.HandleFunc("/api/performance", s.corsMiddleware(s.handlePerformance))
	mux.HandleFunc("/api/stream", s.corsMiddleware(s.handleStream))

	s.server = &http.Server{
		Addr:    s.addr,
		Handler: mux,
	}

	log.Printf("[API] Starting analytics server on %s", s.addr)
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[API] Server error: %v", err)
		}
	}()

	return nil
}

func (s *Server) Stop() error {
	if s.server != nil {
		return s.server.Close()
	}
	return nil
}

type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(HealthResponse{
		Status:  "ok",
		Version: "4.0.0",
	})
}

type TradeResponse struct {
	ID         uint            `json:"id"`
	Symbol     string          `json:"symbol"`
	Side       string          `json:"side"`
	EntryPrice decimal.Decimal `json:"entry_price"`
	ExitPrice  decimal.Decimal `json:"exit_price"`
	Profit     decimal.Decimal `json:"profit"`
	ExitReason string          `json:"exit_reason"`
	CreatedAt  string          `json:"created_at"`
}

func (s *Server) handleTrades(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 500 {
			limit = parsed
		}
	}

	trades, err := s.repo.GetLastTrades(limit)
	if err != nil {
		http.Error(w, `{"error":"failed to fetch trades"}`, http.StatusInternalServerError)
		return
	}

	var resp []TradeResponse
	for _, t := range trades {
		resp = append(resp, TradeResponse{
			ID:         t.ID,
			Symbol:     t.Symbol,
			Side:       t.Side,
			EntryPrice: t.EntryPrice,
			ExitPrice:  t.ExitPrice,
			Profit:     t.Profit,
			ExitReason: t.ExitReason,
			CreatedAt:  t.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	json.NewEncoder(w).Encode(resp)
}

type SessionResponse struct {
	ID          uint    `json:"id"`
	Mode        string  `json:"mode"`
	Symbol      string  `json:"symbol"`
	DurationSec int     `json:"duration_secs"`
	TotalTrades int     `json:"total_trades"`
	Wins        int     `json:"wins"`
	Losses      int     `json:"losses"`
	NetPnL      float64 `json:"net_pnl"`
	FinalBalance float64 `json:"final_balance"`
	ProfitFactor float64 `json:"profit_factor"`
	MaxDrawdown float64 `json:"max_drawdown"`
	SharpeRatio float64 `json:"sharpe_ratio"`
	Expectancy  float64 `json:"expectancy"`
	Status      string  `json:"status"`
	CreatedAt   string  `json:"created_at"`
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 500 {
			limit = parsed
		}
	}

	sessions, err := s.repo.GetLastSessions(limit)
	if err != nil {
		http.Error(w, `{"error":"failed to fetch sessions"}`, http.StatusInternalServerError)
		return
	}

	var resp []SessionResponse
	for _, sess := range sessions {
		resp = append(resp, SessionResponse{
			ID:           sess.ID,
			Mode:         sess.Mode,
			Symbol:       sess.Symbol,
			DurationSec:  sess.DurationSecs,
			TotalTrades:  sess.TotalTrades,
			Wins:         sess.Wins,
			Losses:       sess.Losses,
			NetPnL:       sess.NetPnL,
			FinalBalance: sess.FinalBalance,
			ProfitFactor: sess.ProfitFactor,
			MaxDrawdown:  sess.MaxDrawdown,
			SharpeRatio:  sess.SharpeRatio,
			Expectancy:   sess.Expectancy,
			Status:       sess.Status,
			CreatedAt:    sess.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	json.NewEncoder(w).Encode(resp)
}

type LatestSessionResponse struct {
	Testnet SessionResponse `json:"testnet"`
	Paper   SessionResponse `json:"paper"`
	Backtest SessionResponse `json:"backtest"`
}

func (s *Server) handleLatestSession(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	sessions, err := s.repo.GetLastSessions(100)
	if err != nil {
		http.Error(w, `{"error":"failed to fetch sessions"}`, http.StatusInternalServerError)
		return
	}

	var testnetLatest, paperLatest, backtestLatest *database.Session

	for i := range sessions {
		if sessions[i].Mode == "testnet" && testnetLatest == nil {
			testnetLatest = &sessions[i]
		}
		if sessions[i].Mode == "paper" && paperLatest == nil {
			paperLatest = &sessions[i]
		}
		if sessions[i].Mode == "backtest" && backtestLatest == nil {
			backtestLatest = &sessions[i]
		}
	}

	toResp := func(s *database.Session) SessionResponse {
		if s == nil {
			return SessionResponse{}
		}
		return SessionResponse{
			ID:           s.ID,
			Mode:         s.Mode,
			Symbol:       s.Symbol,
			DurationSec:  s.DurationSecs,
			TotalTrades:  s.TotalTrades,
			Wins:         s.Wins,
			Losses:       s.Losses,
			NetPnL:       s.NetPnL,
			FinalBalance: s.FinalBalance,
			ProfitFactor: s.ProfitFactor,
			MaxDrawdown:  s.MaxDrawdown,
			SharpeRatio:  s.SharpeRatio,
			Expectancy:   s.Expectancy,
			Status:       s.Status,
			CreatedAt:    s.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	resp := LatestSessionResponse{
		Testnet: toResp(testnetLatest),
		Paper:   toResp(paperLatest),
		Backtest: toResp(backtestLatest),
	}

	json.NewEncoder(w).Encode(resp)
}

type StatsResponse struct {
	TotalTrades    int64   `json:"total_trades"`
	WinRate        float64 `json:"win_rate"`
	TotalPnL       float64 `json:"total_pnl"`
	PaperSessions  int     `json:"paper_sessions"`
	TestnetSessions int    `json:"testnet_sessions"`
	BacktestSessions int   `json:"backtest_sessions"`
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	stats, err := s.repo.GetPerformanceStats()
	if err != nil {
		http.Error(w, `{"error":"failed to fetch stats"}`, http.StatusInternalServerError)
		return
	}

	sessions, _ := s.repo.GetLastSessions(1000)
	paperCount := 0
	testnetCount := 0
	backtestCount := 0
	for _, sess := range sessions {
		switch sess.Mode {
		case "paper":
			paperCount++
		case "testnet":
			testnetCount++
		case "backtest":
			backtestCount++
		}
	}

	resp := StatsResponse{
		TotalTrades:      stats.TotalTrades,
		WinRate:          stats.WinRate,
		TotalPnL:         stats.TotalPnl,
		PaperSessions:    paperCount,
		TestnetSessions:  testnetCount,
		BacktestSessions: backtestCount,
	}

	json.NewEncoder(w).Encode(resp)
}

type PerformanceResponse struct {
	Mode              string  `json:"mode"`
	Symbol            string  `json:"symbol"`
	TotalTrades       int     `json:"total_trades"`
	Wins              int     `json:"wins"`
	WinRate           float64 `json:"win_rate"`
	Losses            int     `json:"losses"`
	NetPnL            float64 `json:"net_pnl"`
	AvgWin            float64 `json:"avg_win"`
	AvgLoss           float64 `json:"avg_loss"`
	ProfitFactor      float64 `json:"profit_factor"`
	Expectancy        float64 `json:"expectancy"`
	MaxDrawdown       float64 `json:"max_drawdown"`
	SharpeRatio       float64 `json:"sharpe_ratio"`
	BestTrade         float64 `json:"best_trade"`
	WorstTrade        float64 `json:"worst_trade"`
}

func (s *Server) handlePerformance(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	mode := strings.ToLower(r.URL.Query().Get("mode"))
	if mode == "" {
		mode = "all"
	}

	trades, err := s.repo.GetLastTrades(10000)
	if err != nil {
		http.Error(w, `{"error":"failed to fetch trades"}`, http.StatusInternalServerError)
		return
	}

	var filtered []tradingv1.Trade
	if mode != "all" {
		for _, t := range trades {
			if strings.HasPrefix(strings.ToLower(t.Symbol), strings.ToLower(mode)) {
				filtered = append(filtered, t)
			}
		}
	} else {
		filtered = trades
	}

	if len(filtered) == 0 {
		http.Error(w, `{"error":"no trades found"}`, http.StatusNotFound)
		return
	}

	var wins, losses int
	var totalProfit, totalLoss, bestTrade, worstTrade float64
	bestTrade = -9999
	worstTrade = 9999

	for _, t := range filtered {
		pnl, _ := t.Profit.Float64()
		if pnl > 0 {
			wins++
			totalProfit += pnl
		} else {
			losses++
			totalLoss += -pnl
		}
		if pnl > bestTrade {
			bestTrade = pnl
		}
		if pnl < worstTrade {
			worstTrade = pnl
		}
	}

	winRate := 0.0
	if len(filtered) > 0 {
		winRate = float64(wins) / float64(len(filtered)) * 100
	}

	profitFactor := 0.0
	if totalLoss > 0 {
		profitFactor = totalProfit / totalLoss
	}

	expectancy := (totalProfit - totalLoss) / float64(len(filtered))

	avgWin := 0.0
	if wins > 0 {
		avgWin = totalProfit / float64(wins)
	}

	avgLoss := 0.0
	if losses > 0 {
		avgLoss = totalLoss / float64(losses)
	}

	resp := PerformanceResponse{
		Mode:         mode,
		Symbol:       "",
		TotalTrades:  len(filtered),
		Wins:         wins,
		Losses:       losses,
		WinRate:      winRate,
		NetPnL:       totalProfit - totalLoss,
		AvgWin:       avgWin,
		AvgLoss:      avgLoss,
		ProfitFactor: profitFactor,
		Expectancy:   expectancy,
		BestTrade:    bestTrade,
		WorstTrade:   worstTrade,
	}

	json.NewEncoder(w).Encode(resp)
}

type StreamEvent struct {
	Type      string      `json:"type"`
	Timestamp string      `json:"timestamp"`
	Data      interface{} `json:"data"`
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	stats, _ := s.repo.GetPerformanceStats()
	latestSessions, _ := s.repo.GetLastSessions(10)
	latestTrades, _ := s.repo.GetLastTrades(10)

	events := []StreamEvent{
		{
			Type:      "stats",
			Timestamp: time.Now().Format(time.RFC3339),
			Data: map[string]interface{}{
				"total_trades":      stats.TotalTrades,
				"win_rate":          stats.WinRate,
				"total_pnl":         stats.TotalPnl,
			},
		},
		{
			Type:      "sessions",
			Timestamp: time.Now().Format(time.RFC3339),
			Data:      latestSessions,
		},
		{
			Type:      "trades",
			Timestamp: time.Now().Format(time.RFC3339),
			Data:      latestTrades,
		},
	}

	for _, event := range events {
		data, _ := json.Marshal(event)
		fmt.Fprintf(w, "data: %s\n\n", string(data))
		flusher.Flush()
	}
}
