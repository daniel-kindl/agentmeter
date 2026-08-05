package discovery

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// File describes one session log selected for scanning.
type File struct {
	Source    string
	Path      string
	SessionID string
}

// Claude returns JSONL files beneath the projects directory of each config root.
func Claude(configRoots []string) ([]File, error) {
	var files []File
	for _, root := range configRoots {
		projects := filepath.Join(root, "projects")
		if !directoryExists(projects) {
			continue
		}
		err := filepath.WalkDir(projects, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".jsonl") {
				return nil
			}
			files = append(files, File{Source: "claude", Path: path, SessionID: strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))})
			return nil
		})
		if err != nil {
			return nil, errors.New("discover Claude sessions")
		}
	}
	sortFiles(files)
	return files, nil
}

// Codex returns active and archived JSONL files, preferring the active copy
// when both trees contain the same relative path.
func Codex(configRoot string) ([]File, error) {
	selected := make(map[string]File)
	for _, directory := range []string{"archived_sessions", "sessions"} {
		root := filepath.Join(configRoot, directory)
		if !directoryExists(root) {
			continue
		}
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".jsonl") {
				return nil
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			key := filepath.ToSlash(relative)
			selected[key] = File{
				Source:    "codex",
				Path:      path,
				SessionID: strings.TrimSuffix(key, filepath.Ext(key)),
			}
			return nil
		})
		if err != nil {
			return nil, errors.New("discover Codex sessions")
		}
	}
	files := make([]File, 0, len(selected))
	for _, file := range selected {
		files = append(files, file)
	}
	sortFiles(files)
	return files, nil
}

func directoryExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func sortFiles(files []File) {
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
}
