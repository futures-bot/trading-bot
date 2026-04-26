package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration.
// It is loaded from a YAML file.
type Config struct {
	Symbol               string  `yaml:"symbol"`
	Leverage             int     `yaml:"leverage"`
	SessionBudget        float64 `yaml:"session_budget"`
	EMAFast              int     `yaml:"ema_fast"`
	EMASlow              int     `yaml:"ema_slow"`
	MinEMAGap            float64 `yaml:"min_ema_gap"`
	TakeProfitPct        float64 `yaml:"take_profit_pct"`
	StopLossPct          float64 `yaml:"stop_loss_pct"`
	BreakEvenTriggerPct  float64 `yaml:"break_even_trigger_pct"`
	TrailDistancePct     float64 `yaml:"trail_distance_pct"`
	ConfirmationCount    int     `yaml:"confirmation_count"`
	MinProfitForFlipExit float64 `yaml:"min_profit_for_flip_exit"`
	PaperBalance         float64 `yaml:"paper_balance"`
	LossCooldown         int     `yaml:"loss_cooldown"`
	WinCooldown          int     `yaml:"win_cooldown"`
	TelegramBotToken     string
	TelegramChatID       int64
	APIKey               string `yaml:"api_key"`
	HTTPPort             string `yaml:"http_port"`
}

// LoadConfig reads the configuration from the given file path.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}

	cfg.TelegramBotToken = os.Getenv("TELEGRAM_BOT_TOKEN")
	chatIDStr := os.Getenv("TELEGRAM_CHAT_ID")
	if chatIDStr != "" {
		var chatID int64
		_, err := fmt.Sscan(chatIDStr, &chatID)
		if err == nil {
			cfg.TelegramChatID = chatID
		}
	}

	return &cfg, nil
}
