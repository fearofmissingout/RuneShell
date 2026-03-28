package content

import "embed"

//go:embed assets/*.json locales/*.json
var embeddedFiles embed.FS
