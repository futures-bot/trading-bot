package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"trading-bot/internal/analytics"
	"trading-bot/internal/api"
	"trading-bot/internal/backtest"
	"trading-bot/internal/config"
	"trading-bot/internal/database"
	"trading-bot/internal/notifications"
	"trading-bot/internal/scraper"
	"trading-bot/internal/trading"
	"trading-bot/internal/trading/domain"

	"github.com/joho/godotenv"
	"github.com/shopspring/decimal"
)

const version = "4.0.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "run":
		runAll()
	case "backtest":
		runBacktest()
	case "paper":
		runPaper()
	case "testnet":
		runTestnet()
	case "scrape":
		runScrape()
	case "trades":
		showTrades()
	case "sessions":
		showSessions()
	case "stats":
		showStats()
	case "version":
		fmt.Printf("trading-bot %s\n", version)
	case "clean":
		cleanDatabase()
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf(`trading-bot %s - Binance Futures Trading Bot (CLI)

USAGE:
  trading-bot <command> [options]

COMMANDS:
  run              Run all modes concurrently (scrape + paper + testnet + backtest)
  backtest [file]  Run backtest (from file or DB klines if scraped)
  paper            Start paper trading with auto-rotating 1hr sessions
  testnet          Start testnet trading with auto-rotating 1hr sessions
  scrape           Scrape historical klines and auto-run backtest
  trades           Show recent trade history from database
  sessions         Show recent session history from database
  stats            Show overall performance statistics
  clean            Delete all trades, sessions, logs from database
  version          Print version
  help             Show this help message

CONFIGURATION:
  config.yaml   Trading parameters (symbol, leverage, EMA, TP/SL, etc.)
  .env          DATABASE_URL, BINANCE_API_KEY, BINANCE_SECRET_KEY, TELEGRAM_*

SESSIONS:
  Paper and testnet sessions auto-rotate every session_duration_min (default 60 min).
  Each session saves analytics to the DB, then a new one starts.
  Testnet stops if budget is exhausted and sends a Telegram notification.
  Press Ctrl+C to stop.

DATA:
  All trades, sessions, klines, and logs are persisted in PostgreSQL (Supabase).

EXAMPLES:
  trading-bot run                   Run everything 24/7
  trading-bot scrape                Scrape klines and run backtest
  trading-bot backtest              Backtest on DB klines
  trading-bot backtest data/f.jsonl Backtest on local file
  trading-bot paper                 Start rotating paper sessions
  trading-bot testnet               Start rotating testnet sessions
  trading-bot trades                Show last 20 trades
  trading-bot stats                 Show performance analytics

`, version)
}

func loadAll() (*config.Config, database.Repository) {
	_ = godotenv.Load()

	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL must be set in .env")
	}

	db, err := database.NewDatabase(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	repo := database.NewRepository(db)
	return cfg, repo
}

func makeNotifier(cfg *config.Config) notifications.Notifier {
	if cfg.TelegramBotToken != "" && cfg.TelegramChatID != 0 {
		n, err := notifications.NewTelegramNotifier(cfg.TelegramBotToken, cfg.TelegramChatID)
		if err != nil {
			log.Printf("Telegram notifier failed, using null notifier: %v", err)
			return notifications.NewNullNotifier()
		}
		return n
	}
	return notifications.NewNullNotifier()
}

// runAll launches scrape, paper, testnet, and backtest concurrently.
func runAll() {
	cfg, repo := loadAll()

	if cfg.BinanceAPIKey == "" || cfg.BinanceSecretKey == "" {
		log.Fatal("BINANCE_API_KEY and BINANCE_SECRET_KEY must be set in .env for 'run' mode")
	}

	notifier := makeNotifier(cfg)
	notifier.Notify(fmt.Sprintf("Bot starting in RUN mode: %s, budget=%.0f USDT", cfg.Symbol, cfg.SessionBudget))

	apiServer := api.NewServer(repo, "0.0.0.0:8080")
	if err := apiServer.Start(); err != nil {
		log.Printf("Failed to start API server: %v", err)
	}
	defer apiServer.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Received shutdown signal. Stopping all modes...")
		notifier.Notify("Bot shutting down (signal received)")
		cancel()
	}()

	backtestTrigger := make(chan struct{}, 1)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Println("[RUN] Starting continuous scraper...")
		runContinuousScraper(ctx, cfg, repo, backtestTrigger)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Println("[RUN] Starting auto-backtest listener...")
		runAutoBacktest(ctx, cfg, repo, backtestTrigger)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Println("[RUN] Starting paper trading...")
		runPaperLoop(ctx, cfg, repo)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Println("[RUN] Starting testnet trading...")
		runTestnetLoop(ctx, cfg, repo, notifier)
	}()

	wg.Wait()
	notifier.Notify("Bot stopped.")
	log.Println("All modes stopped.")
}

func runContinuousScraper(ctx context.Context, cfg *config.Config, repo database.Repository, trigger chan<- struct{}) {
	s := scraper.New(repo)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		log.Println("[SCRAPE] Starting hourly scrape cycle...")
		klineCount, err := s.ScrapeSymbols(ctx, []string{cfg.Symbol}, "1m", 1)
		if err != nil {
			log.Printf("[SCRAPE] Error: %v", err)
		} else {
			log.Printf("[SCRAPE] Cycle complete: %d klines saved", klineCount)
			if klineCount > 0 {
				select {
				case trigger <- struct{}{}:
				default:
				}
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(60 * time.Minute):
		}
	}
}

func runAutoBacktest(ctx context.Context, cfg *config.Config, repo database.Repository, trigger <-chan struct{}) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-trigger:
			log.Println("[BACKTEST] Triggered by new scrape data...")
			runBacktestFromDB(cfg, repo, cfg.Symbol, "1m")
		}
	}
}

func runPaperLoop(ctx context.Context, cfg *config.Config, repo database.Repository) {
	sessionNum := 1
	for {
		select {
		case <-ctx.Done():
			log.Println("[PAPER] Stopped.")
			return
		default:
		}

		log.Printf("[PAPER] Session #%d starting (symbol=%s, duration=%dmin)",
			sessionNum, cfg.Symbol, cfg.SessionDurationMin)

		trader, err := trading.NewPaperTrader(cfg, repo)
		if err != nil {
			log.Printf("[PAPER] Failed to create paper trader: %v", err)
			return
		}

		sessionCtx, sessionCancel := context.WithTimeout(ctx, time.Duration(cfg.SessionDurationMin)*time.Minute)
		startTime := time.Now()

		trader.Start(sessionCtx)
		sessionCancel()

		duration := time.Since(startTime)
		status := trader.GetStatus()

		report := analytics.Analyze(trader.GetTradeResults())
		session := &database.Session{
			Mode:         "paper",
			Symbol:       cfg.Symbol,
			DurationSecs: int(duration.Seconds()),
			TotalTrades:  report.TotalTrades,
			Wins:         report.Wins,
			Losses:       report.Losses,
			NetPnL:       report.NetPnL,
			FinalBalance: status.Balance,
			ProfitFactor: report.ProfitFactor,
			MaxDrawdown:  report.MaxDrawdown,
			SharpeRatio:  report.SharpeRatio,
			Expectancy:   report.Expectancy,
			Status:       "completed",
		}
		repo.SaveSession(session)

		log.Printf("[PAPER] Session #%d completed: %d trades, PnL: %.4f, PF: %.2f",
			sessionNum, report.TotalTrades, report.NetPnL, report.ProfitFactor)

		sessionNum++

		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

func runTestnetLoop(ctx context.Context, cfg *config.Config, repo database.Repository, notifier notifications.Notifier) {
	sessionNum := 1
	for {
		select {
		case <-ctx.Done():
			log.Println("[TESTNET] Stopped.")
			return
		default:
		}

		log.Printf("[TESTNET] Session #%d starting (symbol=%s, duration=%dmin)",
			sessionNum, cfg.Symbol, cfg.SessionDurationMin)

		trader, err := trading.NewBinanceTrader(
			cfg, notifier, repo,
			time.Now(), sessionNum, decimal.Zero,
			cfg.BinanceAPIKey, cfg.BinanceSecretKey,
		)
		if err != nil {
			log.Printf("[TESTNET] Failed to create trader: %v", err)
			return
		}

		sessionCtx, sessionCancel := context.WithTimeout(ctx, time.Duration(cfg.SessionDurationMin)*time.Minute)
		startTime := time.Now()

		trader.Start(sessionCtx)
		sessionCancel()

		duration := time.Since(startTime)
		shutdownStatus := trader.GetShutdownStatus()

		report := analytics.Analyze(trader.GetTradeResults())
		sessionStatus := "completed"
		if shutdownStatus != "" {
			sessionStatus = shutdownStatus
		}

		session := &database.Session{
			Mode:         "testnet",
			Symbol:       cfg.Symbol,
			DurationSecs: int(duration.Seconds()),
			TotalTrades:  report.TotalTrades,
			Wins:         report.Wins,
			Losses:       report.Losses,
			NetPnL:       report.NetPnL,
			FinalBalance: trader.GetStatus().Balance,
			ProfitFactor: report.ProfitFactor,
			MaxDrawdown:  report.MaxDrawdown,
			SharpeRatio:  report.SharpeRatio,
			Expectancy:   report.Expectancy,
			Status:       sessionStatus,
		}
		repo.SaveSession(session)

		log.Printf("[TESTNET] Session #%d completed: %d trades, PnL: %.4f, status: %s",
			sessionNum, report.TotalTrades, report.NetPnL, sessionStatus)

		if shutdownStatus == "LIQUIDATED_OR_EMPTY" || shutdownStatus == "MARGIN_CALL" {
			msg := fmt.Sprintf("TESTNET STOPPED: Budget exhausted (%s). Session #%d, PnL: %.4f",
				shutdownStatus, sessionNum, report.NetPnL)
			notifier.Notify(msg)
			log.Printf("[TESTNET] %s", msg)
			return
		}

		sessionNum++

		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

func runScrape() {
	cfg, repo := loadAll()

	symbols := []string{cfg.Symbol}
	if len(os.Args) > 2 {
		symbols = os.Args[2:]
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	s := scraper.New(repo)
	klineCount, err := s.ScrapeSymbols(ctx, symbols, "1m", 24)
	if err != nil {
		log.Fatalf("Scrape failed: %v", err)
	}

	fmt.Printf("\nScrape complete. %d klines saved to database.\n", klineCount)

	if klineCount > 0 {
		fmt.Println("\nRunning backtest on scraped data...")
		runBacktestFromDB(cfg, repo, cfg.Symbol, "1m")
	}
}

func runBacktest() {
	cfg, repo := loadAll()

	if len(os.Args) > 2 {
		file := os.Args[2]
		cfg.BacktestFile = file
		tracker := trading.NewTradeTracker(cfg.TakeProfitPct, cfg.StopLossPct, cfg.ConfirmationCount, cfg.MinProfitForFlipExit)
		strategy := trading.NewEMACrossover(cfg.EMAFast, cfg.EMASlow, 0, tracker)
		pm := trading.NewBacktestPositionManager(cfg)
		runner := backtest.NewRunner(strategy, pm, cfg, repo)
		if err := runner.Run(file); err != nil {
			log.Fatalf("Backtest failed: %v", err)
		}
		return
	}

	runBacktestFromDB(cfg, repo, cfg.Symbol, "1m")
}

func runBacktestFromDB(cfg *config.Config, repo database.Repository, symbol, interval string) {
	klines, err := repo.GetKlines(symbol, interval, 500000)
	if err != nil || len(klines) == 0 {
		log.Print("No klines in database. Run 'trading-bot scrape' first.")
		return
	}

	fmt.Printf("Running backtest on %d klines from database (%s %s)...\n", len(klines), symbol, interval)

	tracker := trading.NewTradeTracker(cfg.TakeProfitPct, cfg.StopLossPct, cfg.ConfirmationCount, cfg.MinProfitForFlipExit)
	strategy := trading.NewEMACrossover(cfg.EMAFast, cfg.EMASlow, 0, tracker)
	pm := trading.NewBacktestPositionManager(cfg)
	runner := backtest.NewRunner(strategy, pm, cfg, repo)

	var candles []domain.Candle
	for _, k := range klines {
		candles = append(candles, domain.Candle{
			Open:  decimal.NewFromFloat(k.Open),
			High:  decimal.NewFromFloat(k.High),
			Low:   decimal.NewFromFloat(k.Low),
			Close: decimal.NewFromFloat(k.Close),
		})
	}

	if err := runner.RunFromCandles(candles); err != nil {
		log.Printf("Backtest failed: %v", err)
	}
}

func runPaper() {
	cfg, repo := loadAll()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Received shutdown signal. Stopping paper sessions...")
		cancel()
	}()

	runPaperLoop(ctx, cfg, repo)
}

func runTestnet() {
	cfg, repo := loadAll()

	if cfg.BinanceAPIKey == "" || cfg.BinanceSecretKey == "" {
		log.Fatal("BINANCE_API_KEY and BINANCE_SECRET_KEY must be set in .env")
	}

	notifier := makeNotifier(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Received shutdown signal. Stopping testnet sessions...")
		cancel()
	}()

	runTestnetLoop(ctx, cfg, repo, notifier)
}

func showTrades() {
	_, repo := loadAll()

	trades, err := repo.GetLastTrades(20)
	if err != nil {
		log.Fatalf("Failed to get trades: %v", err)
	}

	if len(trades) == 0 {
		fmt.Println("No trades found.")
		return
	}

	fmt.Println("ID    | Symbol    | Side | Entry      | Exit       | PnL        | Reason       | Time")
	fmt.Println("------|-----------|------|------------|------------|------------|--------------|--------------------")
	for _, t := range trades {
		fmt.Printf("%-5d | %-9s | %-4s | %-10s | %-10s | %-10s | %-12s | %s\n",
			t.ID, t.Symbol, t.Side,
			t.EntryPrice.StringFixed(4), t.ExitPrice.StringFixed(4),
			t.Profit.StringFixed(4), t.ExitReason,
			t.CreatedAt.Format("2006-01-02 15:04"),
		)
	}
}

func showSessions() {
	_, repo := loadAll()

	sessions, err := repo.GetLastSessions(10)
	if err != nil {
		log.Fatalf("Failed to get sessions: %v", err)
	}

	if len(sessions) == 0 {
		fmt.Println("No sessions found.")
		return
	}

	fmt.Println("ID | Mode     | Symbol    | Trades | W/L    | PnL        | PF    | DD       | Sharpe | Status    | Time")
	fmt.Println("---|----------|-----------|--------|--------|------------|-------|----------|--------|-----------|--------------------")
	for _, s := range sessions {
		fmt.Printf("%-2d | %-8s | %-9s | %-6d | %d/%-4d | %-10.4f | %-5.2f | %-8.4f | %-6.4f | %-9s | %s\n",
			s.ID, s.Mode, s.Symbol,
			s.TotalTrades, s.Wins, s.Losses,
			s.NetPnL, s.ProfitFactor, s.MaxDrawdown, s.SharpeRatio,
			s.Status, s.CreatedAt.Format("2006-01-02 15:04"),
		)
	}
}

func showStats() {
	_, repo := loadAll()

	stats, err := repo.GetPerformanceStats()
	if err != nil {
		log.Fatalf("Failed to get stats: %v", err)
	}

	if stats.TotalTrades == 0 {
		fmt.Println("No trades found. Run a backtest or trading session first.")
		return
	}

	fmt.Println("==================== PERFORMANCE ====================")
	fmt.Printf("  Total Trades:  %d\n", stats.TotalTrades)
	fmt.Printf("  Win Rate:      %.2f%%\n", stats.WinRate)
	fmt.Printf("  Total PnL:     %.4f USDT\n", stats.TotalPnl)
	fmt.Println("=====================================================")
}

func cleanDatabase() {
	cfg, _ := loadAll()

	fmt.Println("WARNING: This will delete ALL trades, sessions, and logs from the database.")
	fmt.Print("Type 'yes' to confirm: ")
	var confirm string
	fmt.Scanln(&confirm)
	if confirm != "yes" {
		fmt.Println("Cancelled.")
		return
	}
	db, err := database.NewDatabase(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	db.Exec("DELETE FROM trades")
	db.Exec("DELETE FROM sessions")
	db.Exec("DELETE FROM trade_logs")
	db.Exec("DELETE FROM market_pulse_logs")
	db.Exec("DELETE FROM bot_logs")
	db.Exec("DELETE FROM klines")

	fmt.Println("✓ Database cleaned successfully.")
}
