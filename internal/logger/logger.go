package logger

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

type Level int

const (
	LevelError Level = iota
	LevelWarn
	LevelInfo
	LevelDebug
)

func (l Level) String() string {
	switch l {
	case LevelError:
		return "ERROR"
	case LevelWarn:
		return "WARN"
	case LevelInfo:
		return "INFO"
	case LevelDebug:
		return "DEBUG"
	default:
		return "UNKNOWN"
	}
}

type Logger struct {
	level    Level
	output   io.Writer
	filePath string
	mu       sync.Mutex
}

func New(level string) *Logger {
	var logLevel Level
	switch strings.ToUpper(level) {
	case "ERROR":
		logLevel = LevelError
	case "WARN":
		logLevel = LevelWarn
	case "INFO":
		logLevel = LevelInfo
	case "DEBUG":
		logLevel = LevelDebug
	default:
		logLevel = LevelInfo
	}

	return &Logger{
		level:  logLevel,
		output: os.Stdout,
	}
}

func NewFromEnv() *Logger {
	level := os.Getenv("LOG_LEVEL")
	if level == "" {
		level = "INFO"
	}

	logger := New(level)

	logFile := os.Getenv("LOG_FILE")
	if logFile != "" {
		logger.SetLogFile(logFile)
	}

	return logger
}

func (l *Logger) SetLogFile(filePath string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	l.output = io.MultiWriter(os.Stdout, file)
	l.filePath = filePath

	return nil
}

func (l *Logger) shouldLog(level Level) bool {
	return level <= l.level
}

func (l *Logger) log(level Level, format string, args ...interface{}) {
	if !l.shouldLog(level) {
		return
	}

	message := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logMessage := fmt.Sprintf("[%s] [%s] %s", timestamp, level.String(), message)

	l.mu.Lock()
	defer l.mu.Unlock()

	fmt.Fprintln(l.output, logMessage)
}

func (l *Logger) Error(format string, args ...interface{}) {
	l.log(LevelError, format, args...)
}

func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(LevelWarn, format, args...)
}

func (l *Logger) Info(format string, args ...interface{}) {
	l.log(LevelInfo, format, args...)
}

func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(LevelDebug, format, args...)
}

func (l *Logger) WithFields(fields map[string]interface{}) *Entry {
	return &Entry{
		logger: l,
		fields: fields,
	}
}

type Entry struct {
	logger *Logger
	fields map[string]interface{}
}

func (e *Entry) formatFields() string {
	if len(e.fields) == 0 {
		return ""
	}

	var pairs []string
	for key, value := range e.fields {
		pairs = append(pairs, fmt.Sprintf("%s=%v", key, value))
	}
	return " " + strings.Join(pairs, " ")
}

func (e *Entry) Error(format string, args ...interface{}) {
	fields := e.formatFields()
	if fields != "" {
		format += fields
	}
	e.logger.log(LevelError, format, args...)
}

func (e *Entry) Warn(format string, args ...interface{}) {
	fields := e.formatFields()
	if fields != "" {
		format += fields
	}
	e.logger.log(LevelWarn, format, args...)
}

func (e *Entry) Info(format string, args ...interface{}) {
	fields := e.formatFields()
	if fields != "" {
		format += fields
	}
	e.logger.log(LevelInfo, format, args...)
}

func (e *Entry) Debug(format string, args ...interface{}) {
	fields := e.formatFields()
	if fields != "" {
		format += fields
	}
	e.logger.log(LevelDebug, format, args...)
}
