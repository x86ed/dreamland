---
name: openspec-propose
description: Legacy alias for /opsx:propose — propose a new change with all artifacts generated in one step. Use when the user wants to quickly describe what they want to build and get a complete proposal with design, specs, and tasks ready for implementation.
license: MIT
compatibility: Requires openspec CLI.
metadata:
  author: openspec
  version: "1.0"
  generatedBy: "1.3.1"
---

This is a legacy name for `/opsx:propose` (`.github/prompts/opsx-propose.prompt.md`), kept so old invocations still resolve. It routes to the same target — the `phantasos` agent — directly, not through Janus.

Invoke the `opsx-propose` prompt with the same input and follow its instructions; do not duplicate its steps here.
