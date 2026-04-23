package cmd

import (
	"context"
	"log"

	"trading-bot/internal/config"
	"trading-bot/internal/engine"
	"trading-bot/internal/logging"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var livePaperCmd = &cobra.Command{
	Use:   "livepaper",
	Short: "Run the bot in live paper trading mode",
	Run: func(cmd *cobra.Command, args []string) {
		runLivePaper()
	},
}

func init() {
	rootCmd.AddCommand(livePaperCmd)
}

func runLivePaper() {
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	logger, _ := zap.NewProduction()
	defer logger.Sync()
	sugar := logger.Sugar()

	pnlLogger, err := logging.NewPnlLogger("live_paper_results.log")
	if err != nil {
		log.Fatalf("Failed to create PnL logger: %v", err)
	}
	defer pnlLogger.Close()

	paperTrader, err := engine.NewPaperTrader(cfg, sugar, pnlLogger)
	if err != nil {
		log.Fatalf("Failed to create paper trader: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	paperTrader.Run(ctx)
}
