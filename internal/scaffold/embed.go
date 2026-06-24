package scaffold

import (
	"embed"
	"io/fs"
)

//go:embed all:templates
var templateEmbedFS embed.FS

// TemplateFS is the embedded filesystem containing all agent and hook binding templates.
var TemplateFS fs.FS = templateEmbedFS
