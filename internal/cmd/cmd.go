package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"trading-bot/internal/backtest"
	"trading-bot/internal/config"
	"trading-bot/internal/database"
	"trading-bot/internal/events"
	"trading-bot/internal/notifications"
	"trading-bot/internal/risk"
	"trading-bot/internal/scraper"
	"trading-bot/internal/trading"
	"trading-bot/internal/trading/domain"
	"trading-bot/shared/eventdef"

	"github.com/joho/godotenv"
	"github.com/nats-io/nats.go"
	"github.com/shopspring/decimal"
)

const version = "4.0.0"

func Run() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "run":
		run()
	case "backtest":
		runBacktest()
	case "paper":
		runPaper()
	case "testnet":
		runTestnet()
	case "scrape":
		runScrape()
	case "trades":
		runTrades()
	case "sessions":
		runSessions()
	case "stats":
		runStats()
	case "version":
		fmt.Printf("trading-bot %s\n", version)
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
  run              Run scrape, paper, testnet, and backtest concurrently
  backtest [file]  Run backtest
  paper            Start paper trading
  testnet          Start testnet trading
  scrape           Scrape historical klines
  trades           List recent trades from the database
  sessions         List recent sessions from the database
  stats            Display overall trading statistics
  version          Print version
  help             Show this help message

CONFIGURATION:
  config.yaml   Trading parameters
  .env          NATS_URL, NATS_CREDS_FILE, BINANCE_API_KEY, BINANCE_SECRET_KEY, TELEGRAM_*

`, version)
}

func loadAll() (*config.Config, events.Publisher, database.Repository) {
	_ = godotenv.Load()

	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	repo, err := database.NewGormRepository(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	if cfg.NatsURL == "" {
		log.Fatal("NATS_URL must be set in .env")
	}

	opts := []nats.Option{}
	if cfg.NatsCredsFile != "" {
		opts = append(opts, nats.UserCredentials(cfg.NatsCredsFile))
	}

	publisher, err := events.NewPublisher(cfg.NatsURL, opts...)
	if err != nil {
		log.Printf("Failed to connect to nats (%s): %v. Falling back to NullPublisher.", cfg.NatsURL, err)
		publisher = &events.NullPublisher{}
	}

	return cfg, publisher, repo
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

func run() {
	cfg, publisher, repo := loadAll()
	defer publisher.Close()

	if cfg.BinanceAPIKey == "" || cfg.BinanceSecretKey == "" {
		log.Fatal("BINANCE_API_KEY and BINANCE_SECRET_KEY must be set in .env for 'run' mode")
	}

	notifier := makeNotifier(cfg)
	notifier.Notify(fmt.Sprintf("Bot starting in RUN mode: %d symbols, budget=%.0f USDT", len(cfg.Symbols), cfg.SessionBudget))

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

	sharedBudget := risk.NewSharedBudget(decimal.NewFromFloat(cfg.SessionBudget))

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Println("[RUN] Starting continuous scraper...")
		runContinuousScraper(ctx, cfg, publisher, repo, backtestTrigger)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Println("[RUN] Starting auto-backtest listener...")
		runAutoBacktest(ctx, cfg, publisher, repo, backtestTrigger)
	}()

	for _, sym := range cfg.Symbols {
		wg.Add(1)
		go func(symbol string) {
			defer wg.Done()
			log.Printf("[RUN] Starting paper trading for %s...", symbol)
			runPaperLoop(ctx, cfg, publisher, repo, symbol, sharedBudget)
		}(sym)

		wg.Add(1)
		go func(symbol string) {
			defer wg.Done()
			log.Printf("[RUN] Starting testnet trading for %s...", symbol)
			runTestnetLoop(ctx, cfg, publisher, notifier, repo, symbol, sharedBudget)
		}(sym)
	}

	wg.Wait()
	notifier.Notify("Bot stopped.")
	log.Println("All modes stopped.")
}

func runContinuousScraper(ctx context.Context, cfg *config.Config, publisher events.Publisher, repo database.Repository, trigger chan<- struct{}) {
	s := scraper.New(publisher, repo)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		log.Println("[SCRAPE] Starting hourly scrape cycle...")
		klineCount, err := s.ScrapeSymbols(ctx, cfg.Symbols, "1m", 1)
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

func runAutoBacktest(ctx context.Context, cfg *config.Config, publisher events.Publisher, repo database.Repository, trigger <-chan struct{}) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-trigger:
			log.Println("[BACKTEST] Triggered by new scrape data...")
			for _, sym := range cfg.Symbols {
				runBacktestFromDB(cfg, publisher, repo, sym, "1m")
			}
		}
	}
}

func runPaperLoop(ctx context.Context, cfg *config.Config, publisher events.Publisher, repo database.Repository, symbol string, sharedBudget *risk.SharedBudget) {
	sessionNum := 1
	for {
		select {
		case <-ctx.Done():
			log.Printf("[PAPER] Stopped %s.", symbol)
			return
		default:
		}

		log.Printf("[PAPER] Session #%d starting (symbol=%s, duration=%dmin)",
			sessionNum, symbol, cfg.SessionDurationMin)

		trader, err := trading.NewPaperTrader(cfg, publisher, symbol, sharedBudget)
		if err != nil {
			log.Printf("[PAPER] Failed to create paper trader for %s: %v", symbol, err)
			return
		}

		sessionCtx, sessionCancel := context.WithTimeout(ctx, time.Duration(cfg.SessionDurationMin)*time.Minute)

		now := time.Now()
		dbSession := &database.Session{
			StartTime:    now,
			StartBalance: decimal.NewFromFloat(cfg.PaperBalance),
			Mode:         "paper",
			Status:       "running",
		}
		if repo != nil {
			repo.CreateSession(dbSession)
		}

		trader.Start(sessionCtx)
		sessionCancel()

		log.Printf("[PAPER] Session #%d completed for %s", sessionNum, symbol)

		tradeLogs := trader.GetTradeLogs()
		wins, losses, netPnl, profitFactor, maxDrawdown, sharpeRatio, expectancy := calculateTradeStats(tradeLogs)

		if repo != nil {
			endTime := time.Now()
			dbSession.EndTime = &endTime
			dbSession.EndBalance = decimal.NewFromFloat(trader.GetStatus().Balance)
			dbSession.TotalProfit = decimal.NewFromFloat(netPnl)
			dbSession.Status = "completed"
			repo.UpdateSession(dbSession)

			for _, tl := range tradeLogs {
				repo.SaveTrade(&database.Trade{
					SessionID:  dbSession.ID,
					Symbol:     tl.Symbol,
					Time:       time.Now(), // approximate
					Side:       tl.Side,
					Entry:      decimal.NewFromFloat(tl.Entry),
					Exit:       decimal.NewFromFloat(tl.Exit),
					PnlUSDT:    decimal.NewFromFloat(tl.PnlUSDT),
					ExitReason: tl.ExitReason,
				})
			}
		}

		session := &domain.Session{
			Mode:         "paper",
			Symbol:       symbol,
			DurationSecs: int(time.Since(trader.GetStatus().StartTime).Seconds()),
			TotalTrades:  len(tradeLogs),
			Wins:         wins,
			Losses:       losses,
			NetPnL:       netPnl,
			FinalBalance: trader.GetStatus().Balance,
			ProfitFactor: profitFactor,
			MaxDrawdown:  maxDrawdown,
			SharpeRatio:  sharpeRatio,
			Expectancy:   expectancy,
			Status:       "completed",
		}
		publisher.Publish(context.Background(), "sessions", eventdef.NewEvent("session.completed", "paper-trader", 1, session))

		sessionNum++

		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

func runTestnetLoop(ctx context.Context, cfg *config.Config, publisher events.Publisher, notifier notifications.Notifier, repo database.Repository, symbol string, sharedBudget *risk.SharedBudget) {
	sessionNum := 1
	for {
		select {
		case <-ctx.Done():
			log.Printf("[TESTNET] Stopped %s.", symbol)
			return
		default:
		}

		log.Printf("[TESTNET] Session #%d starting (symbol=%s, duration=%dmin)",
			sessionNum, symbol, cfg.SessionDurationMin)

		trader, err := trading.NewBinanceTrader(
			cfg, notifier, publisher,
			time.Now(), sessionNum, decimal.Zero,
			cfg.BinanceAPIKey, cfg.BinanceSecretKey, symbol, sharedBudget,
		)
		if err != nil {
			log.Printf("[TESTNET] Failed to create trader for %s: %v", symbol, err)
			return
		}

		sessionCtx, sessionCancel := context.WithTimeout(ctx, time.Duration(cfg.SessionDurationMin)*time.Minute)

		now := time.Now()
		dbSession := &database.Session{
			StartTime:    now,
			StartBalance: decimal.NewFromFloat(cfg.SessionBudget), // assume session budget
			Mode:         "testnet",
			Status:       "running",
		}
		if repo != nil {
			repo.CreateSession(dbSession)
		}

		trader.Start(sessionCtx)
		sessionCancel()

		shutdownStatus := trader.GetShutdownStatus()

		log.Printf("[TESTNET] Session #%d completed for %s, status: %s", sessionNum, symbol, shutdownStatus)

		tradeLogs := trader.GetTradeLogs()
		wins, losses, netPnl, profitFactor, maxDrawdown, sharpeRatio, expectancy := calculateTradeStats(tradeLogs)

		if repo != nil {
			endTime := time.Now()
			dbSession.EndTime = &endTime
			dbSession.EndBalance = decimal.NewFromFloat(trader.GetStatus().Balance)
			dbSession.TotalProfit = decimal.NewFromFloat(netPnl)
			dbSession.Status = shutdownStatus
			repo.UpdateSession(dbSession)

			for _, tl := range tradeLogs {
				repo.SaveTrade(&database.Trade{
					SessionID:  dbSession.ID,
					Symbol:     tl.Symbol,
					Time:       time.Now(), // approximate for historical logs
					Side:       tl.Side,
					Entry:      decimal.NewFromFloat(tl.Entry),
					Exit:       decimal.NewFromFloat(tl.Exit),
					PnlUSDT:    decimal.NewFromFloat(tl.PnlUSDT),
					ExitReason: tl.ExitReason,
				})
			}
		}

		session := &domain.Session{
			Mode:         "testnet",
			Symbol:       symbol,
			DurationSecs: int(time.Since(trader.GetStatus().StartTime).Seconds()),
			TotalTrades:  len(tradeLogs),
			Wins:         wins,
			Losses:       losses,
			NetPnL:       netPnl,
			FinalBalance: trader.GetStatus().Balance,
			ProfitFactor: profitFactor,
			MaxDrawdown:  maxDrawdown,
			SharpeRatio:  sharpeRatio,
			Expectancy:   expectancy,
			Status:       shutdownStatus,
		}
		publisher.Publish(context.Background(), "sessions", eventdef.NewEvent("session.completed", "testnet-trader", 1, session))

		if shutdownStatus == "LIQUIDATED_OR_EMPTY" || shutdownStatus == "MARGIN_CALL" {
			msg := fmt.Sprintf("TESTNET STOPPED: Budget exhausted (%s) for %s. Session #%d",
				shutdownStatus, symbol, sessionNum)
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
	cfg, publisher, repo := loadAll()
	defer publisher.Close()

	symbols := cfg.Symbols
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

	s := scraper.New(nil, repo)
	klineCount, err := s.ScrapeSymbols(ctx, symbols, "1m", 24)
	if err != nil {
		log.Fatalf("Scrape failed: %v", err)
	}

	fmt.Printf("\nScrape complete. %d klines saved to database.\n", klineCount)

	if klineCount > 0 {
		fmt.Println("\nRunning backtest on scraped data...")
		for _, sym := range symbols {
			runBacktestFromDB(cfg, publisher, repo, sym, "1m")
		}
	}
}

func runBacktest() {
	cfg, publisher, repo := loadAll()
	defer publisher.Close()

	if len(os.Args) > 2 {
		file := os.Args[2]
		cfg.BacktestFile = file
		tracker := trading.NewTradeTracker(cfg.TakeProfitPct, cfg.StopLossPct, cfg.ConfirmationCount, cfg.MinProfitForFlipExit)
		strategy := trading.NewEMACrossover(cfg.EMAFast, cfg.EMASlow, 0, tracker)
		pm := trading.NewBacktestPositionManager(cfg, nil)
		runner := backtest.NewRunner(strategy, pm, cfg, publisher)
		if err := runner.Run(file, cfg.Symbols[0]); err != nil {
			log.Fatalf("Backtest failed: %v", err)
		}
		return
	}

	for _, sym := range cfg.Symbols {
		runBacktestFromDB(cfg, publisher, repo, sym, "1m")
	}
}

func runBacktestFromDB(cfg *config.Config, publisher events.Publisher, repo database.Repository, symbol, interval string) {
	klines, err := repo.GetKlines(symbol, interval, 500000)
	if err != nil || len(klines) == 0 {
		log.Print("No klines in database. Run 'trading-bot scrape' first.")
		return
	}

	fmt.Printf("Running backtest on %d klines from database (%s %s)...\n", len(klines), symbol, interval)

	tracker := trading.NewTradeTracker(cfg.TakeProfitPct, cfg.StopLossPct, cfg.ConfirmationCount, cfg.MinProfitForFlipExit)
	strategy := trading.NewEMACrossover(cfg.EMAFast, cfg.EMASlow, 0, tracker)
	pm := trading.NewBacktestPositionManager(cfg, nil)
	runner := backtest.NewRunner(strategy, pm, cfg, publisher)

	var candles []domain.Candle
	for _, k := range klines {
		candles = append(candles, domain.Candle{
			Open:  k.Open,
			High:  k.High,
			Low:   k.Low,
			Close: k.Close,
		})
	}

	if err := runner.RunFromCandles(candles, symbol); err != nil {
		log.Printf("Backtest failed: %v", err)
	}
}

func runPaper() {
	cfg, publisher, repo := loadAll()
	defer publisher.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Received shutdown signal. Stopping paper sessions...")
		cancel()
	}()

	sharedBudget := risk.NewSharedBudget(decimal.NewFromFloat(cfg.SessionBudget))

	var wg sync.WaitGroup
	for _, sym := range cfg.Symbols {
		wg.Add(1)
		go func(symbol string) {
			defer wg.Done()
			runPaperLoop(ctx, cfg, publisher, repo, symbol, sharedBudget)
		}(sym)
	}
	wg.Wait()
}

func runTestnet() {
	cfg, publisher, repo := loadAll()
	defer publisher.Close()

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

	sharedBudget := risk.NewSharedBudget(decimal.NewFromFloat(cfg.SessionBudget))

	var wg sync.WaitGroup
	for _, sym := range cfg.Symbols {
		wg.Add(1)
		go func(symbol string) {
			defer wg.Done()
			runTestnetLoop(ctx, cfg, publisher, notifier, repo, symbol, sharedBudget)
		}(sym)
	}
	wg.Wait()
}

func calculateTradeStats(tradeLogs []*domain.TradeLog) (wins, losses int, netPnl, profitFactor, maxDrawdown, sharpeRatio, expectancy float64) {
	var totalTrades, grossProfit, grossLoss float64
	var pnlHistory []float64

	for _, trade := range tradeLogs {
		totalTrades++
		pnlHistory = append(pnlHistory, trade.PnlUSDT)
		if trade.PnlUSDT > 0 {
			wins++
			grossProfit += trade.PnlUSDT
		} else {
			losses++
			grossLoss += trade.PnlUSDT
		}
		netPnl += trade.PnlUSDT
	}

	if grossLoss != 0 {
		profitFactor = grossProfit / -grossLoss
	}

	var peak float64 = 0
	for _, pnl := range pnlHistory {
		peak += pnl
		if peak > 0 {
			peak = 0
		}
		if peak < maxDrawdown {
			maxDrawdown = peak
		}
	}

	if totalTrades > 0 {
		expectancy = netPnl / totalTrades
	}

	return
}

func runTrades() {
	_ = godotenv.Load()
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	repo, err := database.NewGormRepository(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	trades, err := repo.GetTrades(0, 50)
	if err != nil {
		log.Fatalf("Failed to fetch trades: %v", err)
	}

	fmt.Printf("\n--- Recent Trades (Max 50) ---\n")
	fmt.Printf("%-5s | %-10s | %-10s | %-8s | %-10s | %-10s | %-10s | %-15s\n", "ID", "Time", "Symbol", "Side", "Entry", "Exit", "PnL (USDT)", "Reason")
	fmt.Println("-----------------------------------------------------------------------------------------------------------")
	for _, t := range trades {
		fmt.Printf("%-5d | %-10s | %-10s | %-8s | %-10s | %-10s | %-10s | %-15s\n",
			t.ID, t.Time.Format("01-02 15:04"), t.Symbol, t.Side, t.Entry.StringFixed(2), t.Exit.StringFixed(2), t.PnlUSDT.StringFixed(2), t.ExitReason)
	}
	fmt.Println()
}

func runSessions() {
	_ = godotenv.Load()
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	repo, err := database.NewGormRepository(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	sessions, err := repo.GetSessions(20)
	if err != nil {
		log.Fatalf("Failed to fetch sessions: %v", err)
	}

	fmt.Printf("\n--- Recent Sessions (Max 20) ---\n")
	fmt.Printf("%-5s | %-10s | %-10s | %-15s | %-10s | %-10s\n", "ID", "Mode", "Status", "Start Time", "Total PnL", "End Balance")
	fmt.Println("-------------------------------------------------------------------------")
	for _, s := range sessions {
		fmt.Printf("%-5d | %-10s | %-10s | %-15s | %-10s | %-10s\n",
			s.ID, s.Mode, s.Status, s.StartTime.Format("01-02 15:04:05"), s.TotalProfit.StringFixed(2), s.EndBalance.StringFixed(2))
	}
	fmt.Println()
}

func runStats() {
	_ = godotenv.Load()
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	repo, err := database.NewGormRepository(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	stats, err := repo.GetStats()
	if err != nil {
		log.Fatalf("Failed to fetch stats: %v", err)
	}

	fmt.Printf("\n--- Global Trading Statistics ---\n")
	fmt.Printf("Total Trades:    %d\n", stats["total_trades"])
	fmt.Printf("Winning Trades:  %d\n", stats["winning_trades"])
	fmt.Printf("Win Rate:        %.2f%%\n", stats["win_rate_pct"])
	fmt.Printf("Total Net PnL:   %.2f USDT\n", stats["total_profit"])
	fmt.Println("---------------------------------")
	fmt.Println()
}
