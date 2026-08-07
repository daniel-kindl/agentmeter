package web_test

import (
	"fmt"
	"regexp"
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

// elementSelector matches an id selector written as a string literal, which is
// how app.js addresses the markup.
var elementSelector = regexp.MustCompile(`"#([A-Za-z][\w-]*)"`)

// Nothing connects the ids index.html declares to the ids app.js looks up: a
// rename on one side compiles, serves, and silently renders nothing. Both sides
// are read out of the files rather than restated here, so renaming in both
// places stays green and renaming in one does not.
func TestQueriedElementsExistInTheMarkup(t *testing.T) {
	t.Parallel()

	script := readAsset(t, "app.js")
	markup := readAsset(t, "index.html")

	matches := elementSelector.FindAllStringSubmatch(script, -1)
	if len(matches) == 0 {
		t.Fatal("no id selectors found in app.js: the extraction pattern is stale and this test proves nothing")
	}
	for _, match := range matches {
		if !strings.Contains(markup, fmt.Sprintf("id=%q", match[1])) {
			t.Errorf("app.js addresses #%s but index.html declares no such element", match[1])
		}
	}
}

func readAsset(t *testing.T, name string) string {
	t.Helper()
	contents, err := assets.Files.ReadFile(name)
	if err != nil {
		t.Fatalf("read embedded asset %q: %v", name, err)
	}
	return string(contents)
}
