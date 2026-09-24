package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"dreamland/internal/handoff"
)

// handoffCmdRepo points osGetwd at a fresh repo with an isolated handoff state dir.
func handoffCmdRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DREAMLAND_STATE_DIR", t.TempDir())
	old := osGetwd
	osGetwd = func() (string, error) { return repo, nil }
	t.Cleanup(func() { osGetwd = old })
	return repo
}

func runHandoffSub(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errOut)
	rootCmd.SetArgs(args)
	t.Cleanup(func() {
		rootCmd.SetArgs(nil)
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		handoffSession = ""
	})
	err := rootCmd.Execute()
	return out.String(), errOut.String(), err
}

func seedPending(t *testing.T, repo, session string) {
	t.Helper()
	err := handoff.NewStore(repo).UpdatePending(session, func([]handoff.Entry) []handoff.Entry {
		return []handoff.Entry{{Agent: "morpheus", Directive: handoff.Directive{Kind: "dispatch", Target: "phobetor"}}}
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestHandoffClearRemovesPendingEntries(t *testing.T) {
	repo := handoffCmdRepo(t)
	seedPending(t, repo, "s1")
	seedPending(t, repo, "s2")

	out, _, err := runHandoffSub(t, "handoff", "clear", "--session", "s1")
	if err != nil || !strings.Contains(out, "cleared: s1") || strings.Contains(out, "s2") {
		t.Fatalf("clear --session: %v %q", err, out)
	}
	if es, _ := handoff.NewStore(repo).ReadPending("s2"); len(es) != 1 {
		t.Errorf("s2 entries = %+v, want untouched", es)
	}

	handoffSession = ""
	out, _, err = runHandoffSub(t, "handoff", "clear")
	if err != nil || !strings.Contains(out, "cleared: s2") {
		t.Fatalf("clear all: %v %q", err, out)
	}
	if es, _ := handoff.NewStore(repo).ReadPending("s2"); len(es) != 0 {
		t.Errorf("s2 entries = %+v, want none", es)
	}
}

func TestHandoffClearRejectsInvalidSession(t *testing.T) {
	handoffCmdRepo(t)
	if _, _, err := runHandoffSub(t, "handoff", "clear", "--session", "../evil"); err == nil {
		t.Fatal("expected invalid session id error")
	}
}

func TestHandoffCommandsFailOutsideRepo(t *testing.T) {
	old := osGetwd
	osGetwd = func() (string, error) { return t.TempDir(), nil }
	t.Cleanup(func() { osGetwd = old })
	for _, args := range [][]string{{"handoff", "clear"}, {"status"}} {
		if _, _, err := runHandoffSub(t, args...); err == nil {
			t.Errorf("%v outside a repo: expected error", args)
		}
	}
}

func TestStatusCommandPrintsHandoffEntries(t *testing.T) {
	repo := handoffCmdRepo(t)
	out, _, err := runHandoffSub(t, "status")
	if err != nil || !strings.Contains(out, "handoff: no entries") {
		t.Fatalf("empty status: %v %q", err, out)
	}
	seedPending(t, repo, "s1")
	out, _, err = runHandoffSub(t, "status")
	if err != nil || !strings.Contains(out, "session=s1") || !strings.Contains(out, "target=phobetor") {
		t.Fatalf("status: %v %q", err, out)
	}
}

func TestHandoffStatusReportsUnreadableSession(t *testing.T) {
	e, _, _ := newHandoffEnv(t, "block")
	mustRecord(t, e, "morpheus", "[handoff: complete]")
	if err := os.WriteFile(filepath.Join(e.store.Dir, "pending", "s1.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	var w bytes.Buffer
	printHandoffStatus(e.store, &w)
	if !strings.Contains(w.String(), "unreadable state") {
		t.Errorf("status = %q", w.String())
	}
}

func TestActiveChangesParsesOpenspecList(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell stub")
	}
	bin := t.TempDir()
	stub := "#!/bin/sh\necho '{\"changes\":[{\"name\":\"c1\"},{\"name\":\"c2\"}]}'\n"
	if err := os.WriteFile(filepath.Join(bin, "openspec"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	got, err := activeChanges(t.TempDir())
	if err != nil || len(got) != 2 || got[0] != "c1" || got[1] != "c2" {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestActiveChangesErrorsWhenOpenspecMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := activeChanges(t.TempDir()); err == nil {
		t.Fatal("expected error when openspec is not on PATH")
	}
}
