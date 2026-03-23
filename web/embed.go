package web

import "embed"

//go:embed index.html status.html static
var Static embed.FS
