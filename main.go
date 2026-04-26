package main

import (
	"log"
	"trading-bot/internal/api"
	"trading-bot/internal/config"
	"trading-bot/internal/database"
	"trading-bot/internal/manager"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using environment variables")
	}

	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := database.NewDatabase("trading_bot.db")
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	repo := database.NewRepository(db)
	mgr := manager.NewManager(repo)

	apiServer := api.NewServer(mgr, cfg.APIKey, cfg.HTTPPort)
	apiServer.Start()
}
