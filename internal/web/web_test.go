package web_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
		{name: "JavaScript", path: "/app.js", wantStatus: http.StatusOK, wantContent: "future change"},
		{name: "missing", path: "/missing.txt", wantStatus: http.StatusNotFound, wantContent: "404 page not found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, tt.path, nil)

			internalweb.Handler().ServeHTTP(recorder, request)

			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}
			if !strings.Contains(recorder.Body.String(), tt.wantContent) {
				t.Fatalf("body %q does not contain %q", recorder.Body.String(), tt.wantContent)
			}
		})
	}
}
