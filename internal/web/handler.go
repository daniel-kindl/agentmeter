package web

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/daniel-kindl/agentmeter/internal/report"
	"github.com/daniel-kindl/agentmeter/internal/store"
	assets "github.com/daniel-kindl/agentmeter/web"
)

// Handler returns an HTTP handler for the embedded dashboard assets.
func Handler(database *store.Store) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/dashboard", func(writer http.ResponseWriter, request *http.Request) {
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
		writer.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(writer).Encode(dashboard); err != nil {
			return
		}
	})
	mux.Handle("/", http.FileServer(http.FS(assets.Files)))
	return mux
}
