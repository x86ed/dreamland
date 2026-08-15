package workflowgraph

import "embed"

// StaticAssets embeds the browser UI served by `dreamland hypnos-serve`.
// Currently a placeholder (see static/index.html) — the real litegraph.js
// graph editor is vendored in tasks.md §9, built after the server/backend
// layers this embeds alongside.
//
//go:embed static
var StaticAssets embed.FS
