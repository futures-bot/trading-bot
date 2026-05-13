package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"trading-bot/internal/database"

	"github.com/shopspring/decimal"
)

type Server struct {
	repo   database.Repository
	addr   string
	server *http.Server
	hub    *Hub
}

func NewServer(repo database.Repository, addr string) *Server {
	return &Server{
		repo: repo,
		addr: addr,
		hub:  newHub(),
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
	go s.hub.run()
	s.runJobs()

	mux := http.NewServeMux()

	mux.HandleFunc("/api/health", s.corsMiddleware(s.handleHealth))
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		s.handleWs(w, r)
	})

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

func (s *Server) handleWs(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	client := &Client{hub: s.hub, conn: conn, send: make(chan []byte, 256), topics: make(map[string]bool)}
	client.hub.register <- client

	go client.writePump()
	go client.readPump()
}

type WsMessage struct {
	Topic string      `json:"topic"`
	Data  interface{} `json:"data"`
}

func (s *Server) runJobs() {
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		for range ticker.C {
			// stats
			stats, err := s.repo.GetPerformanceStats()
			if err != nil {
				log.Printf("[API] failed to fetch stats: %v", err)
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
			msg, _ := json.Marshal(WsMessage{Topic: "stats", Data: resp})
			s.hub.SendToTopic("stats", msg)

			// trades
			trades, err := s.repo.GetLastTrades(50)
			if err != nil {
				log.Printf("[API] failed to fetch trades: %v", err)
			}
			var tradeResp []TradeResponse
			for _, t := range trades {
				tradeResp = append(tradeResp, TradeResponse{
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
			msg, _ = json.Marshal(WsMessage{Topic: "trades", Data: tradeResp})
			s.hub.SendToTopic("trades", msg)

			// sessions
			sessions, err = s.repo.GetLastSessions(50)
			if err != nil {
				log.Printf("[API] failed to fetch sessions: %v", err)
			}
			var sessResp []SessionResponse
			for _, sess := range sessions {
				sessResp = append(sessResp, SessionResponse{
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
			msg, _ = json.Marshal(WsMessage{Topic: "sessions", Data: sessResp})
			s.hub.SendToTopic("sessions", msg)

			// latest-session
			latestSessions, err := s.repo.GetLastSessions(100)
			if err != nil {
				log.Printf("[API] failed to fetch sessions: %v", err)
			}
			var testnetLatest, paperLatest, backtestLatest *database.Session
			for i := range latestSessions {
				if latestSessions[i].Mode == "testnet" && testnetLatest == nil {
					testnetLatest = &latestSessions[i]
				}
				if latestSessions[i].Mode == "paper" && paperLatest == nil {
					paperLatest = &latestSessions[i]
				}
				if latestSessions[i].Mode == "backtest" && backtestLatest == nil {
					backtestLatest = &latestSessions[i]
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
			latestSessResp := LatestSessionResponse{
				Testnet:  toResp(testnetLatest),
				Paper:    toResp(paperLatest),
				Backtest: toResp(backtestLatest),
			}
			msg, _ = json.Marshal(WsMessage{Topic: "latest-session", Data: latestSessResp})
			s.hub.SendToTopic("latest-session", msg)
		}
	}()
}

type SessionResponse struct {
	ID           uint    `json:"id"`
	Mode         string  `json:"mode"`
	Symbol       string  `json:"symbol"`
	DurationSec  int     `json:"duration_secs"`
	TotalTrades  int     `json:"total_trades"`
	Wins         int     `json:"wins"`
	Losses       int     `json:"losses"`
	NetPnL       float64 `json:"net_pnl"`
	FinalBalance float64 `json:"final_balance"`
	ProfitFactor float64 `json:"profit_factor"`
	MaxDrawdown  float64 `json:"max_drawdown"`
	SharpeRatio  float64 `json:"sharpe_ratio"`
	Expectancy   float64 `json:"expectancy"`
	Status       string  `json:"status"`
	CreatedAt    string  `json:"created_at"`
}

type LatestSessionResponse struct {
	Testnet  SessionResponse `json:"testnet"`
	Paper    SessionResponse `json:"paper"`
	Backtest SessionResponse `json:"backtest"`
}

type StatsResponse struct {
	TotalTrades      int64   `json:"total_trades"`
	WinRate          float64 `json:"win_rate"`
	TotalPnL         float64 `json:"total_pnl"`
	PaperSessions    int     `json:"paper_sessions"`
	TestnetSessions  int     `json:"testnet_sessions"`
	BacktestSessions int     `json:"backtest_sessions"`
}

type PerformanceResponse struct {
	Mode         string  `json:"mode"`
	Symbol       string  `json:"symbol"`
	TotalTrades  int     `json:"total_trades"`
	Wins         int     `json:"wins"`
	WinRate      float64 `json:"win_rate"`
	Losses       int     `json:"losses"`
	NetPnL       float64 `json:"net_pnl"`
	AvgWin       float64 `json:"avg_win"`
	AvgLoss      float64 `json:"avg_loss"`
	ProfitFactor float64 `json:"profit_factor"`
	Expectancy   float64 `json:"expectancy"`
	MaxDrawdown  float64 `json:"max_drawdown"`
	SharpeRatio  float64 `json:"sharpe_ratio"`
	BestTrade    float64 `json:"best_trade"`
	WorstTrade   float64 `json:"worst_trade"`
}

type StreamEvent struct {
	Type      string      `json:"type"`
	Timestamp string      `json:"timestamp"`
	Data      interface{} `json:"data"`
}
