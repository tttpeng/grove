package git_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tttpeng/grove/core/git"
)

func git_(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		"LC_ALL=C")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func initRepoWithCommit(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	git_(t, dir, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git_(t, dir, "add", ".")
	git_(t, dir, "commit", "-m", "init")
}

func TestIsRepoAndCurrentBranch(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "r")
	initRepoWithCommit(t, dir)
	if !git.IsRepo(dir) {
		t.Error("IsRepo should be true for a git repo")
	}
	if git.IsRepo(t.TempDir()) {
		t.Error("IsRepo should be false for a non-repo")
	}
	b, err := git.CurrentBranch(dir)
	if err != nil {
		t.Fatal(err)
	}
	if b != "main" {
		t.Errorf("CurrentBranch = %q, want main", b)
	}
}

func TestIsRepoRoot(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "r")
	initRepoWithCommit(t, repo)
	sub := filepath.Join(repo, "subdir")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if !git.IsRepoRoot(repo) {
		t.Error("IsRepoRoot should be true for a repo root")
	}
	if git.IsRepoRoot(sub) {
		t.Error("IsRepoRoot should be false for a plain subdir inside a repo work tree")
	}
	if git.IsRepoRoot(t.TempDir()) {
		t.Error("IsRepoRoot should be false for a non-repo dir")
	}
}

func TestCloneRemoteURLDefaultBranch(t *testing.T) {
	root := t.TempDir()
	origin := filepath.Join(root, "origin")
	initRepoWithCommit(t, origin)
	clone := filepath.Join(root, "clone")
	if err := git.Clone(origin, clone); err != nil {
		t.Fatalf("Clone: %v", err)
	}
	if !git.IsRepo(clone) {
		t.Fatal("clone is not a repo")
	}
	url, err := git.RemoteURL(clone, "origin")
	if err != nil {
		t.Fatal(err)
	}
	if url != origin {
		t.Errorf("RemoteURL = %q, want %q", url, origin)
	}
	db, err := git.DefaultBranch(clone)
	if err != nil {
		t.Fatal(err)
	}
	if db != "main" {
		t.Errorf("DefaultBranch = %q, want main", db)
	}
}

func TestWorktreeLifecycle(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	initRepoWithCommit(t, repo)
	wt := filepath.Join(root, "wt")
	if err := git.AddWorktree(repo, wt, "feat/x", "main"); err != nil {
		t.Fatalf("AddWorktree: %v", err)
	}
	if !git.IsRepo(wt) {
		t.Fatal("worktree not created")
	}
	b, _ := git.CurrentBranch(wt)
	if b != "feat/x" {
		t.Errorf("worktree branch = %q, want feat/x", b)
	}
	wts, err := git.ListWorktrees(repo)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, w := range wts {
		if w.Branch == "feat/x" {
			found = true
		}
	}
	if !found {
		t.Errorf("feat/x worktree not in list: %+v", wts)
	}
	if err := git.RemoveWorktree(repo, wt, false); err != nil {
		t.Fatalf("RemoveWorktree: %v", err)
	}
	if git.IsRepo(wt) {
		t.Error("worktree should be removed")
	}
}

func TestListWorktreesPrunable(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	initRepoWithCommit(t, repo)
	wt := filepath.Join(root, "wt")
	if err := git.AddWorktree(repo, wt, "feat/zombie", "main"); err != nil {
		t.Fatalf("AddWorktree: %v", err)
	}
	if err := os.RemoveAll(wt); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}
	wts, err := git.ListWorktrees(repo)
	if err != nil {
		t.Fatal(err)
	}
	var found *git.Worktree
	for i := range wts {
		if wts[i].Branch == "feat/zombie" {
			found = &wts[i]
		}
	}
	if found == nil {
		t.Fatalf("feat/zombie worktree not in list: %+v", wts)
	}
	if !found.Prunable {
		t.Errorf("removed worktree dir should be Prunable=true: %+v", *found)
	}
}

func TestIsDirty(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "r")
	initRepoWithCommit(t, dir)
	d, err := git.IsDirty(dir)
	if err != nil {
		t.Fatal(err)
	}
	if d {
		t.Error("fresh repo should be clean")
	}
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	d, _ = git.IsDirty(dir)
	if !d {
		t.Error("repo with untracked file should be dirty")
	}
}

func TestIsDirtyExcluding(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "host")
	initRepoWithCommit(t, dir)

	d, err := git.IsDirtyExcluding(dir, []string{"member"})
	if err != nil {
		t.Fatal(err)
	}
	if d {
		t.Error("fresh repo should be clean")
	}

	member := filepath.Join(dir, "member")
	if err := os.MkdirAll(member, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(member, "x.txt"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err = git.IsDirtyExcluding(dir, []string{"member"})
	if err != nil {
		t.Fatal(err)
	}
	if d {
		t.Error("untracked content only inside excluded subdir should not be dirty")
	}

	if err := os.WriteFile(filepath.Join(dir, "note.md"), []byte("note"), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err = git.IsDirtyExcluding(dir, []string{"member"})
	if err != nil {
		t.Fatal(err)
	}
	if !d {
		t.Error("untracked file outside excluded subdir should be dirty")
	}

	if err := os.Remove(filepath.Join(dir, "note.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err = git.IsDirtyExcluding(dir, []string{"member"})
	if err != nil {
		t.Fatal(err)
	}
	if !d {
		t.Error("tracked modification outside excluded subdir should be dirty")
	}
}

func TestLocalBranchExists(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "r")
	initRepoWithCommit(t, dir)
	if !git.LocalBranchExists(dir, "main") {
		t.Error("main should exist")
	}
	if git.LocalBranchExists(dir, "ghost") {
		t.Error("ghost should not exist")
	}
}

func TestBehind(t *testing.T) {
	root := t.TempDir()
	origin := filepath.Join(root, "origin")
	initRepoWithCommit(t, origin)
	clone := filepath.Join(root, "clone")
	if err := git.Clone(origin, clone); err != nil {
		t.Fatalf("Clone: %v", err)
	}

	n, err := git.Behind(clone, "origin/main")
	if err != nil {
		t.Fatalf("Behind: %v", err)
	}
	if n != 0 {
		t.Errorf("fresh clone Behind = %d, want 0", n)
	}

	if err := os.WriteFile(filepath.Join(origin, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	git_(t, origin, "add", ".")
	git_(t, origin, "commit", "-m", "second")
	if err := os.WriteFile(filepath.Join(origin, "b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	git_(t, origin, "add", ".")
	git_(t, origin, "commit", "-m", "third")

	if err := git.Fetch(clone, "origin", "main"); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	n, err = git.Behind(clone, "origin/main")
	if err != nil {
		t.Fatalf("Behind: %v", err)
	}
	if n != 2 {
		t.Errorf("Behind = %d, want 2", n)
	}
}

func TestRevExists(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "r")
	initRepoWithCommit(t, dir)
	if !git.RevExists(dir, "HEAD") {
		t.Error("HEAD should exist")
	}
	if !git.RevExists(dir, "main") {
		t.Error("main should exist")
	}
	if git.RevExists(dir, "origin/nope") {
		t.Error("origin/nope should not exist")
	}
	if git.RevExists(dir, "deadbeef") {
		t.Error("bogus rev should not exist")
	}
}

func TestBranchDescriptionRoundTrip(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "r")
	initRepoWithCommit(t, dir)

	if err := git.SetBranchDescription(dir, "main", "用途说明"); err != nil {
		t.Fatalf("SetBranchDescription: %v", err)
	}
	got, err := git.BranchDescription(dir, "main")
	if err != nil {
		t.Fatalf("BranchDescription: %v", err)
	}
	if got != "用途说明" {
		t.Errorf("BranchDescription = %q, want %q", got, "用途说明")
	}
}

func TestBranchDescriptionUnset(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "r")
	initRepoWithCommit(t, dir)

	if err := git.SetBranchDescription(dir, "main", "x"); err != nil {
		t.Fatalf("SetBranchDescription: %v", err)
	}
	if err := git.SetBranchDescription(dir, "main", ""); err != nil {
		t.Fatalf("SetBranchDescription unset: %v", err)
	}
	got, err := git.BranchDescription(dir, "main")
	if err != nil {
		t.Fatalf("BranchDescription after unset: %v", err)
	}
	if got != "" {
		t.Errorf("BranchDescription after unset = %q, want empty", got)
	}
}

func TestBranchDescriptionUnsetIdempotent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "r")
	initRepoWithCommit(t, dir)

	if err := git.SetBranchDescription(dir, "main", ""); err != nil {
		t.Fatalf("unset on never-set key should be nil err, got: %v", err)
	}
}

func TestBranchDescriptionUnsetReturnsEmpty(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "r")
	initRepoWithCommit(t, dir)

	got, err := git.BranchDescription(dir, "main")
	if err != nil {
		t.Fatalf("BranchDescription on unset key should be nil err, got: %v", err)
	}
	if got != "" {
		t.Errorf("BranchDescription on unset key = %q, want empty", got)
	}
}

func TestBranchDescriptionSlashBranch(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "r")
	initRepoWithCommit(t, dir)
	git_(t, dir, "branch", "feat/x")

	if err := git.SetBranchDescription(dir, "feat/x", "斜杠分支"); err != nil {
		t.Fatalf("SetBranchDescription: %v", err)
	}
	got, err := git.BranchDescription(dir, "feat/x")
	if err != nil {
		t.Fatalf("BranchDescription: %v", err)
	}
	if got != "斜杠分支" {
		t.Errorf("BranchDescription = %q, want %q", got, "斜杠分支")
	}
}

func TestMergeUpToDate(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "r")
	initRepoWithCommit(t, dir)
	git_(t, dir, "branch", "base")

	res, err := git.Merge(dir, "base")
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if res != git.MergeUpToDate {
		t.Errorf("Merge = %v, want MergeUpToDate", res)
	}
}

func TestMergeMerged(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "r")
	initRepoWithCommit(t, dir)
	git_(t, dir, "branch", "base")

	git_(t, dir, "checkout", "base")
	if err := os.WriteFile(filepath.Join(dir, "base.txt"), []byte("from base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git_(t, dir, "add", ".")
	git_(t, dir, "commit", "-m", "base commit")
	git_(t, dir, "checkout", "main")

	res, err := git.Merge(dir, "base")
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if res != git.MergeMerged {
		t.Errorf("Merge = %v, want MergeMerged", res)
	}
	if _, err := os.Stat(filepath.Join(dir, "base.txt")); err != nil {
		t.Errorf("base.txt should be merged in: %v", err)
	}
}

func TestMergeConflict(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "r")
	initRepoWithCommit(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("original\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git_(t, dir, "add", ".")
	git_(t, dir, "commit", "-m", "add f")
	git_(t, dir, "branch", "base")

	git_(t, dir, "checkout", "base")
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("from base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git_(t, dir, "add", ".")
	git_(t, dir, "commit", "-m", "base edit")

	git_(t, dir, "checkout", "main")
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("from main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git_(t, dir, "add", ".")
	git_(t, dir, "commit", "-m", "main edit")

	res, err := git.Merge(dir, "base")
	if err != nil {
		t.Fatalf("Merge conflict should not error: %v", err)
	}
	if res != git.MergeConflict {
		t.Errorf("Merge = %v, want MergeConflict", res)
	}
	content, err := os.ReadFile(filepath.Join(dir, "f.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "<<<<<<<") {
		t.Errorf("working tree should retain conflict markers, got: %q", content)
	}
}

func TestStashWithChanges(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "r")
	initRepoWithCommit(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stashed, err := git.Stash(dir)
	if err != nil {
		t.Fatalf("Stash: %v", err)
	}
	if !stashed {
		t.Error("Stash should report stashed=true with local changes")
	}
	d, _ := git.IsDirty(dir)
	if d {
		t.Error("repo should be clean after stash")
	}
}

func TestStashClean(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "r")
	initRepoWithCommit(t, dir)

	stashed, err := git.Stash(dir)
	if err != nil {
		t.Fatalf("Stash: %v", err)
	}
	if stashed {
		t.Error("Stash should report stashed=false on a clean repo")
	}
}

func TestStashPopClean(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "r")
	initRepoWithCommit(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stashed, err := git.Stash(dir)
	if err != nil || !stashed {
		t.Fatalf("setup Stash: stashed=%v err=%v", stashed, err)
	}

	conflict, err := git.StashPop(dir)
	if err != nil {
		t.Fatalf("StashPop: %v", err)
	}
	if conflict {
		t.Error("StashPop of non-conflicting change should report conflict=false")
	}
	if _, err := os.Stat(filepath.Join(dir, "dirty.txt")); err != nil {
		t.Errorf("stashed change should be restored: %v", err)
	}
}

func TestStashPopConflict(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "r")
	initRepoWithCommit(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("original\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git_(t, dir, "add", ".")
	git_(t, dir, "commit", "-m", "add f")

	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("stashed change\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stashed, err := git.Stash(dir)
	if err != nil || !stashed {
		t.Fatalf("setup Stash: stashed=%v err=%v", stashed, err)
	}

	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("committed change\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git_(t, dir, "add", ".")
	git_(t, dir, "commit", "-m", "conflicting commit")

	conflict, err := git.StashPop(dir)
	if err != nil {
		t.Fatalf("StashPop conflict should not error: %v", err)
	}
	if !conflict {
		t.Error("StashPop with conflicting change should report conflict=true")
	}
	content, err := os.ReadFile(filepath.Join(dir, "f.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "<<<<<<<") {
		t.Errorf("working tree should retain conflict markers, got: %q", content)
	}
}
