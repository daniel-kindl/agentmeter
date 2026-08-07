package web_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/daniel-kindl/agentmeter/internal/limits"
	"github.com/daniel-kindl/agentmeter/internal/source"
	"github.com/daniel-kindl/agentmeter/internal/store"
	internalweb "github.com/daniel-kindl/agentmeter/internal/web"
	assets "github.com/daniel-kindl/agentmeter/web"
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

			internalweb.Handler(nil, nil).ServeHTTP(recorder, request)

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
	internalweb.Handler(database, nil).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/dashboard?range=all", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"input_tokens":10`) {
		t.Fatalf("response = %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestLimitsAPI(t *testing.T) {
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
	handler := internalweb.Handler(database, &limits.Service{Store: database})
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/limits", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"mode":"estimated"`) || !strings.Contains(body, `"kind":"5h"`) {
		t.Fatalf("body = %s", body)
	}
}

// apiCall matches an API path the embedded page fetches, written as either a
// plain string or the head of a template literal carrying a query string.
var apiCall = regexp.MustCompile("[\"`](/api/v1/[\\w-]+)")

// The page and the mux agree on paths by convention only. Both lists are read
// from source rather than restated, so this fails on a one-sided rename and
// stays green when a path is renamed properly in both places.
func TestPageCallsOnlyRoutesTheHandlerServes(t *testing.T) {
	t.Parallel()

	script, err := assets.Files.ReadFile("app.js")
	if err != nil {
		t.Fatalf("read app.js: %v", err)
	}
	served := internalweb.Routes()

	called := map[string]bool{}
	for _, match := range apiCall.FindAllStringSubmatch(string(script), -1) {
		called[match[1]] = true
	}
	if len(called) == 0 {
		t.Fatal("no API calls found in app.js: the extraction pattern is stale and this test proves nothing")
	}
	for path := range called {
		if !slices.Contains(served, path) {
			t.Errorf("app.js fetches %s but the handler serves no such route", path)
		}
	}
	for _, path := range served {
		if !called[path] {
			t.Errorf("the handler serves %s but the dashboard never fetches it", path)
		}
	}
}

// Without the live flag there is no limit service, and the endpoint says so
// rather than pretending the account has no limits.
func TestLimitsAPIWithoutAService(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	internalweb.Handler(nil, nil).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/limits", nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", recorder.Code)
	}
}
