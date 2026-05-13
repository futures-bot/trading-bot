package notifications

import (
	"log"
	"time"

	"gopkg.in/telebot.v3"
)

// Notifier is an interface for sending notifications.
type Notifier interface {
	Notify(message string)
}

// NullNotifier is a notifier that does nothing.
type NullNotifier struct{}

// NewNullNotifier creates a new NullNotifier.
func NewNullNotifier() *NullNotifier {
	return &NullNotifier{}
}

// Notify does nothing.
func (n *NullNotifier) Notify(message string) {
	// Do nothing
}

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
