package cmd

import (
	"testing"
)

func TestHasMatchingFiles_Go(t *testing.T) {
	status := " M cmd/foo.go\n M README.md\n"
	if !hasMatchingFiles(status, []string{".go"}) {
		t.Error("expected true for .go file")
	}
	if hasMatchingFiles(" M README.md\n", []string{".go"}) {
		t.Error("expected false when no .go files")
	}
}

func TestHasMatchingFiles_TypeScript(t *testing.T) {
	exts := sourceExtensions["Node/TypeScript"]
	cases := []struct {
		line string
		want bool
	}{
		{" M src/app.ts\n", true},
		{" M src/comp.tsx\n", true},
		{" M src/util.js\n", true},
		{" M src/mod.mts\n", true},
		{" M README.md\n", false},
		{" M styles.css\n", false},
	}
	for _, tc := range cases {
		got := hasMatchingFiles(tc.line, exts)
		if got != tc.want {
			t.Errorf("hasMatchingFiles(%q) = %v, want %v", tc.line, got, tc.want)
		}
	}
}

func TestHasMatchingFiles_Rust(t *testing.T) {
	exts := sourceExtensions["Rust"]
	if !hasMatchingFiles(" M src/main.rs\n", exts) {
		t.Error("expected true for .rs file")
	}
	if hasMatchingFiles(" M Cargo.toml\n", exts) {
		t.Error("expected false for Cargo.toml")
	}
}

func TestHasMatchingFiles_Python(t *testing.T) {
	exts := sourceExtensions["Python"]
	if !hasMatchingFiles(" M app.py\n", exts) {
		t.Error("expected true for .py file")
	}
	if hasMatchingFiles(" M requirements.txt\n", exts) {
		t.Error("expected false for .txt file")
	}
}

func TestHasMatchingFiles_Empty(t *testing.T) {
	if hasMatchingFiles("", []string{".go"}) {
		t.Error("expected false for empty status")
	}
}

func TestHasMatchingFiles_MultipleFiles(t *testing.T) {
	status := " M README.md\n M styles.css\n M main.go\n"
	if !hasMatchingFiles(status, []string{".go"}) {
		t.Error("expected true when one of multiple files matches")
	}
}
