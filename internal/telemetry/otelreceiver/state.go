package otelreceiver

// STUB (nyx, TDD red phase): morpheus replaces this file with the real implementation
// per openspec/changes/parallel-session-otel-receiver tasks 1.1.

// StateDir returns the per-user receiver state directory.
func StateDir() string { return "" }

// ValidConversationID reports whether id is safe to use as a mailbox file name.
func ValidConversationID(id string) bool { return false }
