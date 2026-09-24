package otelreceiver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStateDir_EnvOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DREAMLAND_STATE_DIR", dir)
	if got := StateDir(); got != dir {
		t.Errorf("StateDir() = %q, want %q (DREAMLAND_STATE_DIR)", got, dir)
	}
}

func TestStateDir_DefaultsToUserCacheDir(t *testing.T) {
	t.Setenv("DREAMLAND_STATE_DIR", "")
	cache, err := os.UserCacheDir()
	if err != nil {
		t.Skipf("no user cache dir on this host: %v", err)
	}
	want := filepath.Join(cache, "dreamland", "otel")
	if got := StateDir(); got != want {
		t.Errorf("StateDir() = %q, want %q", got, want)
	}
}

func TestStateDir_FallsBackToTempDirWhenUserCacheDirErrors(t *testing.T) {
	t.Setenv("DREAMLAND_STATE_DIR", "")
	// os.UserCacheDir consults XDG_CACHE_HOME then $HOME on unix/darwin; clearing both
	// makes it error. On other platforms it cannot be forced to fail.
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "")
	if _, err := os.UserCacheDir(); err == nil {
		t.Skip("os.UserCacheDir cannot be made to fail on this platform")
	}
	want := filepath.Join(os.TempDir(), "dreamland-otel")
	if got := StateDir(); got != want {
		t.Errorf("StateDir() = %q, want %q", got, want)
	}
}

func TestValidConversationID(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"sess-123", true},
		{"3f2a9c1e-7b1d-4e0a-9c55-0a1b2c3d4e5f", true},
		{"a.b_c-d", true},
		{"A", true},
		{strings.Repeat("a", 128), true},
		{"..a", true},
		{"a..", true},

		{"", false},
		{".", false},
		{"..", false},
		{"../x", false},
		{"../../evil", false},
		{"a/b", false},
		{`a\b`, false},
		{"a:b", false},
		{"a b", false},
		{"a\x00b", false},
		{"a\nb", false},
		{"café", false},
		{strings.Repeat("a", 129), false},
	}
	for _, tt := range tests {
		name := tt.id
		if len(name) > 20 {
			name = name[:20] + "..."
		}
		t.Run(name, func(t *testing.T) {
			if got := ValidConversationID(tt.id); got != tt.want {
				t.Errorf("ValidConversationID(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}
