package logging

import (
	"fmt"
	"log"
	"os"
	"time"
)

type Logger struct {
	level Level
}

type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

func (l Level) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

func NewLogger(level string) *Logger {
	var l Level
	switch level {
	case "debug":
		l = DEBUG
	case "info":
		l = INFO
	case "warn":
		l = WARN
	case "error":
		l = ERROR
	default:
		l = INFO
	}

	return &Logger{level: l}
}

func (l *Logger) log(level Level, msg string, args ...interface{}) {
	if level < l.level {
		return
	}

	timestamp := time.Now().Format(time.RFC3339)
	
	// Format message with key-value pairs
	message := msg
	if len(args) > 0 {
		message += " "
		for i := 0; i < len(args); i += 2 {
			if i+1 < len(args) {
				message += fmt.Sprintf("%v=%v ", args[i], args[i+1])
			} else {
				message += fmt.Sprintf("%v ", args[i])
			}
		}
	}

	// Color coding for different log levels
	var colorCode string
	switch level {
	case DEBUG:
		colorCode = "\033[36m" // Cyan
	case INFO:
		colorCode = "\033[32m" // Green
	case WARN:
		colorCode = "\033[33m" // Yellow
	case ERROR:
		colorCode = "\033[31m" // Red
	}
	resetCode := "\033[0m"

	logMessage := fmt.Sprintf("%s[%s]%s %s %s%s%s",
		colorCode, level.String(), resetCode,
		timestamp,
		colorCode, message, resetCode)

	if level == ERROR {
		log.Println(logMessage)
		// Also write to stderr for errors
		fmt.Fprintln(os.Stderr, logMessage)
	} else {
		log.Println(logMessage)
	}
}

func (l *Logger) Debug(msg string, args ...interface{}) {
	l.log(DEBUG, msg, args...)
}

func (l *Logger) Info(msg string, args ...interface{}) {
	l.log(INFO, msg, args...)
}

func (l *Logger) Warn(msg string, args ...interface{}) {
	l.log(WARN, msg, args...)
}

func (l *Logger) Error(msg string, args ...interface{}) {
	l.log(ERROR, msg, args...)
}

func (l *Logger) Fatal(msg string, args ...interface{}) {
	l.log(ERROR, msg, args...)
	os.Exit(1)
}