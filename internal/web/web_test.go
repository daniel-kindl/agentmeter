package web_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/daniel-kindl/agentmeter/internal/source"
	"github.com/daniel-kindl/agentmeter/internal/store"
	internalweb "github.com/daniel-kindl/agentmeter/internal/web"
)

func TestHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		path        string
		wantStatus  int
		wantContent string
	}{
		{name: "index", path: "/", wantStatus: http.StatusOK, wantContent: "agentmeter"},
		{name: "JavaScript", path: "/app.js", wantStatus: http.StatusOK, wantContent: "loadDashboard"},
		{name: "missing", path: "/missing.txt", wantStatus: http.StatusNotFound, wantContent: "404 page not found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, tt.path, nil)

			internalweb.Handler(nil).ServeHTTP(recorder, request)

			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}
			if !strings.Contains(recorder.Body.String(), tt.wantContent) {
				t.Fatalf("body %q does not contain %q", recorder.Body.String(), tt.wantContent)
			}
		})
	}
}

func TestDashboardAPI(t *testing.T) {
	t.Parallel()

	database, err := store.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	event := source.UsageEvent{Timestamp: time.Now().UTC(), Source: "claude", SessionID: "session", Model: "claude-sonnet-4", InputTokens: 10, DedupeKey: "event"}
	if _, err := database.InsertUsageEvents(context.Background(), []source.UsageEvent{event}); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	recorder := httptest.NewRecorder()
	internalweb.Handler(database).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/dashboard?range=all", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"input_tokens":10`) {
		t.Fatalf("response = %d %s", recorder.Code, recorder.Body.String())
	}
}
