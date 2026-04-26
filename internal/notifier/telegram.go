package notifier

import (
	"log"
	"time"

	"gopkg.in/telebot.v3"
)

// TelegramNotifier is a Notifier that sends messages to a Telegram chat.
type TelegramNotifier struct {
	bot    *telebot.Bot
	chatID int64
}

// NewTelegramNotifier creates a new TelegramNotifier.
func NewTelegramNotifier(token string, chatID int64) (*TelegramNotifier, error) {
	pref := telebot.Settings{
		Token:  token,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	}

	bot, err := telebot.NewBot(pref)
	if err != nil {
		return nil, err
	}

	return &TelegramNotifier{
		bot:    bot,
		chatID: chatID,
	}, nil
}

// Notify sends a message to the Telegram chat.
func (tn *TelegramNotifier) Notify(message string) {
	_, err := tn.bot.Send(&telebot.Chat{ID: tn.chatID}, message)
	if err != nil {
		log.Printf("Failed to send Telegram notification: %v", err)
	}
}
