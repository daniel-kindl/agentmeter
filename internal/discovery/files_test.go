package discovery_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/daniel-kindl/agentmeter/internal/discovery"
)

func TestClaudeDiscoversJSONLRecursively(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeSynthetic(t, filepath.Join(root, "projects", "project", "session.jsonl"))
	writeSynthetic(t, filepath.Join(root, "projects", "project", "ignore.txt"))
	files, err := discovery.Claude([]string{root})
	if err != nil {
		t.Fatalf("discover Claude: %v", err)
	}
	if len(files) != 1 || files[0].SessionID != "session" || files[0].Source != "claude" {
		t.Fatalf("files = %+v", files)
	}
}

func TestCodexActiveCopyWins(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	active := filepath.Join(root, "sessions", "2026", "session.jsonl")
	writeSynthetic(t, active)
	writeSynthetic(t, filepath.Join(root, "archived_sessions", "2026", "session.jsonl"))
	writeSynthetic(t, filepath.Join(root, "archived_sessions", "archived.jsonl"))
	files, err := discovery.Codex(root)
	if err != nil {
		t.Fatalf("discover Codex: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("files = %+v, want two", files)
	}
	var foundActive bool
	for _, file := range files {
		if file.SessionID == "2026/session" {
			foundActive = file.Path == active
		}
	}
	if !foundActive {
		t.Fatalf("active copy not selected: %+v", files)
	}
}

func writeSynthetic(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}
	if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
}
