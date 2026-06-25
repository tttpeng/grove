package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var groveBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "grove-bin-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	bin := filepath.Join(dir, "grove")
	root, err := filepath.Abs("..")
	if err != nil {
		panic(err)
	}
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = root
	build.Env = os.Environ()
	if out, err := build.CombinedOutput(); err != nil {
		panic("go build failed: " + err.Error() + "\n" + string(out))
	}
	groveBin = bin
	os.Exit(m.Run())
}

func gitEnv(home string) []string {
	return append(os.Environ(),
		"HOME="+home,
		"GIT_AUTHOR_NAME=grove-e2e",
		"GIT_AUTHOR_EMAIL=grove-e2e@test",
		"GIT_COMMITTER_NAME=grove-e2e",
		"GIT_COMMITTER_EMAIL=grove-e2e@test",
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
	)
}

func runGit(t *testing.T, home, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = gitEnv(home)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func seedBareRepo(t *testing.T, home, barePath string, files map[string]string) string {
	t.Helper()
	work, err := os.MkdirTemp("", "grove-seed-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(work) })
	runGit(t, home, work, "init", "-b", "main")
	for name, body := range files {
		full := filepath.Join(work, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runGit(t, home, work, "add", ".")
	runGit(t, home, work, "commit", "-m", "seed")
	if err := os.MkdirAll(filepath.Dir(barePath), 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, home, work, "clone", "--bare", work, barePath)
	abs, err := filepath.Abs(barePath)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

func runGrove(t *testing.T, home, cfg string, args ...string) (string, error) {
	t.Helper()
	full := append([]string{"--config", cfg}, args...)
	cmd := exec.Command(groveBin, full...)
	cmd.Env = gitEnv(home)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func e2eLineHas(out string, subs ...string) bool {
	for _, line := range strings.Split(out, "\n") {
		all := true
		for _, s := range subs {
			if !strings.Contains(line, s) {
				all = false
				break
			}
		}
		if all {
			return true
		}
	}
	return false
}

func TestFullLifecycle(t *testing.T) {
	base := t.TempDir()
	home := filepath.Join(base, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(base, "config.yaml")
	if err := os.WriteFile(cfg, []byte("projects: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	origins := filepath.Join(base, "origins")
	clones := filepath.Join(base, "clones")

	repoBBare := filepath.Join(origins, "repo-b.git")
	seedBareRepo(t, home, repoBBare, map[string]string{"README.md": "repo-b\n"})

	indexBare := filepath.Join(origins, "repo-a.git")
	workspaceYAML := "project: proj\n" +
		"defaultBaseline: main\n" +
		"repos:\n" +
		"  - name: repo-a\n" +
		"    remote: " + indexBare + "\n" +
		"  - name: repo-b\n" +
		"    remote: " + repoBBare + "\n"
	seedBareRepo(t, home, indexBare, map[string]string{
		"workspace.yaml": workspaceYAML,
		"README.md":      "repo-a\n",
	})

	worktreeRoot := filepath.Join(home, ".grove", "proj", "trees")

	out, err := runGrove(t, home, cfg, "project", "add", "proj", "--from", indexBare, "--clone-root", clones)
	if err != nil {
		t.Fatalf("project add failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "proj") {
		t.Errorf("project add output missing project name: %q", out)
	}
	if _, err := os.Stat(filepath.Join(clones, "repo-a", ".git")); err != nil {
		t.Errorf("index repo should clone to clones/repo-a: %v", err)
	}
	data, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatalf("config not written: %v", err)
	}
	if !strings.Contains(string(data), "proj:") {
		t.Errorf("config missing proj registration: %q", data)
	}

	out, err = runGrove(t, home, cfg, "bootstrap")
	if err != nil {
		t.Fatalf("bootstrap failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "skipped repo-a") {
		t.Errorf("bootstrap should reuse repo-a as skipped: %q", out)
	}
	if !strings.Contains(out, "cloned  repo-b") {
		t.Errorf("bootstrap should clone repo-b: %q", out)
	}
	if _, err := os.Stat(filepath.Join(clones, "repo-b", ".git")); err != nil {
		t.Errorf("repo-b not cloned: %v", err)
	}

	out, err = runGrove(t, home, cfg, "open", "feat/x", "--no-fetch")
	if err != nil {
		t.Fatalf("open failed: %v\n%s", err, out)
	}
	for _, repo := range []string{"repo-a", "repo-b"} {
		wt := filepath.Join(worktreeRoot, "feat/x", repo)
		if _, err := os.Stat(filepath.Join(wt, ".git")); err != nil {
			t.Errorf("worktree %s not created: %v", repo, err)
		}
		branch := currentBranch(t, home, wt)
		if branch != "feat/x" {
			t.Errorf("worktree %s branch = %q, want feat/x", repo, branch)
		}
	}

	out, err = runGrove(t, home, cfg, "ls")
	if err != nil {
		t.Fatalf("ls failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "feat/x") {
		t.Errorf("ls output missing feat/x: %q", out)
	}

	out, err = runGrove(t, home, cfg, "status", "feat/x")
	if err != nil {
		t.Fatalf("status failed: %v\n%s", err, out)
	}
	if strings.Contains(out, "\t") {
		t.Errorf("status output must not contain tabs: %q", out)
	}
	for _, repo := range []string{"repo-a", "repo-b"} {
		if !e2eLineHas(out, repo, "feat/x", "clean", "ahead 0") {
			t.Errorf("status output missing aligned per-repo line for %q in: %q", repo, out)
		}
	}

	out, err = runGrove(t, home, cfg, "doctor")
	if err != nil {
		t.Fatalf("doctor on healthy workspace should exit 0: %v\n%s", err, out)
	}
	if !strings.Contains(out, "无问题") {
		t.Errorf("doctor should report clean: %q", out)
	}

	zombie := filepath.Join(worktreeRoot, "feat/x", "repo-b")
	if err := os.RemoveAll(zombie); err != nil {
		t.Fatal(err)
	}

	out, err = runGrove(t, home, cfg, "doctor")
	if err == nil {
		t.Fatalf("doctor with zombie worktree should exit non-zero: %q", out)
	}
	if !strings.Contains(out, "prunable") {
		t.Errorf("doctor should report prunable: %q", out)
	}

	out, err = runGrove(t, home, cfg, "prune")
	if err != nil {
		t.Fatalf("prune failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "repo-b") {
		t.Errorf("prune should clean repo-b zombie: %q", out)
	}

	out, err = runGrove(t, home, cfg, "doctor")
	if err != nil {
		t.Fatalf("doctor after prune should exit 0: %v\n%s", err, out)
	}
	if !strings.Contains(out, "无问题") {
		t.Errorf("doctor after prune should report clean: %q", out)
	}

	out, err = runGrove(t, home, cfg, "close", "feat/x")
	if err != nil {
		t.Fatalf("close failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "removed") {
		t.Errorf("close output missing removed: %q", out)
	}
	wtA := filepath.Join(worktreeRoot, "feat/x", "repo-a")
	if _, err := os.Stat(wtA); !os.IsNotExist(err) {
		t.Errorf("repo-a worktree should be removed, stat err = %v", err)
	}

	out, err = runGrove(t, home, cfg, "doctor")
	if err != nil {
		t.Fatalf("doctor at end should exit 0: %v\n%s", err, out)
	}
	if !strings.Contains(out, "无问题") {
		t.Errorf("doctor at end should report clean: %q", out)
	}
}

func gitCapture(t *testing.T, home, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = gitEnv(home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestHostLifecycle(t *testing.T) {
	base := t.TempDir()
	home := filepath.Join(base, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	origins := filepath.Join(base, "origins")
	cloneRoot := filepath.Join(base, "clones")
	worktreeRoot := filepath.Join(base, "trees")

	hostBare := seedBareRepo(t, home, filepath.Join(origins, "super.git"), map[string]string{"HOST.md": "super\n"})
	modABare := seedBareRepo(t, home, filepath.Join(origins, "mod-a.git"), map[string]string{"README.md": "mod-a\n"})
	modBBare := seedBareRepo(t, home, filepath.Join(origins, "mod-b.git"), map[string]string{"README.md": "mod-b\n"})

	manifestPath := filepath.Join(base, "workspace.yaml")
	workspaceYAML := "project: proj\n" +
		"defaultBaseline: main\n" +
		"host:\n" +
		"  name: super\n" +
		"  remote: " + hostBare + "\n" +
		"repos:\n" +
		"  - name: mod-a\n" +
		"    remote: " + modABare + "\n" +
		"  - name: mod-b\n" +
		"    remote: " + modBBare + "\n"
	if err := os.WriteFile(manifestPath, []byte(workspaceYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := filepath.Join(base, "config.yaml")
	configYAML := "current: proj\n" +
		"projects:\n" +
		"  proj:\n" +
		"    manifest: " + manifestPath + "\n" +
		"    cloneRoot: " + cloneRoot + "\n" +
		"    worktreeRoot: " + worktreeRoot + "\n"
	if err := os.WriteFile(cfg, []byte(configYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	runGit(t, home, base, "clone", hostBare, cloneRoot)
	reposClones := filepath.Join(cloneRoot, "repos")
	if err := os.MkdirAll(reposClones, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, home, base, "clone", modABare, filepath.Join(reposClones, "mod-a"))
	runGit(t, home, base, "clone", modBBare, filepath.Join(reposClones, "mod-b"))

	out, err := runGrove(t, home, cfg, "open", "feat/x", "--no-fetch")
	if err != nil {
		t.Fatalf("open failed: %v\n%s", err, out)
	}

	groupRoot := filepath.Join(worktreeRoot, "feat/x")
	if _, err := os.Stat(groupRoot); err != nil {
		t.Fatalf("group root not created: %v", err)
	}
	commonDir := gitCapture(t, home, groupRoot, "rev-parse", "--git-common-dir")
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(groupRoot, commonDir)
	}
	wantCommon, err := filepath.EvalSymlinks(filepath.Join(cloneRoot, ".git"))
	if err != nil {
		t.Fatal(err)
	}
	gotCommon, err := filepath.EvalSymlinks(commonDir)
	if err != nil {
		t.Fatal(err)
	}
	if gotCommon != wantCommon {
		t.Errorf("group root common-dir = %q, want host clone .git %q", gotCommon, wantCommon)
	}
	if b := currentBranch(t, home, groupRoot); b != "feat/x" {
		t.Errorf("group root branch = %q, want feat/x", b)
	}

	for _, member := range []string{"mod-a", "mod-b"} {
		wt := filepath.Join(groupRoot, "repos", member)
		if _, err := os.Stat(filepath.Join(wt, ".git")); err != nil {
			t.Errorf("member worktree %s not nested in group root repos/: %v", member, err)
		}
		if b := currentBranch(t, home, wt); b != "feat/x" {
			t.Errorf("member %s branch = %q, want feat/x", member, b)
		}
	}

	out, err = runGrove(t, home, cfg, "ls")
	if err != nil {
		t.Fatalf("ls failed: %v\n%s", err, out)
	}
	if !e2eLineHas(out, "feat/x", "3") {
		t.Errorf("ls output should count host + 2 members (3) on feat/x row: %q", out)
	}
	if strings.Contains(out, "\t") {
		t.Errorf("ls output must not contain tabs: %q", out)
	}

	out, err = runGrove(t, home, cfg, "status", "feat/x")
	if err != nil {
		t.Fatalf("status failed: %v\n%s", err, out)
	}
	for _, name := range []string{"super", "mod-a", "mod-b"} {
		if !strings.Contains(out, name) {
			t.Errorf("status output missing repo %q: %q", name, out)
		}
	}

	note := filepath.Join(groupRoot, "NOTES.txt")
	if err := os.WriteFile(note, []byte("wip\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err = runGrove(t, home, cfg, "close", "feat/x")
	if err == nil {
		t.Fatalf("close should be blocked by untracked host change: %q", out)
	}
	if !strings.Contains(out, "super") {
		t.Errorf("close block output missing host name: %q", out)
	}
	if !strings.Contains(out, "未提交改动") {
		t.Errorf("close block output missing dirty reason: %q", out)
	}
	if _, err := os.Stat(groupRoot); err != nil {
		t.Errorf("blocked close must not delete group root: %v", err)
	}

	out, err = runGrove(t, home, cfg, "close", "feat/x", "--force")
	if err != nil {
		t.Fatalf("forced close failed: %v\n%s", err, out)
	}
	if _, err := os.Stat(groupRoot); !os.IsNotExist(err) {
		t.Errorf("forced close should remove group root, stat err = %v", err)
	}
}

func currentBranch(t *testing.T, home, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	cmd.Env = gitEnv(home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("rev-parse failed: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestSyncRoot(t *testing.T) {
	base := t.TempDir()
	home := filepath.Join(base, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	origins := filepath.Join(base, "origins")
	cloneRoot := filepath.Join(base, "clones")
	worktreeRoot := filepath.Join(base, "trees")

	hostBare := seedBareRepo(t, home, filepath.Join(origins, "super.git"), map[string]string{"HOST.md": "super\n"})
	modABare := seedBareRepo(t, home, filepath.Join(origins, "mod-a.git"), map[string]string{"README.md": "mod-a\n"})

	manifestPath := filepath.Join(base, "workspace.yaml")
	workspaceYAML := "project: proj\n" +
		"defaultBaseline: main\n" +
		"host:\n" +
		"  name: super\n" +
		"  remote: " + hostBare + "\n" +
		"repos:\n" +
		"  - name: mod-a\n" +
		"    remote: " + modABare + "\n"
	if err := os.WriteFile(manifestPath, []byte(workspaceYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := filepath.Join(base, "config.yaml")
	configYAML := "current: proj\n" +
		"projects:\n" +
		"  proj:\n" +
		"    manifest: " + manifestPath + "\n" +
		"    cloneRoot: " + cloneRoot + "\n" +
		"    worktreeRoot: " + worktreeRoot + "\n"
	if err := os.WriteFile(cfg, []byte(configYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	runGit(t, home, base, "clone", hostBare, cloneRoot)
	reposClones := filepath.Join(cloneRoot, "repos")
	if err := os.MkdirAll(reposClones, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, home, base, "clone", modABare, filepath.Join(reposClones, "mod-a"))

	advance := func(bare, file, body string) {
		work := filepath.Join(t.TempDir(), "adv")
		runGit(t, home, base, "clone", bare, work)
		full := filepath.Join(work, file)
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		runGit(t, home, work, "commit", "-am", "advance")
		runGit(t, home, work, "push", "origin", "main")
	}
	advance(hostBare, "HOST.md", "super-v2\n")
	advance(modABare, "README.md", "mod-a-v2\n")

	out, err := runGrove(t, home, cfg, "sync", "--root")
	if err != nil {
		t.Fatalf("sync --root failed: %v\n%s", err, out)
	}
	if !e2eLineHas(out, "super", "updated") {
		t.Errorf("host should be updated: %q", out)
	}
	if !e2eLineHas(out, "mod-a", "updated") {
		t.Errorf("mod-a should be updated: %q", out)
	}

	hostHead := gitCapture(t, home, cloneRoot, "rev-parse", "HEAD")
	hostUp := gitCapture(t, home, cloneRoot, "rev-parse", "origin/main")
	if hostHead != hostUp {
		t.Errorf("host clone HEAD %q != origin/main %q after sync --root", hostHead, hostUp)
	}

	out, err = runGrove(t, home, cfg, "ls")
	if err != nil {
		t.Fatalf("ls failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "root") {
		t.Errorf("ls output should contain root row: %q", out)
	}

	out, err = runGrove(t, home, cfg, "status", "--root")
	if err != nil {
		t.Fatalf("status --root failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "super") || !strings.Contains(out, "mod-a") {
		t.Errorf("status --root should list root repos: %q", out)
	}

	out, err = runGrove(t, home, cfg, "sync", "--root", "feat/x")
	if err == nil {
		t.Errorf("sync --root with a branch arg should error, got:\n%s", out)
	}
}

func TestSyncRootFetchFailed(t *testing.T) {
	base := t.TempDir()
	home := filepath.Join(base, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	origins := filepath.Join(base, "origins")
	cloneRoot := filepath.Join(base, "clones")
	worktreeRoot := filepath.Join(base, "trees")

	hostBare := seedBareRepo(t, home, filepath.Join(origins, "super.git"), map[string]string{"HOST.md": "super\n"})
	modABare := seedBareRepo(t, home, filepath.Join(origins, "mod-a.git"), map[string]string{"README.md": "mod-a\n"})

	manifestPath := filepath.Join(base, "workspace.yaml")
	workspaceYAML := "project: proj\n" +
		"defaultBaseline: main\n" +
		"host:\n" +
		"  name: super\n" +
		"  remote: " + hostBare + "\n" +
		"repos:\n" +
		"  - name: mod-a\n" +
		"    remote: " + modABare + "\n"
	if err := os.WriteFile(manifestPath, []byte(workspaceYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := filepath.Join(base, "config.yaml")
	configYAML := "current: proj\n" +
		"projects:\n" +
		"  proj:\n" +
		"    manifest: " + manifestPath + "\n" +
		"    cloneRoot: " + cloneRoot + "\n" +
		"    worktreeRoot: " + worktreeRoot + "\n"
	if err := os.WriteFile(cfg, []byte(configYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	runGit(t, home, base, "clone", hostBare, cloneRoot)
	reposClones := filepath.Join(cloneRoot, "repos")
	if err := os.MkdirAll(reposClones, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, home, base, "clone", modABare, filepath.Join(reposClones, "mod-a"))

	if err := os.RemoveAll(modABare); err != nil {
		t.Fatal(err)
	}

	out, err := runGrove(t, home, cfg, "sync", "--root")
	if err == nil {
		t.Fatalf("sync --root should exit non-zero when a fetch fails:\n%s", out)
	}
	if !strings.Contains(out, "mod-a") {
		t.Errorf("output should mention the failed repo: %q", out)
	}
}
