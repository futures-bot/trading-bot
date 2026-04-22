package cmd

import (
	"log"

	"trading-bot/internal/backtest"
	"trading-bot/internal/config"
	"trading-bot/internal/risk"
	"trading-bot/internal/strategy"

	"github.com/spf13/cobra"
)

var backtestCmd = &cobra.Command{
	Use:   "backtest",
	Short: "Run the bot in backtesting mode",
	Run: func(cmd *cobra.Command, args []string) {
		filePath, _ := cmd.Flags().GetString("file")
		if filePath == "" {
			log.Fatal("Please provide a file path with --file")
		}
		runBacktest(filePath)
	},
}

func init() {
	backtestCmd.Flags().String("file", "", "Path to the historical data file")
	rootCmd.AddCommand(backtestCmd)
}

func runBacktest(filePath string) {
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	tradeTracker := strategy.NewTradeTracker(cfg.TakeProfitPct, cfg.StopLossPct, cfg.ConfirmationCount, cfg.MinProfitForFlipExit)
	strategy := strategy.NewEMACrossover(cfg.EMAFast, cfg.EMASlow, 0, tradeTracker)

	positionManager := risk.NewBacktestPositionManager(cfg)

	backtestRunner := backtest.NewRunner(strategy, positionManager, cfg)

	if err := backtestRunner.Run(filePath); err != nil {
		log.Fatalf("Backtest failed: %v", err)
	}
}
