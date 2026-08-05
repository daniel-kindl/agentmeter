package web_test

import (
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
