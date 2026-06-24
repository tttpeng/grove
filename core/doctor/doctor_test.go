package doctor_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tttpeng/grove/core/config"
	"github.com/tttpeng/grove/core/doctor"
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

func findingsByKind(fs []doctor.Finding, kind string) []doctor.Finding {
	var out []doctor.Finding
	for _, f := range fs {
		if f.Kind == kind {
			out = append(out, f)
		}
	}
	return out
}

func TestCheckCleanNoFindings(t *testing.T) {
	rp, m := setup(t, "a", "b")
	cloneAll(t, rp, m)
	if _, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{}); err != nil {
		t.Fatalf("Open: %v", err)
	}

	fs, err := doctor.Check(rp, m, "feat/x")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(fs) != 0 {
		t.Errorf("clean workspace should yield no findings, got %+v", fs)
	}
}

func TestCheckToleratesMissingFeatureBranch(t *testing.T) {
	rp, m := setup(t, "a", "b")
	cloneAll(t, rp, m)

	fs, err := doctor.Check(rp, m, "feat/never")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(fs) != 0 {
		t.Errorf("missing feature branch must not be reported, got %+v", fs)
	}
}

func TestCheckDetectsDrift(t *testing.T) {
	rp, m := setup(t, "a", "b")
	cloneAll(t, rp, m)
	if _, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{}); err != nil {
		t.Fatalf("Open: %v", err)
	}
	wtA := rp.WorktreePath("feat/x", "a")
	gitCmd(t, wtA, "checkout", "-b", "other")

	fs, err := doctor.Check(rp, m, "feat/x")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	drift := findingsByKind(fs, "drift")
	if len(drift) != 1 {
		t.Fatalf("expected 1 drift finding, got %+v", fs)
	}
	if drift[0].Repo != "a" || drift[0].Branch != "feat/x" {
		t.Errorf("drift finding = %+v, want repo a branch feat/x", drift[0])
	}
}

func TestCheckDetectsDirty(t *testing.T) {
	rp, m := setup(t, "a", "b")
	cloneAll(t, rp, m)
	if _, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{}); err != nil {
		t.Fatalf("Open: %v", err)
	}
	wtB := rp.WorktreePath("feat/x", "b")
	if err := os.WriteFile(filepath.Join(wtB, "dirty.txt"), []byte("d"), 0o644); err != nil {
		t.Fatal(err)
	}

	fs, err := doctor.Check(rp, m, "feat/x")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	dirty := findingsByKind(fs, "dirty")
	if len(dirty) != 1 {
		t.Fatalf("expected 1 dirty finding, got %+v", fs)
	}
	if dirty[0].Repo != "b" {
		t.Errorf("dirty finding repo = %q, want b", dirty[0].Repo)
	}
}

func TestCheckDetectsBehind(t *testing.T) {
	rp, m := setup(t, "a")
	cloneAll(t, rp, m)
	if _, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{}); err != nil {
		t.Fatalf("Open: %v", err)
	}

	seed := filepath.Join(t.TempDir(), "seed-advance")
	gitCmd(t, filepath.Dir(seed), "clone", m.Repos[0].Remote, seed)
	if err := os.WriteFile(filepath.Join(seed, "adv.txt"), []byte("z"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, seed, "add", ".")
	gitCmd(t, seed, "commit", "-m", "advance")
	gitCmd(t, seed, "push", "origin", "main")

	cloneDir := filepath.Join(rp.CloneRoot, "a")
	gitCmd(t, cloneDir, "fetch", "origin", "main")

	fs, err := doctor.Check(rp, m, "feat/x")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	behind := findingsByKind(fs, "behind")
	if len(behind) != 1 {
		t.Fatalf("expected 1 behind finding, got %+v", fs)
	}
	if behind[0].Repo != "a" {
		t.Errorf("behind finding repo = %q, want a", behind[0].Repo)
	}
}

func TestCheckDetectsPrunableAndPruneFixes(t *testing.T) {
	rp, m := setup(t, "a", "b")
	cloneAll(t, rp, m)
	if _, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{}); err != nil {
		t.Fatalf("Open: %v", err)
	}
	wtA := rp.WorktreePath("feat/x", "a")
	if err := os.RemoveAll(wtA); err != nil {
		t.Fatal(err)
	}

	fs, err := doctor.Check(rp, m, "")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	prunable := findingsByKind(fs, "prunable")
	if len(prunable) != 1 {
		t.Fatalf("expected 1 prunable finding, got %+v", fs)
	}
	if prunable[0].Repo != "a" {
		t.Errorf("prunable finding repo = %q, want a", prunable[0].Repo)
	}

	results, err := doctor.Prune(rp, m)
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	var pruned int
	for _, r := range results {
		if r.Err != nil {
			t.Errorf("repo %s prune err: %v", r.Repo, r.Err)
		}
		pruned += len(r.Pruned)
	}
	if pruned != 1 {
		t.Errorf("expected 1 pruned path, got %d (%+v)", pruned, results)
	}

	fs2, err := doctor.Check(rp, m, "")
	if err != nil {
		t.Fatalf("Check after prune: %v", err)
	}
	if len(findingsByKind(fs2, "prunable")) != 0 {
		t.Errorf("prunable should be gone after Prune, got %+v", fs2)
	}
}

func TestCheckBranchScopedPrunableOnlyForThatBranch(t *testing.T) {
	rp, m := setup(t, "a")
	cloneAll(t, rp, m)
	if _, err := workspace.Open(rp, m, "feat/a", workspace.OpenOptions{}); err != nil {
		t.Fatalf("Open feat/a: %v", err)
	}
	if _, err := workspace.Open(rp, m, "feat/b", workspace.OpenOptions{}); err != nil {
		t.Fatalf("Open feat/b: %v", err)
	}
	if err := os.RemoveAll(rp.WorktreePath("feat/a", "a")); err != nil {
		t.Fatal(err)
	}

	fsB, err := doctor.Check(rp, m, "feat/b")
	if err != nil {
		t.Fatalf("Check feat/b: %v", err)
	}
	if got := findingsByKind(fsB, "prunable"); len(got) != 0 {
		t.Errorf("doctor feat/b must not report feat/a's prunable, got %+v", got)
	}

	fsAll, err := doctor.Check(rp, m, "")
	if err != nil {
		t.Fatalf("Check all: %v", err)
	}
	if got := findingsByKind(fsAll, "prunable"); len(got) != 1 {
		t.Errorf("doctor all must report the prunable, got %+v", got)
	}
}

func TestCheckBranchScopedPrunableNonDefaultLayout(t *testing.T) {
	rp, m := setup(t, "a")
	rp.Layout = "{worktreeRoot}/{repo}@{branch}"
	cloneAll(t, rp, m)
	if _, err := workspace.Open(rp, m, "feat/a", workspace.OpenOptions{}); err != nil {
		t.Fatalf("Open feat/a: %v", err)
	}
	if _, err := workspace.Open(rp, m, "feat/b", workspace.OpenOptions{}); err != nil {
		t.Fatalf("Open feat/b: %v", err)
	}
	if err := os.RemoveAll(rp.WorktreePath("feat/a", "a")); err != nil {
		t.Fatal(err)
	}

	fsA, err := doctor.Check(rp, m, "feat/a")
	if err != nil {
		t.Fatalf("Check feat/a: %v", err)
	}
	if got := findingsByKind(fsA, "prunable"); len(got) != 1 {
		t.Errorf("doctor feat/a must report feat/a's prunable under non-default layout, got %+v", got)
	}

	fsB, err := doctor.Check(rp, m, "feat/b")
	if err != nil {
		t.Fatalf("Check feat/b: %v", err)
	}
	if got := findingsByKind(fsB, "prunable"); len(got) != 0 {
		t.Errorf("doctor feat/b must not report feat/a's prunable, got %+v", got)
	}

	fsAll, err := doctor.Check(rp, m, "")
	if err != nil {
		t.Fatalf("Check all: %v", err)
	}
	if got := findingsByKind(fsAll, "prunable"); len(got) != 1 {
		t.Errorf("doctor all must report the prunable, got %+v", got)
	}
}

func TestCheckHostDrift(t *testing.T) {
	rp, m := setupHost(t, "a")
	cloneMembers(t, rp, m)
	if _, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{}); err != nil {
		t.Fatalf("Open: %v", err)
	}
	hostWt := filepath.Join(rp.WorktreeRoot, "feat/x")
	gitCmd(t, hostWt, "checkout", "-b", "other")

	fs, err := doctor.Check(rp, m, "feat/x")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	drift := findingsByKind(fs, "drift")
	var hostDrift *doctor.Finding
	for i := range drift {
		if drift[i].Repo == "erp-main" {
			hostDrift = &drift[i]
		}
	}
	if hostDrift == nil {
		t.Fatalf("expected host drift finding, got %+v", fs)
	}
	if hostDrift.Branch != "feat/x" {
		t.Errorf("host drift branch = %q, want feat/x", hostDrift.Branch)
	}
}

func TestCheckHostDirtyOnNotes(t *testing.T) {
	rp, m := setupHost(t, "a")
	cloneMembers(t, rp, m)
	if _, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{}); err != nil {
		t.Fatalf("Open: %v", err)
	}
	hostWt := filepath.Join(rp.WorktreeRoot, "feat/x")
	if err := os.WriteFile(filepath.Join(hostWt, ".incident-note.md"), []byte("notes"), 0o644); err != nil {
		t.Fatal(err)
	}

	fs, err := doctor.Check(rp, m, "feat/x")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	dirty := findingsByKind(fs, "dirty")
	var found bool
	for _, f := range dirty {
		if f.Repo == "erp-main" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected host dirty finding for untracked note, got %+v", fs)
	}
}

func TestCheckHostNotDirtyOnMemberDirOnly(t *testing.T) {
	rp, m := setupHost(t, "a")
	cloneMembers(t, rp, m)
	if _, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{}); err != nil {
		t.Fatalf("Open: %v", err)
	}

	fs, err := doctor.Check(rp, m, "feat/x")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	for _, f := range findingsByKind(fs, "dirty") {
		if f.Repo == "erp-main" {
			t.Errorf("host must not be dirty when only nested member dirs present, got %+v", f)
		}
	}
}

func TestCheckHostBehind(t *testing.T) {
	rp, m := setupHost(t, "a")
	cloneMembers(t, rp, m)
	if _, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{}); err != nil {
		t.Fatalf("Open: %v", err)
	}

	seed := filepath.Join(t.TempDir(), "seed-host-advance")
	gitCmd(t, filepath.Dir(seed), "clone", m.Host.Remote, seed)
	if err := os.WriteFile(filepath.Join(seed, "adv.txt"), []byte("z"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, seed, "add", ".")
	gitCmd(t, seed, "commit", "-m", "advance")
	gitCmd(t, seed, "push", "origin", "main")
	gitCmd(t, rp.CloneRoot, "fetch", "origin", "main")

	fs, err := doctor.Check(rp, m, "feat/x")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	behind := findingsByKind(fs, "behind")
	var found bool
	for _, f := range behind {
		if f.Repo == "erp-main" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected host behind finding, got %+v", fs)
	}
}

func TestCheckHostMissingWorktreeTolerated(t *testing.T) {
	rp, m := setupHost(t, "a")
	cloneMembers(t, rp, m)

	fs, err := doctor.Check(rp, m, "feat/never")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(fs) != 0 {
		t.Errorf("missing host worktree must be tolerated, got %+v", fs)
	}
}

func TestCheckHostMemberUnderReposDetected(t *testing.T) {
	rp, m := setupHost(t, "a")
	cloneMembers(t, rp, m)
	if _, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{}); err != nil {
		t.Fatalf("Open: %v", err)
	}

	memberWt := workspace.MemberWorktreePath(rp, m, "feat/x", "a")
	wantWt := filepath.Join(rp.WorktreeRoot, "feat/x", "repos", "a")
	if memberWt != wantWt {
		t.Fatalf("member worktree path = %q, want %q", memberWt, wantWt)
	}
	if !git.IsRepo(memberWt) {
		t.Fatalf("member worktree %s not created under repos/", memberWt)
	}
	if err := os.WriteFile(filepath.Join(memberWt, "dirty.txt"), []byte("d"), 0o644); err != nil {
		t.Fatal(err)
	}

	fs, err := doctor.Check(rp, m, "feat/x")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	var found bool
	for _, f := range findingsByKind(fs, "dirty") {
		if f.Repo == "a" {
			found = true
		}
	}
	if !found {
		t.Fatalf("doctor should detect dirty member under repos/, got %+v", fs)
	}
}

func TestCheckPrunableDeduplicatedAcrossBranches(t *testing.T) {
	rp, m := setup(t, "a")
	cloneAll(t, rp, m)
	if _, err := workspace.Open(rp, m, "feat/x", workspace.OpenOptions{}); err != nil {
		t.Fatalf("Open feat/x: %v", err)
	}
	if _, err := workspace.Open(rp, m, "feat/y", workspace.OpenOptions{}); err != nil {
		t.Fatalf("Open feat/y: %v", err)
	}
	if err := os.RemoveAll(rp.WorktreePath("feat/x", "a")); err != nil {
		t.Fatal(err)
	}

	fs, err := doctor.Check(rp, m, "")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(findingsByKind(fs, "prunable")) != 1 {
		t.Errorf("prunable for one clone must be reported once, got %+v", fs)
	}
}
