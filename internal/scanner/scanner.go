package scanner

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/daniel-kindl/agentmeter/internal/discovery"
	claudeparser "github.com/daniel-kindl/agentmeter/internal/parser/claude"
	codexparser "github.com/daniel-kindl/agentmeter/internal/parser/codex"
	"github.com/daniel-kindl/agentmeter/internal/source"
	"github.com/daniel-kindl/agentmeter/internal/store"
)

const batchSize = 1000

// SourceStats summarizes one source in a completed scan.
type SourceStats struct {
	Files         int64 `json:"files"`
	Lines         int64 `json:"lines"`
	Emitted       int64 `json:"emitted"`
	Ignored       int64 `json:"ignored"`
	UnparsedLines int64 `json:"unparsed_lines"`
}

// Result summarizes discovery, parsing, and persistence.
type Result struct {
	Claude     SourceStats `json:"claude"`
	Codex      SourceStats `json:"codex"`
	Inserted   int         `json:"inserted"`
	Updated    int         `json:"updated"`
	Duplicates int         `json:"duplicates"`
}

// Scan streams files through their source parser and stores bounded batches.
func Scan(ctx context.Context, database *store.Store, files []discovery.File) (Result, error) {
	var result Result
	batch := make([]source.UsageEvent, 0, batchSize)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		inserted, err := database.InsertUsageEvents(ctx, batch)
		if err != nil {
			return err
		}
		result.Inserted += inserted.Inserted
		result.Updated += inserted.Updated
		result.Duplicates += inserted.Duplicates
		batch = batch[:0]
		return nil
	}
	emit := func(event source.UsageEvent) error {
		batch = append(batch, event)
		if len(batch) == batchSize {
			return flush()
		}
		return nil
	}

	for _, selected := range files {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		file, err := os.Open(selected.Path)
		if err != nil {
			return result, errors.New("open session log")
		}
		switch selected.Source {
		case "claude":
			result.Claude.Files++
			stats, parseErr := claudeparser.Parse(ctx, file, claudeparser.FileMetadata{SessionID: selected.SessionID}, emit)
			result.Claude.Lines += stats.Lines
			result.Claude.Emitted += stats.Emitted
			result.Claude.Ignored += stats.Ignored
			result.Claude.UnparsedLines += stats.UnparsedLines
			err = parseErr
		case "codex":
			result.Codex.Files++
			stats, parseErr := codexparser.Parse(ctx, file, codexparser.FileMetadata{SessionID: selected.SessionID}, emit)
			result.Codex.Lines += stats.Lines
			result.Codex.Emitted += stats.Emitted
			result.Codex.Ignored += stats.Ignored
			result.Codex.UnparsedLines += stats.UnparsedLines
			err = parseErr
		default:
			err = fmt.Errorf("unsupported source %q", selected.Source)
		}
		closeErr := file.Close()
		if err != nil {
			return result, err
		}
		if closeErr != nil {
			return result, errors.New("close session log")
		}
	}
	if err := flush(); err != nil {
		return result, fmt.Errorf("store usage batch: %w", err)
	}
	return result, nil
}
