package debug

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

type Level int

const (
	LevelOff Level = iota
	LevelBasic
	LevelDetailed
	LevelFull
)

type Logger struct {
	level   Level
	mu      sync.Mutex
	writer  io.Writer
	file    *os.File
	colored bool
}

var (
	globalLogger *Logger
	once         sync.Once
)

// Colors for debug output
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
)

func Init(level Level, filePath string, colored bool) error {
	once.Do(func() {
		globalLogger = &Logger{
			level:   level,
			writer:  os.Stderr,
			colored: colored,
		}
	})

	globalLogger.level = level
	globalLogger.colored = colored

	if filePath != "" {
		f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to open debug file: %w", err)
		}
		globalLogger.file = f
		globalLogger.writer = io.MultiWriter(os.Stderr, f)
		globalLogger.colored = false // No colors in file
	}

	return nil
}

func Close() {
	if globalLogger != nil && globalLogger.file != nil {
		globalLogger.file.Close()
	}
}

func GetLogger() *Logger {
	if globalLogger == nil {
		globalLogger = &Logger{
			level:   LevelOff,
			writer:  os.Stderr,
			colored: true,
		}
	}
	return globalLogger
}

func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

func (l *Logger) GetLevel() Level {
	return l.level
}

func (l *Logger) IsEnabled() bool {
	return l.level > LevelOff
}

func (l *Logger) timestamp() string {
	return time.Now().Format("2006-01-02 15:04:05.000")
}

func (l *Logger) colorize(color, text string) string {
	if !l.colored {
		return text
	}
	return color + text + colorReset
}

func (l *Logger) log(level Level, category, message string) {
	if l.level < level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := l.colorize(colorGray, l.timestamp())
	tag := l.colorize(colorCyan, "[DEBUG]")
	cat := l.colorize(colorYellow, fmt.Sprintf("[%s]", category))

	fmt.Fprintf(l.writer, "%s %s %s %s\n", timestamp, tag, cat, message)
}

// Basic level logging (Level 1)
func (l *Logger) Info(category, format string, args ...interface{}) {
	l.log(LevelBasic, category, fmt.Sprintf(format, args...))
}

// Detailed level logging (Level 2)
func (l *Logger) Detail(category, format string, args ...interface{}) {
	l.log(LevelDetailed, category, fmt.Sprintf(format, args...))
}

// Full level logging (Level 3)
func (l *Logger) Trace(category, format string, args ...interface{}) {
	l.log(LevelFull, category, fmt.Sprintf(format, args...))
}

// SMTP conversation logging
func (l *Logger) SMTPSend(cmd string) {
	if l.level < LevelDetailed {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := l.colorize(colorGray, l.timestamp())
	tag := l.colorize(colorCyan, "[DEBUG]")
	arrow := l.colorize(colorGreen, ">>>")

	fmt.Fprintf(l.writer, "%s %s %s %s\n", timestamp, tag, arrow, cmd)
}

func (l *Logger) SMTPRecv(response string) {
	if l.level < LevelDetailed {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := l.colorize(colorGray, l.timestamp())
	tag := l.colorize(colorCyan, "[DEBUG]")
	arrow := l.colorize(colorBlue, "<<<")

	fmt.Fprintf(l.writer, "%s %s %s %s\n", timestamp, tag, arrow, response)
}

// Error logging (always shown if debug enabled)
func (l *Logger) Error(category, format string, args ...interface{}) {
	if l.level < LevelBasic {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := l.colorize(colorGray, l.timestamp())
	tag := l.colorize(colorRed, "[ERROR]")
	cat := l.colorize(colorYellow, fmt.Sprintf("[%s]", category))

	fmt.Fprintf(l.writer, "%s %s %s %s\n", timestamp, tag, cat, fmt.Sprintf(format, args...))
}

// Success logging
func (l *Logger) Success(category, format string, args ...interface{}) {
	if l.level < LevelBasic {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := l.colorize(colorGray, l.timestamp())
	tag := l.colorize(colorGreen, "[OK]")
	cat := l.colorize(colorYellow, fmt.Sprintf("[%s]", category))

	fmt.Fprintf(l.writer, "%s %s %s %s\n", timestamp, tag, cat, fmt.Sprintf(format, args...))
}

// Timing helper
type Timer struct {
	start    time.Time
	category string
	message  string
	logger   *Logger
}

func (l *Logger) StartTimer(category, message string) *Timer {
	if l.level >= LevelBasic {
		l.Info(category, "Starting: %s", message)
	}
	return &Timer{
		start:    time.Now(),
		category: category,
		message:  message,
		logger:   l,
	}
}

func (t *Timer) Stop() time.Duration {
	elapsed := time.Since(t.start)
	if t.logger.level >= LevelBasic {
		t.logger.Info(t.category, "Completed: %s (took %v)", t.message, elapsed)
	}
	return elapsed
}

func (t *Timer) Elapsed() time.Duration {
	return time.Since(t.start)
}

// Convenience functions using global logger
func Info(category, format string, args ...interface{}) {
	GetLogger().Info(category, format, args...)
}

func Detail(category, format string, args ...interface{}) {
	GetLogger().Detail(category, format, args...)
}

func Trace(category, format string, args ...interface{}) {
	GetLogger().Trace(category, format, args...)
}

func SMTPSend(cmd string) {
	GetLogger().SMTPSend(cmd)
}

func SMTPRecv(response string) {
	GetLogger().SMTPRecv(response)
}

func Error(category, format string, args ...interface{}) {
	GetLogger().Error(category, format, args...)
}

func Success(category, format string, args ...interface{}) {
	GetLogger().Success(category, format, args...)
}

func StartTimer(category, message string) *Timer {
	return GetLogger().StartTimer(category, message)
}
