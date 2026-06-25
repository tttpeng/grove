package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func gitInit(t *testing.T, dir, branch string) {
	t.Helper()
	os.MkdirAll(dir, 0o755)
	for _, args := range [][]string{
		{"init", "-b", branch},
		{"add", "."},
		{"commit", "--allow-empty", "-m", "c"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func TestDetectBranchInWorktree(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "feat", "here")
	gitInit(t, dir, "feat/here")
	if got := detectBranch(dir, root); got != "feat/here" {
		t.Fatalf("detectBranch in worktree = %q, want feat/here", got)
	}
}

func TestDetectBranchNonRepo(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "plain")
	os.MkdirAll(dir, 0o755)
	if got := detectBranch(dir, root); got != "" {
		t.Fatalf("detectBranch in non-repo = %q, want empty", got)
	}
}

func TestDetectBranchUnrelatedRepo(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(t.TempDir(), "proj")
	gitInit(t, dir, "main")
	if got := detectBranch(dir, root); got != "" {
		t.Fatalf("detectBranch in unrelated repo = %q, want empty（不在 worktreeRoot 下应进列表）", got)
	}
}

func TestSyncCmdHasRootFlag(t *testing.T) {
	f := newSyncCmd().Flags().Lookup("root")
	if f == nil {
		t.Fatal("sync command should register --root flag")
	}
	if f.Value.Type() != "bool" {
		t.Errorf("--root flag type = %q, want bool", f.Value.Type())
	}
}

func TestStatusCmdHasRootFlag(t *testing.T) {
	f := newStatusCmd().Flags().Lookup("root")
	if f == nil {
		t.Fatal("status command should register --root flag")
	}
	if f.Value.Type() != "bool" {
		t.Errorf("--root flag type = %q, want bool", f.Value.Type())
	}
}
