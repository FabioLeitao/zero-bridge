package web

import "embed"

// Files holds the static chat UI.
//
//go:embed index.html app.js style.css
var Files embed.FS
