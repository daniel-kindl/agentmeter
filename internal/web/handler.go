package web

import (
	"net/http"

	assets "github.com/daniel-kindl/agentmeter/web"
)

// Handler returns an HTTP handler for the embedded dashboard assets.
func Handler() http.Handler {
	return http.FileServer(http.FS(assets.Files))
}
