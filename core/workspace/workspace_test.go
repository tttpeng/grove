package workspace_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/tttpeng/grove/core/config"
	"github.com/tttpeng/grove/core/git"
	"github.com/tttpeng/grove/core/manifest"
	"github.com/tttpeng/grove/core/workspace"
)

func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func makeBareOrigin(t *testing.T, root, name string) string {
	t.Helper()
	bare := filepath.Join(root, "origins", name+".git")
	if err := os.MkdirAll(bare, 0o755); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, bare, "init", "--bare", "-b", "main")
	seed := filepath.Join(t.TempDir(), "seed-"+name)
	gitCmd(t, root, "clone", bare, seed)
	if err := os.WriteFile(filepath.Join(seed, "README.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, seed, "add", ".")
	gitCmd(t, seed, "commit", "-m", "init")
	gitCmd(t, seed, "push", "origin", "main")
	return bare
}

func setup(t *testing.T, repoNames ...string) (config.ResolvedProject, *manifest.Manifest) {
	t.Helper()
	root := t.TempDir()
	cloneRoot := filepath.Join(root, "clones")
	worktreeRoot := filepath.Join(root, "trees")
	if err := os.MkdirAll(cloneRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	m := &manifest.Manifest{Project: "p", DefaultBaseline: "main"}
	for _, name := range repoNames {
		bare := makeBareOrigin(t, root, name)
		m.Repos = append(m.Repos, manifest.Repo{Name: name, Remote: bare, Baseline: "main"})
	}
	rp := config.ResolvedProject{
		Name:         "p",
		CloneRoot:    cloneRoot,
		WorktreeRoot: worktreeRoot,
		Layout:       config.DefaultLayout,
	}
	return rp, m
}

func cloneAll(t *testing.T, rp config.ResolvedProject, m *manifest.Manifest) {
	t.Helper()
	for _, repo := range m.Repos {
		dst := filepath.Join(rp.CloneRoot, repo.Name)
		if git.IsRepo(dst) {
			continue
		}
		gitCmd(t, rp.CloneRoot, "clone", repo.Remote, dst)
	}
}

func setupHost(t *testing.T, memberNames ...string) (config.ResolvedProject, *manifest.Manifest) {
	t.Helper()
	root := t.TempDir()
	cloneRoot := filepath.Join(root, "clones")
	worktreeRoot := filepath.Join(root, "trees")

	hostBare := makeBareOrigin(t, root, "erp-main")
	if err := os.MkdirAll(filepath.Dir(cloneRoot), 0o755); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, filepath.Dir(cloneRoot), "clone", hostBare, cloneRoot)

	m := &manifest.Manifest{
		Project:         "erp-main",
		DefaultBaseline: "main",
		Host:            &manifest.Repo{Name: "erp-main", Remote: hostBare, Baseline: "main"},
	}
	for _, name := range memberNames {
		bare := makeBareOrigin(t, root, name)
		m.Repos = append(m.Repos, manifest.Repo{Name: name, Remote: bare, Baseline: "main"})
	}
	rp := config.ResolvedProject{
		Name:         "erp-main",
		CloneRoot:    cloneRoot,
		WorktreeRoot: worktreeRoot,
		Layout:       config.DefaultLayout,
	}
	return rp, m
}

func cloneMembers(t *testing.T, rp config.ResolvedProject, m *manifest.Manifest) {
	t.Helper()
	for _, repo := range m.Repos {
		dst := workspace.MemberCloneDir(rp, m, repo.Name)
		if git.IsRepo(dst) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		gitCmd(t, filepath.Dir(dst), "clone", repo.Remote, dst)
	}
}

func TestOpenHostNestedLayout(t *testing.T) {
	rp, m := setupHost(t, "a", "b")
	cloneMembers(t, rp, m)

	results, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results (host+2 members), got %d: %+v", len(results), results)
	}
	if results[0].Repo != "erp-main" {
		t.Errorf("first result repo = %q, want host erp-main first", results[0].Repo)
	}

	hostWt := filepath.Join(rp.WorktreeRoot, "feat/x")
	if !git.IsRepo(hostWt) {
		t.Errorf("host worktree %s not created at group root", hostWt)
	}
	b, _ := git.CurrentBranch(hostWt)
	if b != "feat/x" {
		t.Errorf("host worktree branch = %q, want feat/x", b)
	}
	reposDir := filepath.Join(hostWt, "repos")
	for _, name := range []string{"a", "b"} {
		wantClone := filepath.Join(rp.CloneRoot, "repos", name)
		if !git.IsRepo(wantClone) {
			t.Errorf("member %s clone %s not under cloneRoot/repos", name, wantClone)
		}
		memberWt := workspace.MemberWorktreePath(rp, m, "feat/x", name)
		if got := filepath.Join(rp.WorktreeRoot, "feat/x", "repos", name); memberWt != got {
			t.Errorf("MemberWorktreePath = %q, want %q", memberWt, got)
		}
		if !git.IsRepo(memberWt) {
			t.Errorf("member worktree %s not created", memberWt)
		}
		if filepath.Dir(memberWt) != reposDir {
			t.Errorf("member %s worktree %s not nested under host repos/ %s", name, memberWt, reposDir)
		}
	}
}

func TestOpenHostIdempotentReuse(t *testing.T) {
	rp, m := setupHost(t, "a")
	cloneMembers(t, rp, m)

	if _, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{}); err != nil {
		t.Fatalf("first Open: %v", err)
	}
	results, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{})
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	for _, r := range results {
		if r.Action != "reused" {
			t.Errorf("repo %s action = %q, want reused", r.Repo, r.Action)
		}
	}
}

func TestOpenHostCompensationRollsBackHost(t *testing.T) {
	rp, m := setupHost(t, "a")
	cloneMembers(t, rp, m)
	m.Repos[0].Baseline = "does-not-exist"

	results, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{})
	if err == nil {
		t.Fatal("expected error when member baseline missing")
	}
	hostWt := filepath.Join(rp.WorktreeRoot, "feat/x")
	if git.IsRepo(hostWt) {
		t.Error("host worktree should be rolled back after member failure")
	}
	var hostResult *workspace.RepoResult
	for i := range results {
		if results[i].Repo == "erp-main" {
			hostResult = &results[i]
		}
	}
	if hostResult == nil {
		t.Fatal("expected host result in partial results")
	}
	if hostResult.Action != "rolled-back" {
		t.Errorf("host action = %q, want rolled-back", hostResult.Action)
	}
}

func TestCloseHostBlocksOnNotes(t *testing.T) {
	rp, m := setupHost(t, "a")
	cloneMembers(t, rp, m)
	if _, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{}); err != nil {
		t.Fatalf("Open: %v", err)
	}
	hostWt := filepath.Join(rp.WorktreeRoot, "feat/x")
	if err := os.WriteFile(filepath.Join(hostWt, ".incident-note.md"), []byte("notes"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := workspace.Close(rp, m, "feat/x", workspace.CloseOptions{})
	if err == nil {
		t.Fatal("expected Close to be blocked by host untracked note")
	}
	if !git.IsRepo(hostWt) {
		t.Error("host worktree must not be removed when blocked")
	}
	if !git.IsRepo(workspace.MemberWorktreePath(rp, m, "feat/x", "a")) {
		t.Error("member worktree must not be removed when group blocked")
	}
}

func TestCloseHostAllowsMemberOnlyDir(t *testing.T) {
	rp, m := setupHost(t, "a")
	cloneMembers(t, rp, m)
	if _, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{}); err != nil {
		t.Fatalf("Open: %v", err)
	}
	hostWt := filepath.Join(rp.WorktreeRoot, "feat/x")

	results, err := workspace.Close(rp, m, "feat/x", workspace.CloseOptions{})
	if err != nil {
		t.Fatalf("Close should pass when host root only has member dirs: %v", err)
	}
	if git.IsRepo(hostWt) {
		t.Error("host worktree should be removed")
	}
	if git.IsRepo(workspace.MemberWorktreePath(rp, m, "feat/x", "a")) {
		t.Error("member worktree should be removed")
	}
	var hostRemoved bool
	for _, r := range results {
		if r.Repo == "erp-main" && r.Action == "removed" {
			hostRemoved = true
		}
	}
	if !hostRemoved {
		t.Error("host should be removed in close results")
	}
	cloneDir := rp.CloneRoot
	wts, _ := git.ListWorktrees(cloneDir)
	for _, w := range wts {
		if w.Prunable {
			t.Errorf("host clone has prunable worktree %s", w.Path)
		}
	}
}

func TestCloseHostForceRemovesNotes(t *testing.T) {
	rp, m := setupHost(t, "a")
	cloneMembers(t, rp, m)
	if _, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{}); err != nil {
		t.Fatalf("Open: %v", err)
	}
	hostWt := filepath.Join(rp.WorktreeRoot, "feat/x")
	if err := os.WriteFile(filepath.Join(hostWt, ".incident-note.md"), []byte("notes"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := workspace.Close(rp, m, "feat/x", workspace.CloseOptions{Force: true}); err != nil {
		t.Fatalf("forced Close: %v", err)
	}
	if git.IsRepo(hostWt) {
		t.Error("forced Close should remove host worktree with notes")
	}
	if _, err := os.Stat(hostWt); !os.IsNotExist(err) {
		t.Errorf("host worktree dir should be gone, stat err = %v", err)
	}
}

func TestListAndStatusWithHost(t *testing.T) {
	rp, m := setupHost(t, "a", "b")
	cloneMembers(t, rp, m)
	if _, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{}); err != nil {
		t.Fatalf("Open: %v", err)
	}

	wss, err := workspace.List(rp, m)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(wss) != 2 {
		t.Fatalf("expected 2 workspaces (root + feat/x), got %d: %+v", len(wss), wss)
	}
	if len(wss[1].Repos) != 3 {
		t.Fatalf("workspace should have 3 repos (host+2), got %d: %+v", len(wss[1].Repos), wss[1].Repos)
	}
	var sawHost bool
	for _, r := range wss[1].Repos {
		if r.Repo == "erp-main" {
			sawHost = true
		}
	}
	if !sawHost {
		t.Error("List should include host repo")
	}

	st, err := workspace.Status(rp, m, "feat/x")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(st.Repos) != 3 {
		t.Fatalf("status repos = %d, want 3", len(st.Repos))
	}
	var host *workspace.RepoStatus
	for i := range st.Repos {
		if st.Repos[i].Repo == "erp-main" {
			host = &st.Repos[i]
		}
	}
	if host == nil {
		t.Fatal("Status should include host")
	}
	if !host.Exists {
		t.Error("host should exist")
	}
	if host.Branch != "feat/x" {
		t.Errorf("host branch = %q, want feat/x", host.Branch)
	}
	if host.Dirty {
		t.Error("host should be clean (only member dir present)")
	}
}

func advanceOrigin(t *testing.T, bare, branch string, n int) {
	t.Helper()
	work := filepath.Join(t.TempDir(), "advance")
	gitCmd(t, t.TempDir(), "clone", "-b", branch, bare, work)
	for i := 0; i < n; i++ {
		fn := filepath.Join(work, "f"+strings.Repeat("x", i+1))
		if err := os.WriteFile(fn, []byte("c\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		gitCmd(t, work, "add", ".")
		gitCmd(t, work, "commit", "-m", "upstream")
	}
	gitCmd(t, work, "push", "origin", branch)
}

func commitInWorktree(t *testing.T, wt string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		fn := filepath.Join(wt, "w"+strings.Repeat("y", i+1))
		if err := os.WriteFile(fn, []byte("c\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		gitCmd(t, wt, "add", ".")
		gitCmd(t, wt, "commit", "-m", "local")
	}
}

func hostStatus(t *testing.T, st *workspace.Workspace) workspace.RepoStatus {
	t.Helper()
	for _, r := range st.Repos {
		if r.Repo == "erp-main" {
			return r
		}
	}
	t.Fatal("host status not found")
	return workspace.RepoStatus{}
}

func TestStatusBehindRelativeBaseline(t *testing.T) {
	rp, m := setupHost(t)
	if _, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{}); err != nil {
		t.Fatalf("Open: %v", err)
	}

	advanceOrigin(t, m.Host.Remote, "main", 3)
	gitCmd(t, rp.CloneRoot, "fetch", "origin", "main")

	st, err := workspace.Status(rp, m, "feat/x")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	host := hostStatus(t, st)
	if host.Behind != 3 {
		t.Errorf("host Behind = %d, want 3", host.Behind)
	}
	if host.Ahead != 0 {
		t.Errorf("host Ahead = %d, want 0", host.Ahead)
	}
}

func TestStatusAheadRelativeBaseline(t *testing.T) {
	rp, m := setupHost(t)
	if _, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{}); err != nil {
		t.Fatalf("Open: %v", err)
	}

	hostWt := filepath.Join(rp.WorktreeRoot, "feat/x")
	commitInWorktree(t, hostWt, 2)

	st, err := workspace.Status(rp, m, "feat/x")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	host := hostStatus(t, st)
	if host.Ahead != 2 {
		t.Errorf("host Ahead = %d, want 2", host.Ahead)
	}
	if host.Behind != 0 {
		t.Errorf("host Behind = %d, want 0", host.Behind)
	}
}

func TestStatusAheadBehindNoOriginBaseline(t *testing.T) {
	rp, m := setupHost(t)
	if _, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{}); err != nil {
		t.Fatalf("Open: %v", err)
	}

	m.Host.Baseline = "no-such-baseline"

	st, err := workspace.Status(rp, m, "feat/x")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	host := hostStatus(t, st)
	if host.Ahead != 0 || host.Behind != 0 {
		t.Errorf("host Ahead/Behind = %d/%d, want 0/0", host.Ahead, host.Behind)
	}
}

func TestNoHostMemberPathsUnchanged(t *testing.T) {
	rp, m := setup(t, "a", "b")
	cloneAll(t, rp, m)

	for _, repo := range m.Repos {
		if got := workspace.MemberCloneDir(rp, m, repo.Name); got != filepath.Join(rp.CloneRoot, repo.Name) {
			t.Errorf("no-host MemberCloneDir(%s) = %q, want cloneRoot/%s", repo.Name, got, repo.Name)
		}
		if got := workspace.MemberWorktreePath(rp, m, "feat/x", repo.Name); got != rp.WorktreePath("feat/x", repo.Name) {
			t.Errorf("no-host MemberWorktreePath(%s) = %q, want %q", repo.Name, got, rp.WorktreePath("feat/x", repo.Name))
		}
	}

	if _, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{}); err != nil {
		t.Fatalf("Open: %v", err)
	}
	for _, repo := range m.Repos {
		wt := rp.WorktreePath("feat/x", repo.Name)
		if !git.IsRepo(wt) {
			t.Errorf("no-host member worktree %s should not be under repos/", wt)
		}
		if strings.Contains(wt, string(filepath.Separator)+"repos"+string(filepath.Separator)) {
			t.Errorf("no-host worktree %s must not contain repos/ segment", wt)
		}
	}
}

func TestOpenCreatesWorktreesAcrossRepos(t *testing.T) {
	rp, m := setup(t, "a", "b")
	cloneAll(t, rp, m)

	results, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Err != nil {
			t.Fatalf("repo %s err: %v", r.Repo, r.Err)
		}
		if r.Action != "created" {
			t.Errorf("repo %s action = %q, want created", r.Repo, r.Action)
		}
		wt := rp.WorktreePath("feat/x", r.Repo)
		if !git.IsRepo(wt) {
			t.Errorf("worktree %s not created", wt)
		}
		b, _ := git.CurrentBranch(wt)
		if b != "feat/x" {
			t.Errorf("repo %s branch = %q, want feat/x", r.Repo, b)
		}
	}
}

func TestOpenIdempotentReuse(t *testing.T) {
	rp, m := setup(t, "a", "b")
	cloneAll(t, rp, m)

	if _, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{}); err != nil {
		t.Fatalf("first Open: %v", err)
	}
	results, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{})
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	for _, r := range results {
		if r.Action != "reused" {
			t.Errorf("repo %s action = %q, want reused", r.Repo, r.Action)
		}
	}
}

func TestOpenBaselineOverride(t *testing.T) {
	rp, m := setup(t, "a")
	cloneAll(t, rp, m)

	cloneDir := filepath.Join(rp.CloneRoot, "a")
	seed := filepath.Join(t.TempDir(), "seed-rel")
	gitCmd(t, filepath.Dir(seed), "clone", m.Repos[0].Remote, seed)
	if err := os.WriteFile(filepath.Join(seed, "rel.txt"), []byte("z\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, seed, "add", ".")
	gitCmd(t, seed, "commit", "-m", "rel commit")
	gitCmd(t, seed, "push", "origin", "main:release")

	relHead := gitCmd(t, seed, "rev-parse", "HEAD")

	results, err := workspace.Open(rp, m, "feat/y", workspace.OpenOptions{Baseline: "release"})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if results[0].Err != nil {
		t.Fatalf("repo err: %v", results[0].Err)
	}
	wt := rp.WorktreePath("feat/y", "a")
	head := gitCmd(t, wt, "rev-parse", "HEAD")
	if head != relHead {
		t.Errorf("worktree head = %q, want release head %q", head, relHead)
	}
	_ = cloneDir
}

func TestOpenNoFetchUsesOriginBaseline(t *testing.T) {
	rp, m := setup(t, "a")
	cloneAll(t, rp, m)

	cloneDir := filepath.Join(rp.CloneRoot, "a")

	seed := filepath.Join(t.TempDir(), "seed-stage")
	gitCmd(t, filepath.Dir(seed), "clone", m.Repos[0].Remote, seed)
	if err := os.WriteFile(filepath.Join(seed, "stage.txt"), []byte("s\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, seed, "add", ".")
	gitCmd(t, seed, "commit", "-m", "stage commit")
	gitCmd(t, seed, "push", "origin", "main:stage")
	stageHead := gitCmd(t, seed, "rev-parse", "HEAD")

	gitCmd(t, cloneDir, "checkout", "-b", "feat/local")
	gitCmd(t, cloneDir, "fetch", "origin", "stage")
	if git.LocalBranchExists(cloneDir, "stage") {
		t.Fatal("precondition: clone must not have a local stage branch")
	}
	if !git.RevExists(cloneDir, "origin/stage") {
		t.Fatal("precondition: clone must have origin/stage")
	}

	results, err := workspace.Open(rp, m, "feat/new", workspace.OpenOptions{Baseline: "stage", NoFetch: true})
	if err != nil {
		t.Fatalf("Open --no-fetch: %v", err)
	}
	if results[0].Err != nil {
		t.Fatalf("repo err: %v", results[0].Err)
	}
	if results[0].Action != "created" {
		t.Fatalf("action = %q, want created", results[0].Action)
	}
	wt := rp.WorktreePath("feat/new", "a")
	if !git.IsRepo(wt) {
		t.Fatalf("worktree %s not created", wt)
	}
	head := gitCmd(t, wt, "rev-parse", "HEAD")
	if head != stageHead {
		t.Errorf("worktree head = %q, want origin/stage head %q", head, stageHead)
	}
	wtBranch := gitCmd(t, wt, "rev-parse", "--abbrev-ref", "HEAD")
	if wtBranch != "feat/new" {
		t.Errorf("worktree branch = %q, want feat/new", wtBranch)
	}
}

func TestOpenMissingCloneErrors(t *testing.T) {
	rp, m := setup(t, "a", "b")
	gitCmd(t, rp.CloneRoot, "clone", m.Repos[0].Remote, filepath.Join(rp.CloneRoot, "a"))

	_, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{})
	if err == nil {
		t.Fatal("expected error for missing clone")
	}
	wt := rp.WorktreePath("feat/x", "a")
	if git.IsRepo(wt) {
		t.Error("no worktree should be created when a clone is missing")
	}
}

func TestOpenCompensationOnFailure(t *testing.T) {
	rp, m := setup(t, "a", "b")
	cloneAll(t, rp, m)
	m.Repos[1].Baseline = "does-not-exist"

	results, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{})
	if err == nil {
		t.Fatal("expected error when second repo baseline missing")
	}
	wtA := rp.WorktreePath("feat/x", "a")
	if git.IsRepo(wtA) {
		t.Error("first repo worktree should be rolled back after compensation")
	}
	wtB := rp.WorktreePath("feat/x", "b")
	if git.IsRepo(wtB) {
		t.Error("second repo worktree should not exist")
	}
	if len(results) == 0 {
		t.Error("expected partial results to be returned")
	}
	for _, r := range results {
		if r.Repo == "a" {
			if r.Action != "rolled-back" {
				t.Errorf("repo a action = %q, want rolled-back after compensation", r.Action)
			}
			if r.Err != nil {
				t.Errorf("repo a err = %v, want nil for rolled-back entry", r.Err)
			}
		}
	}
}

func TestCloseRemovesGroup(t *testing.T) {
	rp, m := setup(t, "a", "b")
	cloneAll(t, rp, m)
	if _, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{}); err != nil {
		t.Fatalf("Open: %v", err)
	}

	results, err := workspace.Close(rp, m, "feat/x", workspace.CloseOptions{})
	if err != nil {
		t.Fatalf("Close: %v", err)
	}
	for _, r := range results {
		if r.Action != "removed" {
			t.Errorf("repo %s action = %q, want removed", r.Repo, r.Action)
		}
		if git.IsRepo(rp.WorktreePath("feat/x", r.Repo)) {
			t.Errorf("worktree for %s should be removed", r.Repo)
		}
	}
	for _, repo := range m.Repos {
		cloneDir := filepath.Join(rp.CloneRoot, repo.Name)
		wts, _ := git.ListWorktrees(cloneDir)
		for _, w := range wts {
			if w.Prunable {
				t.Errorf("repo %s has prunable worktree %s", repo.Name, w.Path)
			}
		}
	}
}

func TestCloseBlocksDirty(t *testing.T) {
	rp, m := setup(t, "a", "b")
	cloneAll(t, rp, m)
	if _, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{}); err != nil {
		t.Fatalf("Open: %v", err)
	}
	wtA := rp.WorktreePath("feat/x", "a")
	if err := os.WriteFile(filepath.Join(wtA, "dirty.txt"), []byte("d"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := workspace.Close(rp, m, "feat/x", workspace.CloseOptions{})
	if err == nil {
		t.Fatal("expected error closing dirty worktree without force")
	}
	if !git.IsRepo(wtA) {
		t.Error("dirty worktree must not be removed without force")
	}
	wtB := rp.WorktreePath("feat/x", "b")
	if !git.IsRepo(wtB) {
		t.Error("clean sibling worktree must not be removed when group is blocked")
	}

	if _, err := workspace.Close(rp, m, "feat/x", workspace.CloseOptions{Force: true}); err != nil {
		t.Fatalf("forced Close: %v", err)
	}
	if git.IsRepo(wtA) {
		t.Error("forced Close should remove dirty worktree")
	}
}

func TestOpenWithDescriptionListReadsBack(t *testing.T) {
	rp, m := setup(t, "a", "b")
	cloneAll(t, rp, m)

	if _, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{Description: "修复登录"}); err != nil {
		t.Fatalf("Open: %v", err)
	}

	wss, err := workspace.List(rp, m)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(wss) != 2 {
		t.Fatalf("expected 2 workspaces (root + feat/x), got %d", len(wss))
	}
	if wss[1].Description != "修复登录" {
		t.Errorf("Description = %q, want %q", wss[1].Description, "修复登录")
	}
}

func TestOpenNoDescriptionListEmpty(t *testing.T) {
	rp, m := setup(t, "a")
	cloneAll(t, rp, m)

	if _, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{}); err != nil {
		t.Fatalf("Open: %v", err)
	}
	wss, err := workspace.List(rp, m)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(wss) != 2 {
		t.Fatalf("expected 2 workspaces (root + feat/x), got %d", len(wss))
	}
	if wss[1].Description != "" {
		t.Errorf("Description = %q, want empty", wss[1].Description)
	}
}

func TestSetDescriptionOverwrites(t *testing.T) {
	rp, m := setup(t, "a")
	cloneAll(t, rp, m)
	if _, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{Description: "旧"}); err != nil {
		t.Fatalf("Open: %v", err)
	}

	if err := workspace.SetDescription(rp, m, "feat/x", "新"); err != nil {
		t.Fatalf("SetDescription: %v", err)
	}
	wss, err := workspace.List(rp, m)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if wss[1].Description != "新" {
		t.Errorf("Description = %q, want %q", wss[1].Description, "新")
	}

	if err := workspace.SetDescription(rp, m, "feat/x", ""); err != nil {
		t.Fatalf("SetDescription clear: %v", err)
	}
	wss, err = workspace.List(rp, m)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if wss[1].Description != "" {
		t.Errorf("Description after clear = %q, want empty", wss[1].Description)
	}
}

func TestDescriptionStoredInFirstMemberWhenNoHost(t *testing.T) {
	rp, m := setup(t, "a", "b")
	cloneAll(t, rp, m)
	if _, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{Description: "无 host"}); err != nil {
		t.Fatalf("Open: %v", err)
	}

	firstMember := filepath.Join(rp.CloneRoot, m.Repos[0].Name)
	got, err := git.BranchDescription(firstMember, "feat/x")
	if err != nil {
		t.Fatalf("BranchDescription: %v", err)
	}
	if got != "无 host" {
		t.Errorf("first-member description = %q, want %q", got, "无 host")
	}

	second := filepath.Join(rp.CloneRoot, m.Repos[1].Name)
	if g, _ := git.BranchDescription(second, "feat/x"); g != "" {
		t.Errorf("second member should not carry description, got %q", g)
	}
}

func TestDescriptionStoredInHostWhenHost(t *testing.T) {
	rp, m := setupHost(t, "a", "b")
	cloneMembers(t, rp, m)
	if _, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{Description: "有 host"}); err != nil {
		t.Fatalf("Open: %v", err)
	}

	got, err := git.BranchDescription(rp.CloneRoot, "feat/x")
	if err != nil {
		t.Fatalf("BranchDescription host: %v", err)
	}
	if got != "有 host" {
		t.Errorf("host description = %q, want %q", got, "有 host")
	}

	wss, err := workspace.List(rp, m)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if wss[1].Description != "有 host" {
		t.Errorf("List Description = %q, want %q", wss[1].Description, "有 host")
	}
}

func setRemoteURL(t *testing.T, dir, url string) {
	t.Helper()
	gitCmd(t, dir, "remote", "set-url", "origin", url)
}

func pushBaselineCommit(t *testing.T, remote, baseline, file, content string) string {
	t.Helper()
	seed := filepath.Join(t.TempDir(), "seed-push")
	gitCmd(t, filepath.Dir(seed), "clone", remote, seed)
	if git.RevExists(seed, "origin/"+baseline) {
		gitCmd(t, seed, "checkout", "-B", baseline, "origin/"+baseline)
	} else {
		gitCmd(t, seed, "checkout", "-b", baseline)
	}
	if err := os.WriteFile(filepath.Join(seed, file), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, seed, "add", ".")
	gitCmd(t, seed, "commit", "-m", "advance "+baseline)
	gitCmd(t, seed, "push", "origin", baseline)
	return gitCmd(t, seed, "rev-parse", "HEAD")
}

func TestOpenFetchDegradesToLocalOrigin(t *testing.T) {
	rp, m := setup(t, "a", "b")
	cloneAll(t, rp, m)

	cloneA := filepath.Join(rp.CloneRoot, "a")
	setRemoteURL(t, cloneA, m.Repos[0].Remote+".gone")

	results, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{})
	if err != nil {
		t.Fatalf("Open should succeed with local origin fallback: %v", err)
	}
	if !git.IsRepo(rp.WorktreePath("feat/x", "a")) {
		t.Error("repo a worktree should be created via degraded fallback")
	}
	if !git.IsRepo(rp.WorktreePath("feat/x", "b")) {
		t.Error("repo b worktree should be created normally")
	}
	var ra *workspace.RepoResult
	for i := range results {
		if results[i].Repo == "a" {
			ra = &results[i]
		}
	}
	if ra == nil {
		t.Fatal("missing result for repo a")
	}
	if ra.Action != "created" {
		t.Errorf("repo a action = %q, want created", ra.Action)
	}
	if !strings.Contains(ra.Note, "fetch 失败") {
		t.Errorf("repo a Note = %q, want degraded note", ra.Note)
	}
}

func TestOpenFetchFailsNoLocalOriginRollsBack(t *testing.T) {
	rp, m := setup(t, "a", "b")
	cloneAll(t, rp, m)

	cloneB := filepath.Join(rp.CloneRoot, "b")
	setRemoteURL(t, cloneB, m.Repos[1].Remote+".gone")
	m.Repos[1].Baseline = "absent-baseline"

	_, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{})
	if err == nil {
		t.Fatal("expected error when fetch fails and no local origin baseline")
	}
	if git.IsRepo(rp.WorktreePath("feat/x", "a")) {
		t.Error("repo a worktree should be rolled back after repo b failure")
	}
	if git.IsRepo(rp.WorktreePath("feat/x", "b")) {
		t.Error("repo b worktree should not exist")
	}
}

func TestSyncMergesAdvancedBaseline(t *testing.T) {
	rp, m := setup(t, "a", "b")
	cloneAll(t, rp, m)
	if _, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{}); err != nil {
		t.Fatalf("Open: %v", err)
	}

	want := pushBaselineCommit(t, m.Repos[0].Remote, "main", "adv.txt", "advanced\n")

	results, err := workspace.Sync(rp, m, "feat/x")
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	var ra, rb *workspace.RepoResult
	for i := range results {
		switch results[i].Repo {
		case "a":
			ra = &results[i]
		case "b":
			rb = &results[i]
		}
	}
	if ra == nil || rb == nil {
		t.Fatalf("missing results: %+v", results)
	}
	if ra.Action != "synced" {
		t.Errorf("repo a action = %q, want synced", ra.Action)
	}
	if rb.Action != "up-to-date" {
		t.Errorf("repo b action = %q, want up-to-date", rb.Action)
	}
	wtA := rp.WorktreePath("feat/x", "a")
	if got := gitCmd(t, wtA, "rev-parse", "HEAD"); got != want {
		t.Errorf("repo a HEAD = %q, want merged baseline %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(wtA, "adv.txt")); err != nil {
		t.Errorf("merged file adv.txt missing: %v", err)
	}
}

func TestSyncStashesDirtyWorktreeAndRestores(t *testing.T) {
	rp, m := setup(t, "a")
	cloneAll(t, rp, m)
	if _, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{}); err != nil {
		t.Fatalf("Open: %v", err)
	}
	pushBaselineCommit(t, m.Repos[0].Remote, "main", "adv.txt", "advanced\n")

	wtA := rp.WorktreePath("feat/x", "a")
	if err := os.WriteFile(filepath.Join(wtA, "wip.txt"), []byte("work in progress\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := workspace.Sync(rp, m, "feat/x")
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if results[0].Action != "synced" {
		t.Errorf("action = %q, want synced", results[0].Action)
	}
	if _, err := os.Stat(filepath.Join(wtA, "adv.txt")); err != nil {
		t.Errorf("merged file missing after stash/pop: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(wtA, "wip.txt"))
	if err != nil {
		t.Fatalf("wip.txt should be restored after pop: %v", err)
	}
	if string(got) != "work in progress\n" {
		t.Errorf("wip.txt = %q, want restored content", got)
	}
}

func TestSyncConflictPreservesAndContinues(t *testing.T) {
	rp, m := setup(t, "a", "b")
	cloneAll(t, rp, m)
	if _, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{}); err != nil {
		t.Fatalf("Open: %v", err)
	}

	pushBaselineCommit(t, m.Repos[0].Remote, "main", "conflict.txt", "origin side\n")
	pushBaselineCommit(t, m.Repos[1].Remote, "main", "advb.txt", "b advanced\n")

	wtA := rp.WorktreePath("feat/x", "a")
	if err := os.WriteFile(filepath.Join(wtA, "conflict.txt"), []byte("local side\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, wtA, "add", ".")
	gitCmd(t, wtA, "commit", "-m", "local conflicting change")

	results, err := workspace.Sync(rp, m, "feat/x")
	if err != nil {
		t.Fatalf("Sync should not abort on conflict: %v", err)
	}
	var ra, rb *workspace.RepoResult
	for i := range results {
		switch results[i].Repo {
		case "a":
			ra = &results[i]
		case "b":
			rb = &results[i]
		}
	}
	if ra == nil || rb == nil {
		t.Fatalf("missing results: %+v", results)
	}
	if ra.Action != "conflict" {
		t.Errorf("repo a action = %q, want conflict", ra.Action)
	}
	if rb.Action != "synced" {
		t.Errorf("repo b action = %q, want synced", rb.Action)
	}
	body, err := os.ReadFile(filepath.Join(wtA, "conflict.txt"))
	if err != nil {
		t.Fatalf("conflict.txt should remain: %v", err)
	}
	if !strings.Contains(string(body), "<<<<<<<") {
		t.Errorf("conflict.txt should keep conflict markers, got %q", body)
	}
}

func TestSyncSkipsMissingWorktree(t *testing.T) {
	rp, m := setup(t, "a")
	cloneAll(t, rp, m)

	results, err := workspace.Sync(rp, m, "feat/never-opened")
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != "skipped" {
		t.Errorf("action = %q, want skipped", results[0].Action)
	}
}

func TestSyncFetchDegradedNote(t *testing.T) {
	rp, m := setup(t, "a")
	cloneAll(t, rp, m)
	if _, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{}); err != nil {
		t.Fatalf("Open: %v", err)
	}
	cloneA := filepath.Join(rp.CloneRoot, "a")
	setRemoteURL(t, cloneA, m.Repos[0].Remote+".gone")

	results, err := workspace.Sync(rp, m, "feat/x")
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if results[0].Action != "up-to-date" {
		t.Errorf("action = %q, want up-to-date", results[0].Action)
	}
	if !strings.Contains(results[0].Note, "fetch 失败") {
		t.Errorf("Note = %q, want degraded note", results[0].Note)
	}
}

func TestSyncHostAndMembers(t *testing.T) {
	rp, m := setupHost(t, "a", "b")
	cloneMembers(t, rp, m)
	if _, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{}); err != nil {
		t.Fatalf("Open: %v", err)
	}

	results, err := workspace.Sync(rp, m, "feat/x")
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results (host+2 members), got %d: %+v", len(results), results)
	}
	if results[0].Repo != "erp-main" {
		t.Errorf("first sync result = %q, want host erp-main", results[0].Repo)
	}
	for _, r := range results {
		if r.Action != "up-to-date" {
			t.Errorf("repo %s action = %q, want up-to-date", r.Repo, r.Action)
		}
	}
}

func TestListAndStatus(t *testing.T) {
	rp, m := setup(t, "a", "b")
	cloneAll(t, rp, m)
	if _, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{}); err != nil {
		t.Fatalf("Open feat/x: %v", err)
	}
	if _, err := workspace.Open(rp, m, "feat/y", workspace.OpenOptions{}); err != nil {
		t.Fatalf("Open feat/y: %v", err)
	}

	wss, err := workspace.List(rp, m)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(wss) != 3 {
		t.Fatalf("expected 3 workspaces (root + feat/x + feat/y), got %d: %+v", len(wss), wss)
	}
	branches := []string{wss[1].Branch, wss[2].Branch}
	sort.Strings(branches)
	if branches[0] != "feat/x" || branches[1] != "feat/y" {
		t.Errorf("workspace branches = %v, want feat/x feat/y", branches)
	}
	for _, ws := range wss {
		if ws.IsRoot {
			continue
		}
		if len(ws.Repos) != 2 {
			t.Errorf("workspace %s has %d repos, want 2", ws.Branch, len(ws.Repos))
		}
	}

	st, err := workspace.Status(rp, m, "feat/x")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.Branch != "feat/x" {
		t.Errorf("status branch = %q, want feat/x", st.Branch)
	}
	if len(st.Repos) != 2 {
		t.Fatalf("status repos = %d, want 2", len(st.Repos))
	}
	for _, r := range st.Repos {
		if !r.Exists {
			t.Errorf("repo %s should exist", r.Repo)
		}
		if r.Branch != "feat/x" {
			t.Errorf("repo %s branch = %q, want feat/x", r.Repo, r.Branch)
		}
		if r.Dirty {
			t.Errorf("repo %s should be clean", r.Repo)
		}
	}
}
