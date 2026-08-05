package scanner_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/daniel-kindl/agentmeter/internal/discovery"
	"github.com/daniel-kindl/agentmeter/internal/scanner"
	"github.com/daniel-kindl/agentmeter/internal/store"
)

func TestScanPersistsBothSourcesIdempotently(t *testing.T) {
	t.Parallel()

	database, err := store.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	files := []discovery.File{
		{Source: "claude", Path: filepath.Join("..", "..", "testdata", "synthetic", "claude", "usage.jsonl"), SessionID: "claude-session"},
		{Source: "codex", Path: filepath.Join("..", "..", "testdata", "synthetic", "codex", "usage.jsonl"), SessionID: "codex-session"},
	}
	first, err := scanner.Scan(context.Background(), database, files)
	if err != nil {
		t.Fatalf("first scan: %v", err)
	}
	if first.Inserted != 7 || first.Claude.Emitted != 4 || first.Codex.Emitted != 3 {
		t.Fatalf("first result = %+v", first)
	}
	second, err := scanner.Scan(context.Background(), database, files)
	if err != nil {
		t.Fatalf("second scan: %v", err)
	}
	if second.Inserted != 0 || second.Duplicates != 7 {
		t.Fatalf("second result = %+v", second)
	}
}
