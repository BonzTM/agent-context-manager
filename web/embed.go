package web

import "embed"

//go:embed index.html memories.html status.html static
var Static embed.FS
