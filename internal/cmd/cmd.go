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

	"trading-bot/internal/config"
	"trading-bot/internal/events"
	"trading-bot/internal/notifications"
	"trading-bot/internal/scraper"
	"trading-bot/internal/trading"
	"trading-bot/internal/trading/domain"

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
  version          Print version
  help             Show this help message

CONFIGURATION:
  config.yaml   Trading parameters
  .env          NATS_URL, NATS_CREDS_FILE, BINANCE_API_KEY, BINANCE_SECRET_KEY, TELEGRAM_*

`, version)
}

func loadAll() (*config.Config, events.Publisher) {
	_ = godotenv.Load()

	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
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
		log.Fatalf("Failed to connect to nats: %v", err)
	}

	return cfg, publisher
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
	cfg, publisher := loadAll()
	defer publisher.Close()

	if cfg.BinanceAPIKey == "" || cfg.BinanceSecretKey == "" {
		log.Fatal("BINANCE_API_KEY and BINANCE_SECRET_KEY must be set in .env for 'run' mode")
	}

	notifier := makeNotifier(cfg)
	notifier.Notify(fmt.Sprintf("Bot starting in RUN mode: %s, budget=%.0f USDT", cfg.Symbol, cfg.SessionBudget))

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
		runContinuousScraper(ctx, cfg, publisher, backtestTrigger)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Println("[RUN] Starting auto-backtest listener...")
		runAutoBacktest(ctx, cfg, publisher, backtestTrigger)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Println("[RUN] Starting paper trading...")
		runPaperLoop(ctx, cfg, publisher)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Println("[RUN] Starting testnet trading...")
		runTestnetLoop(ctx, cfg, publisher, notifier)
	}()

	wg.Wait()
	notifier.Notify("Bot stopped.")
	log.Println("All modes stopped.")
}

func runContinuousScraper(ctx context.Context, cfg *config.Config, publisher events.Publisher, trigger chan<- struct{}) {
	s := scraper.New(publisher)
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

func runAutoBacktest(ctx context.Context, cfg *config.Config, publisher events.Publisher, trigger <-chan struct{}) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-trigger:
			log.Println("[BACKTEST] Triggered by new scrape data...")
			runBacktestFromDB(cfg, publisher, cfg.Symbol, "1m")
		}
	}
}

func runPaperLoop(ctx context.Context, cfg *config.Config, publisher events.Publisher) {
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

		trader, err := trading.NewPaperTrader(cfg, publisher)
		if err != nil {
			log.Printf("[PAPER] Failed to create paper trader: %v", err)
			return
		}

		sessionCtx, sessionCancel := context.WithTimeout(ctx, time.Duration(cfg.SessionDurationMin)*time.Minute)

		trader.Start(sessionCtx)
		sessionCancel()

		log.Printf("[PAPER] Session #%d completed", sessionNum)

		tradeLogs := trader.GetTradeLogs()
		wins, losses, netPnl, profitFactor, maxDrawdown, sharpeRatio, expectancy := calculateTradeStats(tradeLogs)

		session := &domain.Session{
			Mode:         "paper",
			Symbol:       cfg.Symbol,
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
		publisher.Publish(context.Background(), "sessions", session)

		sessionNum++

		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

func runTestnetLoop(ctx context.Context, cfg *config.Config, publisher events.Publisher, notifier notifications.Notifier) {
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
			cfg, notifier, publisher,
			time.Now(), sessionNum, decimal.Zero,
			cfg.BinanceAPIKey, cfg.BinanceSecretKey,
		)
		if err != nil {
			log.Printf("[TESTNET] Failed to create trader: %v", err)
			return
		}

		sessionCtx, sessionCancel := context.WithTimeout(ctx, time.Duration(cfg.SessionDurationMin)*time.Minute)

		trader.Start(sessionCtx)
		sessionCancel()

		shutdownStatus := trader.GetShutdownStatus()

		log.Printf("[TESTNET] Session #%d completed, status: %s", sessionNum, shutdownStatus)

		tradeLogs := trader.GetTradeLogs()
		wins, losses, netPnl, profitFactor, maxDrawdown, sharpeRatio, expectancy := calculateTradeStats(tradeLogs)

		session := &domain.Session{
			Mode:         "testnet",
			Symbol:       cfg.Symbol,
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
		publisher.Publish(context.Background(), "sessions", session)

		if shutdownStatus == "LIQUIDATED_OR_EMPTY" || shutdownStatus == "MARGIN_CALL" {
			msg := fmt.Sprintf("TESTNET STOPPED: Budget exhausted (%s). Session #%d",
				shutdownStatus, sessionNum)
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
	cfg, publisher := loadAll()
	defer publisher.Close()

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

	s := scraper.New(nil)
	klineCount, err := s.ScrapeSymbols(ctx, symbols, "1m", 24)
	if err != nil {
		log.Fatalf("Scrape failed: %v", err)
	}

	fmt.Printf("\nScrape complete. %d klines saved to database.\n", klineCount)

	if klineCount > 0 {
		fmt.Println("\nRunning backtest on scraped data...")
		// runBacktestFromDB(cfg, repo, cfg.Symbol, "1m")
	}
}

func runBacktest() {
	// cfg, repo := loadAll()

	// if len(os.Args) > 2 {
	// 	file := os.Args[2]
	// 	cfg.BacktestFile = file
	// 	tracker := trading.NewTradeTracker(cfg.TakeProfitPct, cfg.StopLossPct, cfg.ConfirmationCount, cfg.MinProfitForFlipExit)
	// 	strategy := trading.NewEMACrossover(cfg.EMAFast, cfg.EMASlow, 0, tracker)
	// 	pm := trading.NewBacktestPositionManager(cfg)
	// 	runner := backtest.NewRunner(strategy, pm, cfg, repo)
	// 	if err := runner.Run(file); err != nil {
	// 		log.Fatalf("Backtest failed: %v", err)
	// 	}
	// 	return
	// }

	// runBacktestFromDB(cfg, repo, cfg.Symbol, "1m")
}

func runBacktestFromDB(cfg *config.Config, publisher events.Publisher, symbol, interval string) {
	// klines, err := repo.GetKlines(symbol, interval, 500000)
	// if err != nil || len(klines) == 0 {
	// 	log.Print("No klines in database. Run 'trading-bot scrape' first.")
	// 	return
	// }

	// fmt.Printf("Running backtest on %d klines from database (%s %s)...\n", len(klines), symbol, interval)

	// tracker := trading.NewTradeTracker(cfg.TakeProfitPct, cfg.StopLossPct, cfg.ConfirmationCount, cfg.MinProfitForFlipExit)
	// strategy := trading.NewEMACrossover(cfg.EMAFast, cfg.EMASlow, 0, tracker)
	// pm := trading.NewBacktestPositionManager(cfg)
	// runner := backtest.NewRunner(strategy, pm, cfg, repo)

	// var candles []domain.Candle
	// for _, k := range klines {
	// 	candles = append(candles, domain.Candle{
	// 		Open:  decimal.NewFromFloat(k.Open),
	// 		High:  decimal.NewFromFloat(k.High),
	// 		Low:   decimal.NewFromFloat(k.Low),
	// 		Close: decimal.NewFromFloat(k.Close),
	// 	})
	// }

	// if err := runner.RunFromCandles(candles); err != nil {
	// 	log.Printf("Backtest failed: %v", err)
	// }
}

func runPaper() {
	cfg, publisher := loadAll()
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

	runPaperLoop(ctx, cfg, publisher)
}

func runTestnet() {
	cfg, publisher := loadAll()
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

	runTestnetLoop(ctx, cfg, publisher, notifier)
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
