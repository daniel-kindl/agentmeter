package web

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/daniel-kindl/agentmeter/internal/limits"
	"github.com/daniel-kindl/agentmeter/internal/report"
	"github.com/daniel-kindl/agentmeter/internal/store"
	assets "github.com/daniel-kindl/agentmeter/web"
)

// The API paths the dashboard serves. The embedded page fetches these, and a
// test asserts the two sides stay in step rather than restating the strings.
const (
	DashboardRoute = "/api/v1/dashboard"
	LimitsRoute    = "/api/v1/limits"
)

// Routes returns every API path this handler serves.
func Routes() []string { return []string{DashboardRoute, LimitsRoute} }

// Handler returns an HTTP handler for the embedded dashboard assets.
//
// A nil limitService disables the limits endpoint, which is what serving
// without the live flag does.
func Handler(database *store.Store, limitService *limits.Service) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+DashboardRoute, func(writer http.ResponseWriter, request *http.Request) {
		if database == nil {
			http.Error(writer, `{"error":"database unavailable"}`, http.StatusServiceUnavailable)
			return
		}
		selectedRange := request.URL.Query().Get("range")
		if selectedRange == "" {
			selectedRange = "30d"
		}
		dashboard, err := report.BuildDashboard(request.Context(), database, selectedRange, time.Now(), time.Local)
		if err != nil {
			http.Error(writer, `{"error":"dashboard query failed"}`, http.StatusBadRequest)
			return
		}
		writeJSON(writer, dashboard)
	})
	mux.HandleFunc("GET "+LimitsRoute, func(writer http.ResponseWriter, request *http.Request) {
		if limitService == nil {
			http.Error(writer, `{"error":"limits unavailable"}`, http.StatusServiceUnavailable)
			return
		}
		usageLimits, err := limitService.Report(request.Context(), time.Now(), time.Local)
		if err != nil {
			http.Error(writer, `{"error":"limits query failed"}`, http.StatusInternalServerError)
			return
		}
		writeJSON(writer, usageLimits)
	})
	mux.Handle("/", http.FileServer(http.FS(assets.Files)))
	return mux
}

func writeJSON(writer http.ResponseWriter, value any) {
	writer.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		return
	}
}
