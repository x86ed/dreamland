package cmd

import "errors"

// blockingError marks an error as one that must cause dreamland to exit with
// code 2 — the only exit code Claude Code's hook lifecycle treats as blocking
// (see PreToolUse/Stop/SubagentStop hook semantics). Every other error keeps
// exiting 1, unchanged, so commands not named in this change (version-bump,
// transition-log, telemetry write) are not affected.
type blockingError struct{ err error }

func Blocking(err error) error {
	if err == nil {
		return nil
	}
	return &blockingError{err: err}
}

func (e *blockingError) Error() string { return e.err.Error() }
func (e *blockingError) Unwrap() error { return e.err }

func IsBlocking(err error) bool {
	var be *blockingError
	return errors.As(err, &be)
}
