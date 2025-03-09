package services

import (
	"fmt"
	"log"
	"os"
)

// LogLevel represents the severity of a log message
type LogLevel int

const (
	// DEBUG level for detailed information
	DEBUG LogLevel = iota
	// INFO level for general operational information
	INFO
	// WARN level for warning events
	WARN
	// ERROR level for error events
	ERROR
	// FATAL level for critical errors
	FATAL
)

// Logger provides logging functionality
type Logger struct {
	level  LogLevel
	logger *log.Logger
}

// NewLogger creates a new logger
func NewLogger(level LogLevel) *Logger {
	// Set up standard logger to stdout
	logger := log.New(os.Stdout, "", log.LstdFlags)

	return &Logger{
		level:  level,
		logger: logger,
	}
}

// Debug logs a debug message
func (l *Logger) Debug(format string, v ...interface{}) {
	if l.level <= DEBUG {
		l.log("DEBUG", format, v...)
	}
}

// Info logs an info message
func (l *Logger) Info(format string, v ...interface{}) {
	if l.level <= INFO {
		l.log("INFO", format, v...)
	}
}

// Warn logs a warning message
func (l *Logger) Warn(format string, v ...interface{}) {
	if l.level <= WARN {
		l.log("WARN", format, v...)
	}
}

// Error logs an error message
func (l *Logger) Error(format string, v ...interface{}) {
	if l.level <= ERROR {
		l.log("ERROR", format, v...)
	}
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(format string, v ...interface{}) {
	if l.level <= FATAL {
		l.log("FATAL", format, v...)
		os.Exit(1)
	}
}

// log logs a message with the given level
func (l *Logger) log(level, format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)
	logMessage := fmt.Sprintf("[%s] %s", level, message)
	l.logger.Println(logMessage)
}
