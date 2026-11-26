// Package static contains the embedded Bootstrap CSS and JavaScript assets.
package static

import (
	"embed"
)

// FS is the embedded filesystem containing the Bootstrap CSS and JavaScript assets.
//
//go:embed css/* javascript/*
var FS embed.FS
