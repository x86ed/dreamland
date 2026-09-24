package cmd

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Pure output parsers shared by the unix and windows process-control files. They live in
// an untagged file so their table-driven tests run on every platform.

// parseLsofPIDs parses `lsof -nP -iTCP:<port> -sTCP:LISTEN -t` output into distinct,
// ascending pids. Non-numeric output is an error (unparseable).
func parseLsofPIDs(out string) ([]int, error) {
	seen := map[int]bool{}
	var pids []int
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pid, err := strconv.Atoi(line)
		if err != nil || pid <= 0 {
			return nil, fmt.Errorf("unparseable lsof output line %q", line)
		}
		if !seen[pid] {
			seen[pid] = true
			pids = append(pids, pid)
		}
	}
	sort.Ints(pids)
	return pids, nil
}

// parseNetstatPIDs extracts the distinct pids of LISTENING TCP sockets whose local port is
// exactly port from `netstat -ano` output.
func parseNetstatPIDs(out string, port int) []int {
	want := strconv.Itoa(port)
	seen := map[int]bool{}
	var pids []int
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) < 5 || !strings.EqualFold(f[0], "TCP") || !strings.EqualFold(f[3], "LISTENING") {
			continue
		}
		local := f[1]
		i := strings.LastIndex(local, ":")
		if i < 0 || local[i+1:] != want {
			continue
		}
		pid, err := strconv.Atoi(f[4])
		if err != nil || pid <= 0 || seen[pid] {
			continue
		}
		seen[pid] = true
		pids = append(pids, pid)
	}
	sort.Ints(pids)
	return pids
}

// parsePSCommandLine combines `ps -o comm=` (comm) and `ps -o args=` (argsLine) into the
// executable path and its arguments (excluding the executable itself). comm is the
// authoritative executable (a full path on macOS, the short name on Linux); argsLine is
// used only for the arguments, after removing argv[0].
func parsePSCommandLine(comm, argsLine string) (string, []string, error) {
	exe := strings.TrimSpace(comm)
	if exe == "" {
		return "", nil, errors.New("empty executable name")
	}
	line := strings.TrimSpace(argsLine)

	rest := ""
	if idx := argv0End(line, exe); idx >= 0 {
		rest = line[idx:]
	} else if i := strings.IndexAny(line, " \t"); i >= 0 {
		rest = line[i:]
	}
	return exe, strings.Fields(rest), nil
}

// argv0End returns the index just after argv[0] in line, where argv[0] ends with exe
// (exe may be the whole argv[0], as on macOS, or only its last path element, as on Linux).
// It returns -1 when no such boundary is found.
func argv0End(line, exe string) int {
	for from := 0; ; {
		i := strings.Index(line[from:], exe)
		if i < 0 {
			return -1
		}
		start, end := from+i, from+i+len(exe)
		startsAtElement := start == 0 || line[start-1] == '/' || line[start-1] == '\\'
		endsAtBoundary := end == len(line) || line[end] == ' ' || line[end] == '\t'
		if startsAtElement && endsAtBoundary {
			return end
		}
		from = start + 1
	}
}

// splitWindowsCommandLine splits a Windows command line into its arguments (excluding the
// program name, argv[0]), honouring double quotes.
func splitWindowsCommandLine(cmdline string) []string {
	var args []string
	var cur strings.Builder
	inQuotes, has := false, false
	flush := func() {
		if has {
			args = append(args, cur.String())
			cur.Reset()
			has = false
		}
	}
	for _, r := range cmdline {
		switch {
		case r == '"':
			inQuotes = !inQuotes
			has = true
		case (r == ' ' || r == '\t') && !inQuotes:
			flush()
		default:
			cur.WriteRune(r)
			has = true
		}
	}
	flush()
	if len(args) <= 1 {
		return nil
	}
	return args[1:]
}
