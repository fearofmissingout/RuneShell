package content

import "embed"

//go:embed assets/*.json
var embeddedFiles embed.FS
