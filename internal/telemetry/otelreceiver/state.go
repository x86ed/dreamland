package otelreceiver

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

var conversationIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

// StateDir returns the per-user receiver state directory: $DREAMLAND_STATE_DIR if set,
// else <os.UserCacheDir()>/dreamland/otel, else <os.TempDir()>/dreamland-otel. The
// receiver and the Copilot collector must resolve the same directory.
func StateDir() string {
	if dir := os.Getenv("DREAMLAND_STATE_DIR"); dir != "" {
		return dir
	}
	if cache, err := os.UserCacheDir(); err == nil {
		return filepath.Join(cache, "dreamland", "otel")
	}
	return filepath.Join(os.TempDir(), "dreamland-otel")
}

// ValidConversationID reports whether id is safe to use as a mailbox file name.
func ValidConversationID(id string) bool {
	return id != "." && id != ".." && conversationIDPattern.MatchString(id)
}

// SessionsDir is where receiver-owned mailboxes live.
func SessionsDir(stateDir string) string { return filepath.Join(stateDir, "sessions") }

// LogPath is the receiver request log.
func LogPath(stateDir string) string { return filepath.Join(stateDir, "receiver.log") }

// PidFilePath is the advisory pid file for the receiver on port.
func PidFilePath(stateDir string, port int) string {
	return filepath.Join(stateDir, fmt.Sprintf("receiver-%d.json", port))
}

// LockPath is the O_EXCL start lock for the receiver on port.
func LockPath(stateDir string, port int) string {
	return filepath.Join(stateDir, fmt.Sprintf("receiver-%d.lock", port))
}
