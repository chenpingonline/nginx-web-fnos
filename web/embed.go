package web

import "embed"

// Assets contains the frontend files served by the management API.
//
//go:embed index.html app.js styles.css
var Assets embed.FS
