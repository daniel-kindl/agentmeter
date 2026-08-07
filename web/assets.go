// Package web exposes the dashboard's embedded static assets.
package web

import "embed"

// Files contains the static dashboard assets rooted at this package.
//
// Fonts are embedded rather than fetched from a font CDN. A dashboard about
// private usage data must not announce itself to a third party to render, and
// the binary is meant to run with no network at all.
//
//go:embed index.html app.js style.css fonts
var Files embed.FS
