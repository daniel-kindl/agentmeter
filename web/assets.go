// Package web exposes the dashboard's embedded static assets.
package web

import "embed"

// Files contains the static dashboard assets rooted at this package.
//
//go:embed index.html app.js style.css
var Files embed.FS
