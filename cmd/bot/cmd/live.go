package cmd

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
	"trading-bot/internal/config"
	"trading-bot/internal/engine"
	"trading-bot/internal/exchange"
	"trading-bot/internal/logging"

	"github.com/adshao/go-binance/v2/futures"
	"github.com/joho/godotenv"
	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var liveCmd = &cobra.Command{
	Use:   "live",
	Short: "Run the bot in live trading mode",
	Run: func(cmd *cobra.Command, args []string) {
		runLive()
	},
}

func init() {
	rootCmd.AddCommand(liveCmd)
}

type Meta struct {
	SessionNumber int `json:"session_number"`
}

func runLive() {
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Print the loaded configuration.
	log.Printf("Loaded configuration: %+v", cfg)

	log.Printf("Protocol Updated: Waiting for %d confirmations", cfg.ConfirmationCount)

	err = godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	if os.Getenv("BINANCE_API_KEY") == "" || os.Getenv("BINANCE_SECRET_KEY") == "" {
		log.Fatal("BINANCE_API_KEY and BINANCE_SECRET_KEY must be set in .env file")
	}

	futures.UseTestnet = true // This is a global setting for the library
	pnlLogger, err := logging.NewPnlLogger("data/trades.jsonl")
	if err != nil {
		log.Fatalf("Failed to create PnL logger: %v", err)
	}
	defer pnlLogger.Close()

	marketPulseLogger, err := logging.NewMarketPulseLogger("data/market_pulse.jsonl")
	if err != nil {
		log.Fatalf("Failed to create Market Pulse logger: %v", err)
	}
	defer marketPulseLogger.Close()

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}
	defer logger.Sync() // flushes buffer, if any

	sugar := logger.Sugar()
	sugar.Info("Starting trading bot...")

	// Read meta.json to get the session number.
	metaFile, err := ioutil.ReadFile("meta.json")
	if err != nil {
		log.Fatalf("Failed to read meta.json: %v", err)
	}

	var meta Meta
	err = json.Unmarshal(metaFile, &meta)
	if err != nil {
		log.Fatalf("Failed to unmarshal meta.json: %v", err)
	}

	meta.SessionNumber++

	// Write the updated session number back to meta.json.
	metaFile, err = json.MarshalIndent(meta, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal meta.json: %v", err)
	}

	err = ioutil.WriteFile("meta.json", metaFile, 0644)
	if err != nil {
		log.Fatalf("Failed to write meta.json: %v", err)
	}

	sugar.Infof("Session number: %d", meta.SessionNumber)

	// Get the start time of the session.
	startTime := time.Now()

	// Create a context that can be cancelled.
	ctx, cancel := context.WithCancel(context.Background())

	// Set up a channel to listen for OS signals.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Create a new REST client.
	restClient := exchange.NewRestClient()

	// Set leverage.
	err = restClient.SetLeverage(ctx, cfg.Symbol, cfg.Leverage)
	if err != nil {
		sugar.Fatalf("Failed to set leverage: %v", err)
	}

	// Verify credentials on startup.
	balance, err := restClient.VerifyCredentialsAndGetBalance(ctx)
	if err != nil {
		sugar.Fatalf("Failed to verify credentials and get balance: %v", err)
	}

	startingBalance, err := decimal.NewFromString(balance.Balance)
	if err != nil {
		sugar.Fatalf("Failed to parse starting balance: %v", err)
	}

	availableBalance, err := decimal.NewFromString(balance.AvailableBalance)
	if err != nil {
		sugar.Fatalf("Failed to parse available balance: %v", err)
	}

	if availableBalance.IsZero() {
		sugar.Fatal("ERROR: Futures Wallet is empty. Please transfer USDT to Futures Testnet.")
	}

	sugar.Infof("Starting balance: %s | Available: %s", startingBalance, availableBalance)

	// Get exchange info.
	err = restClient.GetExchangeInfo(ctx, cfg.Symbol)
	if err != nil {
		sugar.Fatalf("Failed to get exchange info: %v", err)
	}

	// Create a new engine.
	eng, err := engine.New(cfg, sugar, pnlLogger, marketPulseLogger, startTime, meta.SessionNumber, availableBalance)
	if err != nil {
		sugar.Fatalf("Failed to create engine: %v", err)
	}

	// Run the engine.
	go eng.Run(ctx)

	// Wait for a signal to exit.
	<-sigCh
	sugar.Info("Received shutdown signal, cancelling orders and shutting down...")
	cancel()
	// The hardShutdown is now called from within the engine, so we just need to wait for the engine to stop.
	time.Sleep(2 * time.Second)
}
