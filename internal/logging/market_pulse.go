package logging

import (
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

// MarketPulseLog represents a snapshot of market data when the bot is not in a trade.

type MarketPulseLog struct {
	Time   string          `json:"time"`
	Price  decimal.Decimal `json:"price"`
	RSI    decimal.Decimal `json:"rsi"`
	EMAGap decimal.Decimal `json:"ema_gap"`
}

// MarketPulseLogger handles writing market pulse logs to a file.

type MarketPulseLogger struct {
	file *os.File
	mu   sync.Mutex
}

// NewMarketPulseLogger creates a new MarketPulseLogger.

func NewMarketPulseLogger(filePath string) (*MarketPulseLogger, error) {
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	return &MarketPulseLogger{file: file}, nil
}

// Log appends a market pulse log to the file.

func (l *MarketPulseLogger) Log(logEntry MarketPulseLog) {
	l.mu.Lock()
	defer l.mu.Unlock()

	logEntry.Time = time.Now().UTC().Format(time.RFC3339)

	data, err := json.Marshal(logEntry)
	if err != nil {
		log.Printf("Failed to marshal market pulse log: %v", err)
		return
	}

	if _, err := l.file.WriteString(string(data) + "\n"); err != nil {
		log.Printf("Failed to write to market pulse log file: %v", err)
	}
}

// Close closes the log file.

func (l *MarketPulseLogger) Close() {
	l.file.Close()
}
