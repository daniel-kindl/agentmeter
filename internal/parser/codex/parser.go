package codex

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

// FileMetadata contains identity derived from a Codex session file.
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

type rawUsage struct {
	InputTokens           int64 `json:"input_tokens"`
	CachedInputTokens     int64 `json:"cached_input_tokens"`
	OutputTokens          int64 `json:"output_tokens"`
	ReasoningOutputTokens int64 `json:"reasoning_output_tokens"`
	TotalTokens           int64 `json:"total_tokens"`
}

type record struct {
	Timestamp string  `json:"timestamp"`
	Type      string  `json:"type"`
	Payload   payload `json:"payload"`
}

type payload struct {
	Type           string         `json:"type"`
	Model          string         `json:"model"`
	ModelName      string         `json:"model_name"`
	Info           usageInfo      `json:"info"`
	ThreadSettings threadSettings `json:"thread_settings"`
}

type usageInfo struct {
	Model           string    `json:"model"`
	ModelName       string    `json:"model_name"`
	TotalTokenUsage *rawUsage `json:"total_token_usage"`
	LastTokenUsage  *rawUsage `json:"last_token_usage"`
}

type threadSettings struct {
	ServiceTier *string `json:"service_tier"`
}

// Parse streams one Codex session in strict file order.
func Parse(ctx context.Context, reader io.Reader, metadata FileMetadata, emit func(source.UsageEvent) error) (Stats, error) {
	var stats Stats
	if err := ctx.Err(); err != nil {
		return stats, err
	}

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, initialScannerBuffer), maxLineSize)
	var previous *rawUsage
	model := "unknown"
	rateClass := ""

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

		var value record
		if err := json.Unmarshal(line, &value); err != nil {
			stats.UnparsedLines++
			continue
		}
		switch {
		case value.Type == "turn_context":
			candidate := firstNonBlank(value.Payload.Model, value.Payload.ModelName)
			if candidate == "" {
				stats.UnparsedLines++
			} else {
				model = candidate
				stats.Ignored++
			}
			continue
		case value.Type == "event_msg" && value.Payload.Type == "thread_settings_applied":
			if value.Payload.ThreadSettings.ServiceTier != nil {
				rateClass = normalizeRateClass(*value.Payload.ThreadSettings.ServiceTier)
			}
			stats.Ignored++
			continue
		case value.Type != "event_msg" || value.Payload.Type != "token_count":
			stats.Ignored++
			continue
		}

		timestamp, err := time.Parse(time.RFC3339Nano, value.Timestamp)
		if err != nil || strings.TrimSpace(metadata.SessionID) == "" {
			stats.UnparsedLines++
			continue
		}
		candidateModel := firstNonBlank(value.Payload.Model, value.Payload.ModelName, value.Payload.Info.Model, value.Payload.Info.ModelName)
		if candidateModel != "" {
			model = candidateModel
		}

		advanced := value.Payload.Info.TotalTokenUsage == nil || previous == nil || *value.Payload.Info.TotalTokenUsage != *previous
		var usage *rawUsage
		if advanced && value.Payload.Info.LastTokenUsage != nil {
			usageCopy := *value.Payload.Info.LastTokenUsage
			usage = &usageCopy
		} else if value.Payload.Info.TotalTokenUsage != nil {
			usageCopy := subtract(*value.Payload.Info.TotalTokenUsage, previous)
			usage = &usageCopy
		}
		if value.Payload.Info.TotalTokenUsage != nil {
			totalCopy := *value.Payload.Info.TotalTokenUsage
			previous = &totalCopy
		}
		if usage == nil {
			stats.Ignored++
			continue
		}
		if invalidUsage(*usage) {
			stats.UnparsedLines++
			continue
		}
		cached := min(usage.CachedInputTokens, usage.InputTokens)
		if usage.InputTokens == 0 && usage.OutputTokens == 0 && cached == 0 {
			stats.Ignored++
			continue
		}
		event := source.UsageEvent{
			Timestamp:            timestamp.UTC(),
			Source:               "codex",
			SessionID:            metadata.SessionID,
			Model:                model,
			InputTokens:          usage.InputTokens - cached,
			OutputTokens:         usage.OutputTokens,
			CacheReadInputTokens: cached,
			DedupeKey:            fmt.Sprintf("%s:%d", metadata.SessionID, stats.Lines),
			RateClass:            rateClass,
		}
		if err := emit(event); err != nil {
			return stats, fmt.Errorf("emit Codex usage event at line %d: %w", stats.Lines, err)
		}
		stats.Emitted++
	}
	if err := scanner.Err(); err != nil {
		return stats, fmt.Errorf("scan Codex JSONL: %w", err)
	}
	return stats, nil
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func normalizeRateClass(value string) string {
	switch strings.TrimSpace(value) {
	case "default", "standard":
		return "standard"
	case "fast", "priority":
		return "fast"
	default:
		return ""
	}
}

func subtract(current rawUsage, previous *rawUsage) rawUsage {
	if previous == nil {
		return current
	}
	return rawUsage{
		InputTokens:           max(0, current.InputTokens-previous.InputTokens),
		CachedInputTokens:     max(0, current.CachedInputTokens-previous.CachedInputTokens),
		OutputTokens:          max(0, current.OutputTokens-previous.OutputTokens),
		ReasoningOutputTokens: max(0, current.ReasoningOutputTokens-previous.ReasoningOutputTokens),
		TotalTokens:           max(0, current.TotalTokens-previous.TotalTokens),
	}
}

func invalidUsage(usage rawUsage) bool {
	return usage.InputTokens < 0 || usage.CachedInputTokens < 0 || usage.OutputTokens < 0 ||
		usage.ReasoningOutputTokens < 0 || usage.TotalTokens < 0
}
