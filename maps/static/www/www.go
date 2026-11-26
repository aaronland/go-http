// Package www provides an embedded example web application for displaying maps.
package www

import (
	"embed"
)

// FS is the embedded filesystem containing the example application's CSS, JavaScript and HTML files.
//
//go:embed css/* javascript/* index.html
var FS embed.FS
