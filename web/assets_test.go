package web_test

import (
	"strings"
	"testing"

	assets "github.com/daniel-kindl/agentmeter/web"
)

func TestFilesContainDashboardAssets(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"index.html", "app.js", "style.css"} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			contents, err := assets.Files.ReadFile(name)
			if err != nil {
				t.Fatalf("read embedded asset %q: %v", name, err)
			}
			if len(contents) == 0 {
				t.Fatalf("embedded asset %q is empty", name)
			}
		})
	}
}

// The limits panel is wired across three files, so a rename in one of them
// silently stops rendering. Assert the seams instead.
func TestLimitsPanelIsWiredIntoTheDashboard(t *testing.T) {
	t.Parallel()

	tests := []struct {
		asset string
		want  []string
	}{
		{asset: "index.html", want: []string{`id="limits"`}},
		{asset: "app.js", want: []string{"loadLimits", "/api/v1/limits", "renderLimitPanel"}},
		{asset: "style.css", want: []string{".meter-fill", ".origin-badge", ".limits"}},
	}

	for _, tt := range tests {
		t.Run(tt.asset, func(t *testing.T) {
			t.Parallel()
			contents, err := assets.Files.ReadFile(tt.asset)
			if err != nil {
				t.Fatalf("read embedded asset %q: %v", tt.asset, err)
			}
			for _, want := range tt.want {
				if !strings.Contains(string(contents), want) {
					t.Errorf("asset %q does not contain %q", tt.asset, want)
				}
			}
		})
	}
}
