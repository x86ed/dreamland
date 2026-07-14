---
name: openspec-apply-change
description: Legacy alias for /opsx:apply — implement tasks from an OpenSpec change. Use when the user wants to start implementing, continue implementation, or work through tasks.
license: MIT
compatibility: Requires openspec CLI.
metadata:
  author: openspec
  version: "1.0"
  generatedBy: "1.3.1"
---

This is a legacy name for `/opsx:apply`, kept so old invocations still resolve. It routes through `janus`, which chooses `nyx` (acceptance-test flow) or `morpheus` (direct-implementation flow) per task — the same two-flow decision `/opsx:apply` documents, not a single fixed target.

Invoke the `opsx:apply` skill with the same input and follow its instructions; do not duplicate its steps here.
