package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeGitDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git", "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestInstallCommitMsgHook_Fresh(t *testing.T) {
	root := makeGitDir(t)
	if err := InstallCommitMsgHook(root); err != nil {
		t.Fatalf("install: %v", err)
	}
	target := filepath.Join(root, ".git", "hooks", "commit-msg")
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(data), hookBegin) {
		t.Error("hook file should contain BEGIN marker")
	}
	if !strings.Contains(string(data), hookEnd) {
		t.Error("hook file should contain END marker")
	}
	info, _ := os.Stat(target)
	if info.Mode()&0o111 == 0 {
		t.Error("hook file should be executable")
	}
}

func TestInstallCommitMsgHook_AppendToExisting(t *testing.T) {
	root := makeGitDir(t)
	existing := "#!/bin/sh\necho hello\n"
	if err := os.WriteFile(filepath.Join(root, ".git", "hooks", "commit-msg"), []byte(existing), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := InstallCommitMsgHook(root); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(root, ".git", "hooks", "commit-msg"))
	s := string(data)
	if !strings.Contains(s, "echo hello") {
		t.Error("existing content should be preserved")
	}
	if !strings.Contains(s, hookBegin) {
		t.Error("dreamland block should be appended")
	}
}

func TestInstallCommitMsgHook_Idempotent(t *testing.T) {
	root := makeGitDir(t)
	if err := InstallCommitMsgHook(root); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(root, ".git", "hooks", "commit-msg"))
	if err := InstallCommitMsgHook(root); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join(root, ".git", "hooks", "commit-msg"))
	if string(before) != string(after) {
		t.Error("second install should not change the file")
	}
}

func TestUninstallCommitMsgHook(t *testing.T) {
	root := makeGitDir(t)
	if err := InstallCommitMsgHook(root); err != nil {
		t.Fatal(err)
	}
	if err := UninstallCommitMsgHook(root); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	target := filepath.Join(root, ".git", "hooks", "commit-msg")
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Error("hook file should be removed after uninstall (empty remaining content)")
	}
}

func TestUninstallCommitMsgHook_PreservesExistingContent(t *testing.T) {
	root := makeGitDir(t)
	if err := os.WriteFile(filepath.Join(root, ".git", "hooks", "commit-msg"), []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := InstallCommitMsgHook(root); err != nil {
		t.Fatal(err)
	}
	if err := UninstallCommitMsgHook(root); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".git", "hooks", "commit-msg"))
	if err != nil {
		t.Fatal("file should still exist with existing content")
	}
	if !strings.Contains(string(data), "echo hi") {
		t.Error("existing content should be preserved after uninstall")
	}
	if strings.Contains(string(data), hookBegin) {
		t.Error("dreamland block should be removed")
	}
}

func TestInstallCommitMsgHook_ReadOnlyHooksDir(t *testing.T) {
	root := t.TempDir()
	hooksDir := filepath.Join(root, ".git", "hooks")
	// Create with normal permissions first, then restrict.
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(hooksDir, 0o555); err != nil {
		t.Fatal(err)
	}
	// Restore before cleanup so t.TempDir can remove the directory.
	t.Cleanup(func() { os.Chmod(hooksDir, 0o755) })

	err := InstallCommitMsgHook(root)
	if err == nil {
		t.Error("expected error when hooks dir is read-only")
	}
}

func TestUninstallCommitMsgHook_ReadError(t *testing.T) {
	root := t.TempDir()
	hooksDir := filepath.Join(root, ".git", "hooks")
	os.MkdirAll(hooksDir, 0o755)
	hookPath := filepath.Join(hooksDir, "commit-msg")
	os.WriteFile(hookPath, []byte("content"), 0o000)
	t.Cleanup(func() { os.Chmod(hookPath, 0o644) })
	err := UninstallCommitMsgHook(root)
	if err == nil {
		t.Error("expected error for unreadable file")
	}
}

func TestUninstallCommitMsgHook_NoEndMarker(t *testing.T) {
	root := t.TempDir()
	hooksDir := filepath.Join(root, ".git", "hooks")
	os.MkdirAll(hooksDir, 0o755)
	// Write a file with BEGIN but no END — all content from BEGIN to EOF should be removed.
	content := "#!/bin/sh\n" + hookBegin + "\nsome content\n"
	os.WriteFile(filepath.Join(hooksDir, "commit-msg"), []byte(content), 0o755)
	if err := UninstallCommitMsgHook(root); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Remaining is just "#!/bin/sh" — file should be deleted.
	if _, err := os.Stat(filepath.Join(hooksDir, "commit-msg")); !os.IsNotExist(err) {
		t.Error("hook file should be removed when only shebang remains")
	}
}

func TestInstallCommitMsgHook_UnreadableExistingHook(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root; skip permission test")
	}
	root := t.TempDir()
	hooksDir := filepath.Join(root, ".git", "hooks")
	os.MkdirAll(hooksDir, 0o755)
	// Create an unreadable existing hook file.
	hookPath := filepath.Join(hooksDir, "commit-msg")
	os.WriteFile(hookPath, []byte("#!/bin/sh\nexisting"), 0o000)
	t.Cleanup(func() { os.Chmod(hookPath, 0o755) })
	err := InstallCommitMsgHook(root)
	if err == nil {
		t.Error("expected error when existing hook is unreadable")
	}
}
