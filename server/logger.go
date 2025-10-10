package server

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
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

// LogBuffer stores recent log entries in memory for admin panels
type LogBuffer struct {
	entries []LogEntry
	mutex   sync.RWMutex
	maxSize int
}

// Global log buffer for capturing logs
var globalLogBuffer = &LogBuffer{
	entries: make([]LogEntry, 0, 200),
	maxSize: 200, // Keep last 200 log entries
}

// Global debug file for runtime logs
var debugFile *os.File

// AddEntry adds a log entry to the buffer
func (lb *LogBuffer) AddEntry(entry LogEntry) {
	lb.mutex.Lock()
	defer lb.mutex.Unlock()

	lb.entries = append(lb.entries, entry)

	// Keep only the most recent entries
	if len(lb.entries) > lb.maxSize {
		lb.entries = lb.entries[len(lb.entries)-lb.maxSize:]
	}
}

// GetEntries returns a copy of all log entries (newest first)
func (lb *LogBuffer) GetEntries() []LogEntry {
	lb.mutex.RLock()
	defer lb.mutex.RUnlock()

	// Create a copy and reverse order (newest first)
	entriesCopy := make([]LogEntry, len(lb.entries))
	for i, j := 0, len(lb.entries)-1; j >= 0; i, j = i+1, j-1 {
		entriesCopy[i] = lb.entries[j]
	}

	return entriesCopy
}

// GetRecentEntries returns the most recent N log entries (newest first)
func (lb *LogBuffer) GetRecentEntries(count int) []LogEntry {
	lb.mutex.RLock()
	defer lb.mutex.RUnlock()

	if count > len(lb.entries) {
		count = len(lb.entries)
	}

	// Get the last N entries and reverse order (newest first)
	startIdx := len(lb.entries) - count
	entriesCopy := make([]LogEntry, count)
	for i, j := 0, len(lb.entries)-1; j >= startIdx; i, j = i+1, j-1 {
		entriesCopy[i] = lb.entries[j]
	}

	return entriesCopy
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

	// Add to log buffer for admin panels
	globalLogBuffer.AddEntry(entry)

	// Output as JSON
	jsonData, err := json.Marshal(entry)
	if err != nil {
		// Fallback to standard log if JSON marshaling fails
		log.Printf("[%s] %s: %s", level, l.component, message)
		return
	}

	// Write structured logs to debug file directly (not via log.Printf to avoid redirection)
	if debugFile != nil {
		fmt.Fprintf(debugFile, "%s\n", string(jsonData))
	} else {
		// Fallback to log.Printf if debug file not set
		log.Printf("%s", string(jsonData))
	}
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

// LogToFile enables logging to a file instead of stdout with rotation
func LogToFile(filename string) error {
	// Check if file exists and rotate if it's too large (>10MB)
	if stat, err := os.Stat(filename); err == nil {
		if stat.Size() > 10*1024*1024 { // 10MB
			// Rotate the log file
			rotatedName := filename + ".old"
			// Remove old backup if it exists (ignore error if it doesn't exist)
			_ = os.Remove(rotatedName)
			// Rename current to backup (ignore error, we'll create new file anyway)
			_ = os.Rename(filename, rotatedName)
		}
	}

	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("failed to open log file: %v", err)
	}

	// Store the debug file for structured logger to use
	debugFile = file

	// Redirect all log.Printf calls to the debug file
	log.SetOutput(file)
	return nil
}

// GetLogBuffer returns the global log buffer for admin panel access
func GetLogBuffer() *LogBuffer {
	return globalLogBuffer
}
