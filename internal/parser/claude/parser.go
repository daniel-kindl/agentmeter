package claude

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/daniel-kindl/agentmeter/internal/source"
)

const (
	initialScannerBuffer = 64 * 1024
	maxLineSize          = 64 * 1024 * 1024
)

// FileMetadata contains values derived from the file containing a session.
type FileMetadata struct {
	SessionID string
}

// Stats summarizes how the parser classified the lines it read.
type Stats struct {
	Lines         int64
	Emitted       int64
	Ignored       int64
	UnparsedLines int64
}

// Parse streams JSONL records from reader and calls emit for every valid usage event.
// Duplicate records are emitted; persistence is responsible for deduplication.
func Parse(
	ctx context.Context,
	reader io.Reader,
	metadata FileMetadata,
	emit func(source.UsageEvent) error,
) (Stats, error) {
	var stats Stats
	if err := ctx.Err(); err != nil {
		return stats, err
	}

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, initialScannerBuffer), maxLineSize)

	for {
		if err := ctx.Err(); err != nil {
			return stats, err
		}
		if !scanner.Scan() {
			break
		}
		stats.Lines++

		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			stats.Ignored++
			continue
		}

		event, classification := parseLine(line, metadata)
		switch classification {
		case lineIgnored:
			stats.Ignored++
		case lineUnparsed:
			stats.UnparsedLines++
		case lineUsage:
			if err := emit(event); err != nil {
				return stats, fmt.Errorf("emit Claude usage event at line %d: %w", stats.Lines, err)
			}
			stats.Emitted++
		}
	}

	if err := scanner.Err(); err != nil {
		return stats, fmt.Errorf("scan Claude JSONL: %w", err)
	}
	return stats, nil
}

type lineClassification uint8

const (
	lineIgnored lineClassification = iota
	lineUnparsed
	lineUsage
)

type envelope struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	SessionID string          `json:"sessionId"`
	RequestID string          `json:"requestId"`
	Message   json.RawMessage `json:"message"`
	Data      json.RawMessage `json:"data"`
}

type progressData struct {
	Type    string          `json:"type"`
	Message json.RawMessage `json:"message"`
}

type assistantMessage struct {
	ID    string          `json:"id"`
	Model string          `json:"model"`
	Usage json.RawMessage `json:"usage"`
}

type tokenUsage struct {
	InputTokens              *int64 `json:"input_tokens"`
	OutputTokens             *int64 `json:"output_tokens"`
	CacheCreationInputTokens *int64 `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     *int64 `json:"cache_read_input_tokens"`
}

func parseLine(line []byte, metadata FileMetadata) (source.UsageEvent, lineClassification) {
	var record envelope
	if err := json.Unmarshal(line, &record); err != nil {
		return source.UsageEvent{}, lineUnparsed
	}

	switch record.Type {
	case "assistant":
		return parseAssistant(record, metadata)
	case "progress":
		return parseProgress(record, metadata)
	default:
		if containsTokenUsage(line) {
			return source.UsageEvent{}, lineUnparsed
		}
		return source.UsageEvent{}, lineIgnored
	}
}

func parseProgress(outer envelope, metadata FileMetadata) (source.UsageEvent, lineClassification) {
	var data progressData
	if len(outer.Data) == 0 || json.Unmarshal(outer.Data, &data) != nil {
		if containsTokenUsage(outer.Data) {
			return source.UsageEvent{}, lineUnparsed
		}
		return source.UsageEvent{}, lineIgnored
	}
	if data.Type != "agent_progress" {
		if containsTokenUsage(outer.Data) {
			return source.UsageEvent{}, lineUnparsed
		}
		return source.UsageEvent{}, lineIgnored
	}

	var nested envelope
	if len(data.Message) == 0 || json.Unmarshal(data.Message, &nested) != nil {
		if containsTokenUsage(outer.Data) {
			return source.UsageEvent{}, lineUnparsed
		}
		return source.UsageEvent{}, lineIgnored
	}
	if nested.Type != "assistant" {
		if containsTokenUsage(data.Message) {
			return source.UsageEvent{}, lineUnparsed
		}
		return source.UsageEvent{}, lineIgnored
	}

	if nested.Timestamp == "" {
		nested.Timestamp = outer.Timestamp
	}
	if nested.SessionID == "" {
		nested.SessionID = outer.SessionID
	}
	return parseAssistant(nested, metadata)
}

func parseAssistant(record envelope, metadata FileMetadata) (source.UsageEvent, lineClassification) {
	if len(record.Message) == 0 {
		return source.UsageEvent{}, lineIgnored
	}

	var message assistantMessage
	if err := json.Unmarshal(record.Message, &message); err != nil {
		if containsTokenUsage(record.Message) {
			return source.UsageEvent{}, lineUnparsed
		}
		return source.UsageEvent{}, lineIgnored
	}
	if len(message.Usage) == 0 || bytes.Equal(bytes.TrimSpace(message.Usage), []byte("null")) {
		return source.UsageEvent{}, lineIgnored
	}

	var usage tokenUsage
	if err := json.Unmarshal(message.Usage, &usage); err != nil {
		return source.UsageEvent{}, lineUnparsed
	}

	timestamp, err := time.Parse(time.RFC3339Nano, record.Timestamp)
	if err != nil || strings.TrimSpace(message.ID) == "" || strings.TrimSpace(record.RequestID) == "" ||
		strings.TrimSpace(message.Model) == "" || usage.InputTokens == nil || usage.OutputTokens == nil {
		return source.UsageEvent{}, lineUnparsed
	}

	sessionID := record.SessionID
	if strings.TrimSpace(sessionID) == "" {
		sessionID = metadata.SessionID
	}
	if strings.TrimSpace(sessionID) == "" || hasNegativeTokenCount(usage) {
		return source.UsageEvent{}, lineUnparsed
	}

	return source.UsageEvent{
		Timestamp:                timestamp.UTC(),
		Source:                   "claude",
		SessionID:                sessionID,
		Model:                    message.Model,
		InputTokens:              *usage.InputTokens,
		OutputTokens:             *usage.OutputTokens,
		CacheCreationInputTokens: optionalTokenCount(usage.CacheCreationInputTokens),
		CacheReadInputTokens:     optionalTokenCount(usage.CacheReadInputTokens),
		DedupeKey:                message.ID + ":" + record.RequestID,
	}, lineUsage
}

func optionalTokenCount(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func hasNegativeTokenCount(usage tokenUsage) bool {
	return *usage.InputTokens < 0 || *usage.OutputTokens < 0 ||
		optionalTokenCount(usage.CacheCreationInputTokens) < 0 ||
		optionalTokenCount(usage.CacheReadInputTokens) < 0
}

func containsTokenUsage(data []byte) bool {
	var value any
	if len(data) == 0 || json.Unmarshal(data, &value) != nil {
		return false
	}
	return findTokenUsage(value)
}

func findTokenUsage(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if key == "usage" {
				if usage, ok := child.(map[string]any); ok {
					for field := range usage {
						if field == "input_tokens" || field == "output_tokens" ||
							field == "cache_creation_input_tokens" || field == "cache_read_input_tokens" {
							return true
						}
					}
				}
			}
			if findTokenUsage(child) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if findTokenUsage(child) {
				return true
			}
		}
	}
	return false
}
