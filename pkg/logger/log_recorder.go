package logger

import (
	"fmt"

	"github.com/trussworks/sesh/pkg/domain"
)

// LogLine is a mock log line
type LogLine struct {
	Level   string
	Message string
	Fields  map[string]string
}

// EventLogger should match the EventLogger defined in sesh
type EventLogger interface {
	LogSeshEvent(message string, metadata map[string]string)
}

// LogRecorder is a log recorder for testing
type LogRecorder struct {
	EventLogger
	lines []LogLine
}

// NewLogRecorder constructs a LogRecorder
func NewLogRecorder(wrappedLogger EventLogger) LogRecorder {
	return LogRecorder{
		EventLogger: wrappedLogger,
	}
}

// RecordLine records and returns a new LogLine with its level, message, and fields.
func (r *LogRecorder) RecordLine(level string, message string, fields map[string]string) LogLine {
	newLine := LogLine{
		Level:   level,
		Message: message,
		Fields:  domain.LogFields{},
	}

	for k, v := range fields {
		newLine.Fields[k] = v
	}

	r.lines = append(r.lines, newLine)

	return newLine
}

func (r *LogRecorder) LogSeshEvent(message string, fields map[string]string) {
	r.RecordLine("N/A", message, fields)
	r.EventLogger.LogSeshEvent(message, fields)
}

// GetOnlyMatchingMessage returns singular LogLine that matches message or errors
func (r *LogRecorder) GetOnlyMatchingMessage(message string) (LogLine, error) {
	messages := r.MatchingMessages(message)
	if len(messages) != 1 {
		return LogLine{}, fmt.Errorf("Didn't find only one line for message: %s (%s) ", message, messages)
	}
	return messages[0], nil
}

// MatchingMessages compares message to LogLines to seek those LogLines that match on LogRecorder
func (r *LogRecorder) MatchingMessages(message string) []LogLine {
	matches := []LogLine{}
	for _, line := range r.lines {
		if line.Message == message {
			matches = append(matches, line)
		}
	}
	return matches
}

