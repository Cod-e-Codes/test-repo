package server

import (
	"fmt"
	"strings"
	"time"

	"github.com/Cod-e-Codes/marchat/shared"
)

// FilterType represents the type of filter being applied
type FilterType string

const (
	FilterUser     FilterType = "user"
	FilterTime     FilterType = "time"
	FilterFile     FilterType = "file"
	FilterAdmin    FilterType = "admin"
	FilterToday    FilterType = "today"
	FilterDuration FilterType = "duration"
)

// MessageFilter represents a filter for messages
type MessageFilter struct {
	Type      FilterType
	Value     string
	StartTime *time.Time
	EndTime   *time.Time
}

// FilterEngine handles message filtering operations
type FilterEngine struct {
	activeFilters []MessageFilter
}

// NewFilterEngine creates a new filter engine
func NewFilterEngine() *FilterEngine {
	return &FilterEngine{
		activeFilters: make([]MessageFilter, 0),
	}
}

// ParseFilterCommand parses a filter command string
func (fe *FilterEngine) ParseFilterCommand(command string) ([]MessageFilter, error) {
	// Remove the :filter prefix
	command = strings.TrimPrefix(command, ":filter")
	command = strings.TrimSpace(command)

	if command == "" {
		return nil, fmt.Errorf("no filter specified")
	}

	var filters []MessageFilter
	parts := strings.Fields(command)

	for _, part := range parts {
		filter, err := fe.parseFilterPart(part)
		if err != nil {
			return nil, fmt.Errorf("invalid filter '%s': %v", part, err)
		}
		filters = append(filters, filter)
	}

	return filters, nil
}

// parseFilterPart parses a single filter part
func (fe *FilterEngine) parseFilterPart(part string) (MessageFilter, error) {
	switch {
	case strings.HasPrefix(part, "@"):
		// User filter: @username
		username := strings.TrimPrefix(part, "@")
		if username == "" {
			return MessageFilter{}, fmt.Errorf("username cannot be empty")
		}
		return MessageFilter{
			Type:  FilterUser,
			Value: username,
		}, nil

	case part == "today":
		// Today filter
		now := time.Now()
		startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		endOfDay := startOfDay.Add(24 * time.Hour)

		return MessageFilter{
			Type:      FilterToday,
			StartTime: &startOfDay,
			EndTime:   &endOfDay,
		}, nil

	case part == "#file":
		// File messages only
		return MessageFilter{
			Type: FilterFile,
		}, nil

	case part == "admin":
		// Admin messages only
		return MessageFilter{
			Type: FilterAdmin,
		}, nil

	case strings.HasPrefix(part, ">"):
		// Duration filter: >5min, >1h, etc.
		durationStr := strings.TrimPrefix(part, ">")
		duration, err := fe.parseDuration(durationStr)
		if err != nil {
			return MessageFilter{}, fmt.Errorf("invalid duration: %v", err)
		}

		startTime := time.Now().Add(-duration)
		return MessageFilter{
			Type:      FilterDuration,
			StartTime: &startTime,
		}, nil

	default:
		// Try to parse as a time filter (e.g., "2024-01-01", "15:30")
		if filter, err := fe.parseTimeFilter(part); err == nil {
			return filter, nil
		}

		return MessageFilter{}, fmt.Errorf("unknown filter type: %s", part)
	}
}

// parseDuration parses duration strings like "5min", "1h", "30s"
func (fe *FilterEngine) parseDuration(durationStr string) (time.Duration, error) {
	// Remove common suffixes and parse
	durationStr = strings.ToLower(durationStr)

	var multiplier time.Duration
	switch {
	case strings.HasSuffix(durationStr, "min"):
		multiplier = time.Minute
		durationStr = strings.TrimSuffix(durationStr, "min")
	case strings.HasSuffix(durationStr, "h"):
		multiplier = time.Hour
		durationStr = strings.TrimSuffix(durationStr, "h")
	case strings.HasSuffix(durationStr, "s"):
		multiplier = time.Second
		durationStr = strings.TrimSuffix(durationStr, "s")
	case strings.HasSuffix(durationStr, "d"):
		multiplier = 24 * time.Hour
		durationStr = strings.TrimSuffix(durationStr, "d")
	default:
		// Try to parse as minutes if no suffix
		multiplier = time.Minute
	}

	// Parse the number
	var value float64
	_, err := fmt.Sscanf(durationStr, "%f", &value)
	if err != nil {
		return 0, fmt.Errorf("invalid duration format")
	}

	return time.Duration(value) * multiplier, nil
}

// parseTimeFilter attempts to parse time-based filters
func (fe *FilterEngine) parseTimeFilter(part string) (MessageFilter, error) {
	// Try different time formats
	formats := []string{
		"2006-01-02",
		"15:04",
		"2006-01-02 15:04",
		"2006-01-02 15:04:05",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, part); err == nil {
			// If it's just a date, create a day range
			if format == "2006-01-02" {
				startOfDay := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
				endOfDay := startOfDay.Add(24 * time.Hour)
				return MessageFilter{
					Type:      FilterTime,
					StartTime: &startOfDay,
					EndTime:   &endOfDay,
				}, nil
			}

			// For time-only formats, assume today
			if format == "15:04" {
				now := time.Now()
				startTime := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
				endTime := startTime.Add(time.Hour)
				return MessageFilter{
					Type:      FilterTime,
					StartTime: &startTime,
					EndTime:   &endTime,
				}, nil
			}

			// For full datetime formats
			return MessageFilter{
				Type:      FilterTime,
				StartTime: &t,
			}, nil
		}
	}

	return MessageFilter{}, fmt.Errorf("invalid time format")
}

// ApplyFilters applies the current filters to a list of messages
func (fe *FilterEngine) ApplyFilters(messages []shared.Message) []shared.Message {
	if len(fe.activeFilters) == 0 {
		return messages
	}

	var filtered []shared.Message
	for _, msg := range messages {
		if fe.matchesFilters(msg) {
			filtered = append(filtered, msg)
		}
	}

	return filtered
}

// matchesFilters checks if a message matches all active filters
func (fe *FilterEngine) matchesFilters(msg shared.Message) bool {
	for _, filter := range fe.activeFilters {
		if !fe.matchesFilter(msg, filter) {
			return false
		}
	}
	return true
}

// matchesFilter checks if a message matches a specific filter
func (fe *FilterEngine) matchesFilter(msg shared.Message, filter MessageFilter) bool {
	switch filter.Type {
	case FilterUser:
		return strings.EqualFold(msg.Sender, filter.Value)

	case FilterFile:
		return msg.Type == shared.FileMessageType

	case FilterAdmin:
		return msg.Sender == "System" || strings.Contains(strings.ToLower(msg.Content), "admin")

	case FilterToday, FilterTime, FilterDuration:
		if filter.StartTime != nil && msg.CreatedAt.Before(*filter.StartTime) {
			return false
		}
		if filter.EndTime != nil && msg.CreatedAt.After(*filter.EndTime) {
			return false
		}
		return true

	default:
		return true
	}
}

// SetActiveFilters sets the active filters
func (fe *FilterEngine) SetActiveFilters(filters []MessageFilter) {
	fe.activeFilters = filters
}

// GetActiveFilters returns the current active filters
func (fe *FilterEngine) GetActiveFilters() []MessageFilter {
	return fe.activeFilters
}

// ClearFilters clears all active filters
func (fe *FilterEngine) ClearFilters() {
	fe.activeFilters = make([]MessageFilter, 0)
}

// GetFilterDescription returns a human-readable description of the filters
func (fe *FilterEngine) GetFilterDescription() string {
	if len(fe.activeFilters) == 0 {
		return "No filters active"
	}

	var descriptions []string
	for _, filter := range fe.activeFilters {
		descriptions = append(descriptions, fe.getFilterDescription(filter))
	}

	return "Active filters: " + strings.Join(descriptions, ", ")
}

// getFilterDescription returns a description for a single filter
func (fe *FilterEngine) getFilterDescription(filter MessageFilter) string {
	switch filter.Type {
	case FilterUser:
		return fmt.Sprintf("user:@%s", filter.Value)
	case FilterFile:
		return "file messages only"
	case FilterAdmin:
		return "admin messages only"
	case FilterToday:
		return "today"
	case FilterTime:
		if filter.StartTime != nil && filter.EndTime != nil {
			return fmt.Sprintf("time:%s to %s",
				filter.StartTime.Format("15:04"),
				filter.EndTime.Format("15:04"))
		} else if filter.StartTime != nil {
			return fmt.Sprintf("time:after %s", filter.StartTime.Format("15:04"))
		}
		return "time filter"
	case FilterDuration:
		if filter.StartTime != nil {
			duration := time.Since(*filter.StartTime)
			return fmt.Sprintf("last %s", duration.Round(time.Minute))
		}
		return "duration filter"
	default:
		return "unknown filter"
	}
}
