package cmd

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"trading-bot/internal/config"
	"trading-bot/internal/exchange"
	"trading-bot/internal/logging"

	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var scrapeCmd = &cobra.Command{
	Use:   "scrape",
	Short: "Run the bot in scrape mode",
	Run: func(cmd *cobra.Command, args []string) {
		runScrape()
	},
}

func init() {
	rootCmd.AddCommand(scrapeCmd)
}

func runScrape() {
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}
	defer logger.Sync()
	sugar := logger.Sugar()

	marketPulseLogger, err := logging.NewMarketPulseLogger("data/market_pulse.jsonl")
	if err != nil {
		log.Fatalf("Failed to create Market Pulse logger: %v", err)
	}
	defer marketPulseLogger.Close()

	wsClient := exchange.NewWebsocketClient(sugar)

	priceCh := make(chan exchange.PriceUpdate)
	ctx, cancel := context.WithCancel(context.Background())

	wsClient.Start(ctx, cfg.Symbol, priceCh)

	log.Println("Scraping data...")

	// Handle shutdown gracefully
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case priceUpdate := <-priceCh:
			price, err := decimal.NewFromString(priceUpdate.Price)
			if err != nil {
				log.Printf("Failed to parse price: %v", err)
				continue
			}
			marketPulseLogger.Log(logging.MarketPulseLog{Price: price})
		case <-sigCh:
			log.Println("Shutting down scraper...")
			cancel()
			return
		}
	}
}
