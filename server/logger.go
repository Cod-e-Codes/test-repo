package server

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

// LogLevel represents the severity level of a log entry
type LogLevel string

const (
	LogLevelDebug LogLevel = "DEBUG"
	LogLevelInfo  LogLevel = "INFO"
	LogLevelWarn  LogLevel = "WARN"
	LogLevelError LogLevel = "ERROR"
)

// LogEntry represents a structured log entry
type LogEntry struct {
	Level     LogLevel               `json:"level"`
	Timestamp time.Time              `json:"timestamp"`
	Component string                 `json:"component"`
	UserID    string                 `json:"user_id,omitempty"`
	Message   string                 `json:"message"`
	Error     string                 `json:"error,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// Logger provides structured logging functionality
type Logger struct {
	component string
	userID    string
}

// NewLogger creates a new logger instance for a specific component
func NewLogger(component string) *Logger {
	return &Logger{
		component: component,
	}
}

// WithUser creates a new logger instance with a specific user ID
func (l *Logger) WithUser(userID string) *Logger {
	return &Logger{
		component: l.component,
		userID:    userID,
	}
}

// Debug logs a debug message
func (l *Logger) Debug(message string, data ...map[string]interface{}) {
	l.log(LogLevelDebug, message, nil, data...)
}

// Info logs an info message
func (l *Logger) Info(message string, data ...map[string]interface{}) {
	l.log(LogLevelInfo, message, nil, data...)
}

// Warn logs a warning message
func (l *Logger) Warn(message string, data ...map[string]interface{}) {
	l.log(LogLevelWarn, message, nil, data...)
}

// Error logs an error message
func (l *Logger) Error(message string, err error, data ...map[string]interface{}) {
	var errorStr string
	if err != nil {
		errorStr = err.Error()
	}
	l.log(LogLevelError, message, &errorStr, data...)
}

// log creates and outputs a structured log entry
func (l *Logger) log(level LogLevel, message string, error *string, data ...map[string]interface{}) {
	entry := LogEntry{
		Level:     level,
		Timestamp: time.Now(),
		Component: l.component,
		UserID:    l.userID,
		Message:   message,
	}

	if error != nil {
		entry.Error = *error
	}

	// Merge additional data
	if len(data) > 0 {
		entry.Data = make(map[string]interface{})
		for _, d := range data {
			for k, v := range d {
				entry.Data[k] = v
			}
		}
	}

	// Output as JSON
	jsonData, err := json.Marshal(entry)
	if err != nil {
		// Fallback to standard log if JSON marshaling fails
		log.Printf("[%s] %s: %s", level, l.component, message)
		return
	}

	// Use standard log output for now, but structured
	log.Printf("%s", string(jsonData))
}

// Global logger instances for different components
var (
	ServerLogger   = NewLogger("Server")
	ClientLogger   = NewLogger("Client")
	HubLogger      = NewLogger("Hub")
	AdminLogger    = NewLogger("Admin")
	PluginLogger   = NewLogger("Plugin")
	DatabaseLogger = NewLogger("Database")
	SecurityLogger = NewLogger("Security")
	FilterLogger   = NewLogger("Filter")
)

// SetLogLevel sets the minimum log level (currently not implemented but ready for future use)
func SetLogLevel(level LogLevel) {
	// This could be implemented to filter log output based on level
	// For now, we log everything
}

// LogToFile enables logging to a file in addition to stdout
func LogToFile(filename string) error {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("failed to open log file: %v", err)
	}

	log.SetOutput(file)
	return nil
}
