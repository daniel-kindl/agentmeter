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
	LiveRoute      = "/api/v1/limits/live"
)

// Routes returns every API path this handler serves.
func Routes() []string { return []string{DashboardRoute, LimitsRoute, LiveRoute} }

// sameOrigin rejects a state change requested by a page other than the
// dashboard. The listener is on loopback, but any site the operator visits can
// reach loopback too, and this route turns on credential reads and network
// egress. A missing Origin is a non-browser client such as curl.
func sameOrigin(request *http.Request) bool {
	origin := request.Header.Get("Origin")
	if origin == "" {
		return true
	}
	return origin == "http://"+request.Host || origin == "https://"+request.Host
}

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
	mux.HandleFunc("PUT "+LiveRoute, func(writer http.ResponseWriter, request *http.Request) {
		if limitService == nil || !limitService.Configurable() {
			http.Error(writer, `{"error":"no limit providers are configured"}`, http.StatusServiceUnavailable)
			return
		}
		if !sameOrigin(request) {
			http.Error(writer, `{"error":"cross-origin request rejected"}`, http.StatusForbidden)
			return
		}
		var body struct {
			Enabled *bool `json:"enabled"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 1<<10)).Decode(&body); err != nil || body.Enabled == nil {
			http.Error(writer, `{"error":"expected {\"enabled\": true|false}"}`, http.StatusBadRequest)
			return
		}
		if err := limitService.SetLive(request.Context(), *body.Enabled); err != nil {
			http.Error(writer, `{"error":"could not store the preference"}`, http.StatusInternalServerError)
			return
		}
		// Reply with the report the switch produces, so the page renders the
		// consequence of the change rather than optimistically guessing it.
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
