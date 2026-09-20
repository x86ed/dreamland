package cmd

import "errors"

// STUBS (nyx, TDD red phase): pure output parsers shared by the unix and windows
// process-control files (tasks 4.1, 4.2, 4.3). They live in an untagged file so their
// table-driven tests run on every platform. morpheus implements them.

// parseLsofPIDs parses `lsof -nP -iTCP:<port> -sTCP:LISTEN -t` output into distinct,
// ascending pids. Non-numeric output is an error (unparseable).
func parseLsofPIDs(out string) ([]int, error) { return nil, errors.New("not implemented") }

// parseNetstatPIDs extracts the distinct pids of LISTENING TCP sockets whose local port is
// exactly port from `netstat -ano` output.
func parseNetstatPIDs(out string, port int) []int { return nil }

// parsePSCommandLine combines `ps -o comm=` (comm) and `ps -o args=` (argsLine) into the
// executable path and its arguments (excluding the executable itself).
func parsePSCommandLine(comm, argsLine string) (string, []string, error) {
	return "", nil, errors.New("not implemented")
}
