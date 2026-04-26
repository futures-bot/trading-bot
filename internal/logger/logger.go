package logger

import (
	"log"

	"gopkg.in/natefinch/lumberjack.v2"
)

// New creates a new logger with log rotation.
func New(logpath string) {
	log.SetOutput(&lumberjack.Logger{
		Filename:   logpath,
		MaxSize:    10, // megabytes
		MaxBackups: 3,
		MaxAge:     28, // days
		Compress:   true,
	})
}
