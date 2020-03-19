package mock

import (
	"fmt"

	"github.com/trussworks/sesh/pkg/domain"
)

// Log Recorder

// LogLine is a mock log line
type LogLine struct {
	Level   string
	Message string
	Fields  domain.LogFields
}

// LogRecorder is a mock log recorder
type LogRecorder struct {
	domain.LogService
	lines   []LogLine
	globals domain.LogFields
}

func NewLogRecorder(service domain.LogService) LogRecorder {
	return LogRecorder{
		LogService: service,
	}
}

// RecordLine records and returns a new LogLine with its level, message, and fields.
func (r *LogRecorder) RecordLine(level string, message string, fields domain.LogFields) LogLine {
	newLine := LogLine{
		Level:   level,
		Message: message,
		Fields:  domain.LogFields{},
	}

	for k, v := range r.globals {
		newLine.Fields[k] = v
	}

	for k, v := range fields {
		newLine.Fields[k] = v
	}

	r.lines = append(r.lines, newLine)

	return newLine
}

// Info records new LogLine as INFO level
func (r *LogRecorder) Info(message string, fields domain.LogFields) {
	line := r.RecordLine("INFO", message, fields)
	r.LogService.Info(line.Message, line.Fields)
}

// AddField adds new fields to LogRecorder's globals field
func (r *LogRecorder) AddField(name string, value string) {
	if r.globals == nil {
		r.globals = domain.LogFields{}
	}
	r.globals[name] = value
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
