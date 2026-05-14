package config

import (
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

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
	SessionDurationMin   int     `yaml:"session_duration_min"`
	BacktestFile         string  `yaml:"backtest_file"`

	NatsURL          string
	NatsCredsFile    string
	TelegramBotToken string
	TelegramChatID   int64
	BinanceAPIKey    string
	BinanceSecretKey string
	DB_DSN           string
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	if cfg.SessionDurationMin == 0 {
		cfg.SessionDurationMin = 60
	}

	cfg.NatsURL = os.Getenv("NATS_URL")
	cfg.NatsCredsFile = os.Getenv("NATS_CREDS_FILE")

	cfg.TelegramBotToken = os.Getenv("TELEGRAM_BOT_TOKEN")
	chatIDStr := os.Getenv("TELEGRAM_CHAT_ID")
	if chatIDStr != "" {
		chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
		if err == nil {
			cfg.TelegramChatID = chatID
		}
	}
	cfg.BinanceAPIKey = os.Getenv("BINANCE_API_KEY")
	cfg.BinanceSecretKey = os.Getenv("BINANCE_SECRET_KEY")
	cfg.DB_DSN = os.Getenv("DB_DSN")

	return &cfg, nil
}
