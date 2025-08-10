package web

import (
	"embed"
)

// Static contains the embedded web UI assets.
//go:embed index.html styles.css app.js reference.html commit.html
var Static embed.FS
