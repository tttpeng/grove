package cli_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/tttpeng/grove/cli"
	"github.com/tttpeng/grove/core/config"
)

func gitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func mkRepo(t *testing.T, dir string) {
	t.Helper()
	os.MkdirAll(dir, 0o755)
	gitCmd(t, dir, "init", "-b", "main")
	os.WriteFile(filepath.Join(dir, "f"), []byte("x"), 0o644)
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "c")
}

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := cli.NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

func TestVersionCommand(t *testing.T) {
	out, err := run(t, "version")
	if err != nil {
		t.Fatalf("version error = %v", err)
	}
	if !strings.Contains(out, "0.1.0") {
		t.Errorf("version output = %q, want substring 0.1.0", out)
	}
}

func TestProjectListShowsCurrent(t *testing.T) {
	path := writeConfig(t, "current: erp\nprojects:\n  erp:\n    manifest: /a\n  nova:\n    manifest: /b\n")
	out, err := run(t, "--config", path, "project", "list")
	if err != nil {
		t.Fatalf("project list error = %v", err)
	}
	if !strings.Contains(out, "* erp") {
		t.Errorf("missing current marker for erp: %q", out)
	}
	if !strings.Contains(out, "nova") {
		t.Errorf("missing nova: %q", out)
	}
}

func TestUseSwitchesCurrent(t *testing.T) {
	path := writeConfig(t, "current: erp\nprojects:\n  erp:\n    manifest: /a\n  nova:\n    manifest: /b\n")
	if _, err := run(t, "--config", path, "use", "nova"); err != nil {
		t.Fatalf("use error = %v", err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Current != "nova" {
		t.Errorf("Current = %q, want nova", cfg.Current)
	}
}

func TestUseUnknownErrors(t *testing.T) {
	path := writeConfig(t, "current: erp\nprojects:\n  erp:\n    manifest: /a\n")
	if _, err := run(t, "--config", path, "use", "ghost"); err == nil {
		t.Error("use ghost expected error")
	}
}

func TestProjectRemoveDeletesEntry(t *testing.T) {
	path := writeConfig(t, "current: erp\nprojects:\n  erp:\n    manifest: /a\n  nova:\n    manifest: /b\n")
	if _, err := run(t, "--config", path, "project", "remove", "nova"); err != nil {
		t.Fatalf("project remove error = %v", err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.Projects["nova"]; ok {
		t.Error("nova should be removed")
	}
	if cfg.Current != "erp" {
		t.Errorf("Current = %q, want erp", cfg.Current)
	}
}

func TestInitWritesWorkspace(t *testing.T) {
	work := t.TempDir()
	a := filepath.Join(work, "repo-a")
	mkRepo(t, a)
	origin := filepath.Join(t.TempDir(), "o")
	mkRepo(t, origin)
	gitCmd(t, a, "remote", "add", "origin", origin)
	t.Chdir(work)

	out, err := run(t, "init")
	if err != nil {
		t.Fatalf("init error = %v", err)
	}
	if !strings.Contains(out, "1") {
		t.Errorf("init output should mention repo count: %q", out)
	}
	data, err := os.ReadFile(filepath.Join(work, "workspace.yaml"))
	if err != nil {
		t.Fatalf("workspace.yaml not written: %v", err)
	}
	if !strings.Contains(string(data), "repo-a") {
		t.Errorf("workspace.yaml missing repo-a: %q", data)
	}
	if _, err := run(t, "init"); err == nil {
		t.Error("init should error when workspace.yaml exists")
	}
}

func TestInitDetectsHost(t *testing.T) {
	work := t.TempDir()
	mkRepo(t, work)
	hostOrigin := filepath.Join(t.TempDir(), "host-o")
	mkRepo(t, hostOrigin)
	gitCmd(t, work, "remote", "add", "origin", hostOrigin)

	member := filepath.Join(work, "repos", "repo-a")
	mkRepo(t, member)
	memberOrigin := filepath.Join(t.TempDir(), "member-o")
	mkRepo(t, memberOrigin)
	gitCmd(t, member, "remote", "add", "origin", memberOrigin)

	t.Chdir(work)
	if _, err := run(t, "init"); err != nil {
		t.Fatalf("init error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(work, "workspace.yaml"))
	if err != nil {
		t.Fatalf("workspace.yaml not written: %v", err)
	}
	body := string(data)
	if !strings.Contains(body, "host:") {
		t.Errorf("workspace.yaml missing host section: %q", body)
	}
	base := filepath.Base(work)
	if !strings.Contains(body, "name: "+base) && !strings.Contains(body, "name: \""+base+"\"") {
		t.Errorf("host name should be cwd basename %q: %q", base, body)
	}
	if !strings.Contains(body, hostOrigin) {
		t.Errorf("host remote should be %q: %q", hostOrigin, body)
	}
	if !strings.Contains(body, "repo-a") {
		t.Errorf("workspace.yaml missing member repo-a: %q", body)
	}
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
	os.WriteFile(filepath.Join(seed, "README.md"), []byte("x\n"), 0o644)
	gitCmd(t, seed, "add", ".")
	gitCmd(t, seed, "commit", "-m", "init")
	gitCmd(t, seed, "push", "origin", "main")
	return bare
}

func setupWorkspaceProject(t *testing.T, repoNames ...string) (string, string) {
	t.Helper()
	root := t.TempDir()
	cloneRoot := filepath.Join(root, "clones")
	worktreeRoot := filepath.Join(root, "trees")
	manifestPath := filepath.Join(root, "workspace.yaml")

	body := "project: p\ndefaultBaseline: main\nrepos:\n"
	for _, name := range repoNames {
		bare := makeBareOrigin(t, root, name)
		body += "  - name: " + name + "\n    remote: " + bare + "\n    baseline: main\n"
	}
	if err := os.WriteFile(manifestPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	cfgBody := "current: p\nprojects:\n  p:\n    manifest: " + manifestPath +
		"\n    cloneRoot: " + cloneRoot + "\n    worktreeRoot: " + worktreeRoot + "\n"
	cfgPath := writeConfig(t, cfgBody)

	if _, err := run(t, "--config", cfgPath, "bootstrap"); err != nil {
		t.Fatalf("bootstrap error = %v", err)
	}
	return cfgPath, worktreeRoot
}

func TestOpenLsStatusCloseLifecycle(t *testing.T) {
	cfgPath, worktreeRoot := setupWorkspaceProject(t, "a", "b")

	out, err := run(t, "--config", cfgPath, "open", "feat/x")
	if err != nil {
		t.Fatalf("open error = %v\n%s", err, out)
	}
	if !strings.Contains(out, "created") {
		t.Errorf("open output missing created: %q", out)
	}
	wtA := filepath.Join(worktreeRoot, "feat/x", "a")
	if _, err := os.Stat(filepath.Join(wtA, ".git")); err != nil {
		t.Errorf("worktree a not created: %v", err)
	}

	out, err = run(t, "--config", cfgPath, "ls")
	if err != nil {
		t.Fatalf("ls error = %v\n%s", err, out)
	}
	if !strings.Contains(out, "feat/x") {
		t.Errorf("ls output missing feat/x: %q", out)
	}

	out, err = run(t, "--config", cfgPath, "status", "feat/x")
	if err != nil {
		t.Fatalf("status error = %v\n%s", err, out)
	}
	if strings.Contains(out, "\t") {
		t.Errorf("status output must not contain tabs: %q", out)
	}
	for _, repo := range []string{"a", "b"} {
		if !lineMatching(out, repo, "feat/x", "clean", "ahead 0", "behind 0") {
			t.Errorf("status output missing aligned per-repo line for %q in: %q", repo, out)
		}
	}
	if !strings.Contains(out, "behind") {
		t.Errorf("status header/rows should include behind column: %q", out)
	}

	out, err = run(t, "--config", cfgPath, "close", "feat/x")
	if err != nil {
		t.Fatalf("close error = %v\n%s", err, out)
	}
	if !strings.Contains(out, "removed") {
		t.Errorf("close output missing removed: %q", out)
	}
	if _, err := os.Stat(wtA); !os.IsNotExist(err) {
		t.Errorf("worktree a should be removed, stat err = %v", err)
	}
}

func TestStatusNoArgEqualsLs(t *testing.T) {
	cfgPath, _ := setupWorkspaceProject(t, "a")
	if _, err := run(t, "--config", cfgPath, "open", "feat/y"); err != nil {
		t.Fatalf("open error = %v", err)
	}
	out, err := run(t, "--config", cfgPath, "status")
	if err != nil {
		t.Fatalf("status error = %v\n%s", err, out)
	}
	if !strings.Contains(out, "feat/y") {
		t.Errorf("status (no arg) should list workspaces: %q", out)
	}
}

func TestLsEmpty(t *testing.T) {
	cfgPath, _ := setupWorkspaceProject(t, "a")
	out, err := run(t, "--config", cfgPath, "ls")
	if err != nil {
		t.Fatalf("ls error = %v\n%s", err, out)
	}
	if !strings.Contains(out, "无 workspace") {
		t.Errorf("ls empty output = %q", out)
	}
}

func TestOpenNoCurrentErrors(t *testing.T) {
	path := writeConfig(t, "current: \"\"\nprojects: {}\n")
	if _, err := run(t, "--config", path, "open", "feat/z"); err == nil {
		t.Error("open without current project should error")
	}
}

func TestBootstrapNoCurrentMatchesResolveCurrent(t *testing.T) {
	path := writeConfig(t, "current: \"\"\nprojects: {}\n")
	_, bootErr := run(t, "--config", path, "bootstrap")
	if bootErr == nil {
		t.Fatal("bootstrap without current project should error")
	}
	_, openErr := run(t, "--config", path, "open", "feat/z")
	if openErr == nil {
		t.Fatal("open without current project should error")
	}
	if bootErr.Error() != openErr.Error() {
		t.Errorf("bootstrap error %q should match resolveCurrent error %q", bootErr, openErr)
	}
}

func setupProjectWithBaselines(t *testing.T, repos map[string]string) (string, string) {
	t.Helper()
	root := t.TempDir()
	cloneRoot := filepath.Join(root, "clones")
	worktreeRoot := filepath.Join(root, "trees")
	manifestPath := filepath.Join(root, "workspace.yaml")

	names := make([]string, 0, len(repos))
	for name := range repos {
		names = append(names, name)
	}
	sort.Strings(names)

	body := "project: p\ndefaultBaseline: main\nrepos:\n"
	for _, name := range names {
		bare := makeBareOrigin(t, root, name)
		body += "  - name: " + name + "\n    remote: " + bare + "\n    baseline: " + repos[name] + "\n"
	}
	if err := os.WriteFile(manifestPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	cfgBody := "current: p\nprojects:\n  p:\n    manifest: " + manifestPath +
		"\n    cloneRoot: " + cloneRoot + "\n    worktreeRoot: " + worktreeRoot + "\n"
	cfgPath := writeConfig(t, cfgBody)

	if _, err := run(t, "--config", cfgPath, "bootstrap"); err != nil {
		t.Fatalf("bootstrap error = %v", err)
	}
	return cfgPath, worktreeRoot
}

func TestOpenRollbackOutputNotMisleading(t *testing.T) {
	cfgPath, worktreeRoot := setupProjectWithBaselines(t, map[string]string{
		"a": "main",
		"b": "does-not-exist",
	})

	out, err := run(t, "--config", cfgPath, "open", "feat/x", "--no-fetch")
	if err == nil {
		t.Fatalf("open should fail when repo b baseline missing, out = %q", out)
	}
	if strings.Contains(out, "created a") {
		t.Errorf("rolled-back repo a must not be reported as created: %q", out)
	}
	if !strings.Contains(out, "回滚") {
		t.Errorf("rolled-back repo a should be reported as 回滚: %q", out)
	}
	wtA := filepath.Join(worktreeRoot, "feat/x", "a")
	if _, err := os.Stat(wtA); !os.IsNotExist(err) {
		t.Errorf("repo a worktree should be rolled back, stat err = %v", err)
	}
}

func TestDoctorPruneRoundtrip(t *testing.T) {
	cfgPath, worktreeRoot := setupWorkspaceProject(t, "a", "b")

	if _, err := run(t, "--config", cfgPath, "open", "feat/x"); err != nil {
		t.Fatalf("open error = %v", err)
	}

	wtA := filepath.Join(worktreeRoot, "feat/x", "a")
	if err := os.RemoveAll(wtA); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "--config", cfgPath, "doctor")
	if err == nil {
		t.Fatalf("doctor with prunable should return error, out = %q", out)
	}
	if !strings.Contains(out, "prunable:\n") {
		t.Errorf("doctor output should have prunable kind header: %q", out)
	}
	if strings.Contains(out, "\t") {
		t.Errorf("doctor output must not contain tabs: %q", out)
	}
	if !lineMatching(out, "a", "僵尸 worktree") {
		t.Errorf("doctor output should report repo a as prunable finding: %q", out)
	}

	out, err = run(t, "--config", cfgPath, "prune")
	if err != nil {
		t.Fatalf("prune error = %v\n%s", err, out)
	}

	out, err = run(t, "--config", cfgPath, "doctor")
	if err != nil {
		t.Fatalf("doctor after prune should be clean, err = %v\n%s", err, out)
	}
	if !strings.Contains(out, "无问题") {
		t.Errorf("doctor after prune should report clean: %q", out)
	}
}

func TestPruneNoZombies(t *testing.T) {
	cfgPath, _ := setupWorkspaceProject(t, "a")
	if _, err := run(t, "--config", cfgPath, "open", "feat/x"); err != nil {
		t.Fatalf("open error = %v", err)
	}
	out, err := run(t, "--config", cfgPath, "prune")
	if err != nil {
		t.Fatalf("prune error = %v\n%s", err, out)
	}
	if !strings.Contains(out, "无僵尸 worktree") {
		t.Errorf("prune with nothing to clean: %q", out)
	}
}

func TestProjectAddAndBootstrap(t *testing.T) {
	work := t.TempDir()
	origin := filepath.Join(work, "origin-a")
	mkRepo(t, origin)

	index := filepath.Join(work, "erp-main.git")
	os.MkdirAll(index, 0o755)
	gitCmd(t, index, "init", "-b", "main")
	ws := "project: erp\ndefaultBaseline: main\nrepos:\n  - name: erp-main\n    remote: " + index + "\n  - name: a\n    remote: " + origin + "\n"
	if err := os.WriteFile(filepath.Join(index, "workspace.yaml"), []byte(ws), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, index, "add", ".")
	gitCmd(t, index, "commit", "-m", "ws")

	cloneRoot := filepath.Join(work, "clones")
	path := writeConfig(t, "projects: {}\n")
	out, err := run(t, "--config", path, "project", "add", "erp", "--from", index, "--clone-root", cloneRoot)
	if err != nil {
		t.Fatalf("project add error = %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(cloneRoot, "erp-main", ".git")); err != nil {
		t.Errorf("index repo should clone to <cloneRoot>/erp-main (url basename): %v", err)
	}
	if _, err := os.Stat(filepath.Join(cloneRoot, "erp", ".git")); err == nil {
		t.Error("index repo must not use project name erp as dir")
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.Projects["erp"]; !ok {
		t.Fatalf("erp not registered: %+v", cfg.Projects)
	}
	if cfg.Projects["erp"].CloneRoot != cloneRoot {
		t.Errorf("CloneRoot = %q, want %q", cfg.Projects["erp"].CloneRoot, cloneRoot)
	}
	if cfg.Current != "erp" {
		t.Errorf("Current = %q, want erp", cfg.Current)
	}

	out, err = run(t, "--config", path, "bootstrap")
	if err != nil {
		t.Fatalf("bootstrap error = %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(cloneRoot, "a", ".git")); err != nil {
		t.Errorf("repo a not cloned: %v", err)
	}
	if !strings.Contains(out, "skipped erp-main") {
		t.Errorf("host repo erp-main should be skipped (reused): %q", out)
	}
}

func lineMatching(out string, subs ...string) bool {
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

func TestOpenWithDescriptionShownInLs(t *testing.T) {
	cfgPath, _ := setupWorkspaceProject(t, "a")
	if _, err := run(t, "--config", cfgPath, "open", "feat/x", "-m", "修复登录"); err != nil {
		t.Fatalf("open -m error = %v", err)
	}
	out, err := run(t, "--config", cfgPath, "ls")
	if err != nil {
		t.Fatalf("ls error = %v\n%s", err, out)
	}
	if !strings.Contains(out, "修复登录") {
		t.Errorf("ls should show description 修复登录: %q", out)
	}
	if strings.Contains(out, "\t") {
		t.Errorf("ls output must not contain tabs: %q", out)
	}
}

func TestDescribeUpdatesDescriptionReflectedInLs(t *testing.T) {
	cfgPath, _ := setupWorkspaceProject(t, "a")
	if _, err := run(t, "--config", cfgPath, "open", "feat/x", "-m", "旧描述"); err != nil {
		t.Fatalf("open -m error = %v", err)
	}
	out, err := run(t, "--config", cfgPath, "describe", "feat/x", "新描述")
	if err != nil {
		t.Fatalf("describe error = %v\n%s", err, out)
	}
	if !strings.Contains(out, "feat/x") || !strings.Contains(out, "新描述") {
		t.Errorf("describe confirmation should mention branch and new description: %q", out)
	}
	out, err = run(t, "--config", cfgPath, "ls")
	if err != nil {
		t.Fatalf("ls error = %v\n%s", err, out)
	}
	if !strings.Contains(out, "新描述") {
		t.Errorf("ls should reflect updated description: %q", out)
	}
	if strings.Contains(out, "旧描述") {
		t.Errorf("ls should not show stale description: %q", out)
	}
}

func TestLsEmptyDescriptionShowsDash(t *testing.T) {
	cfgPath, _ := setupWorkspaceProject(t, "a")
	if _, err := run(t, "--config", cfgPath, "open", "feat/x"); err != nil {
		t.Fatalf("open error = %v", err)
	}
	out, err := run(t, "--config", cfgPath, "ls")
	if err != nil {
		t.Fatalf("ls error = %v\n%s", err, out)
	}
	if !strings.Contains(out, "—") {
		t.Errorf("ls empty description should show em dash: %q", out)
	}
}

func TestLsColumnsAligned(t *testing.T) {
	cfgPath, _ := setupWorkspaceProject(t, "a")
	if _, err := run(t, "--config", cfgPath, "open", "feat/x", "-m", "中文描述"); err != nil {
		t.Fatalf("open error = %v", err)
	}
	if _, err := run(t, "--config", cfgPath, "open", "feature/much-longer-branch"); err != nil {
		t.Fatalf("open error = %v", err)
	}
	out, err := run(t, "--config", cfgPath, "ls")
	if err != nil {
		t.Fatalf("ls error = %v\n%s", err, out)
	}
	if strings.Contains(out, "\t") {
		t.Errorf("ls output must not contain tabs: %q", out)
	}
	rows := branchLines(out, "feat/x", "feature/much-longer-branch")
	if len(rows) != 2 {
		t.Fatalf("expected 2 workspace rows, got %d from %q", len(rows), out)
	}
	s0 := descColumnStart(rows[0], "中文描述")
	s1 := descColumnStart(rows[1], "—")
	if s0 != s1 {
		t.Errorf("description column misaligned: %d vs %d (%q / %q)", s0, s1, rows[0], rows[1])
	}
}

func TestStatusColumnsAligned(t *testing.T) {
	cfgPath, _ := setupWorkspaceProject(t, "short", "a-very-long-repo-name")
	if _, err := run(t, "--config", cfgPath, "open", "feat/x"); err != nil {
		t.Fatalf("open error = %v", err)
	}
	out, err := run(t, "--config", cfgPath, "status", "feat/x")
	if err != nil {
		t.Fatalf("status error = %v\n%s", err, out)
	}
	if strings.Contains(out, "\t") {
		t.Errorf("status output must not contain tabs: %q", out)
	}
	rows := branchLines(out, "short", "a-very-long-repo-name")
	if len(rows) != 2 {
		t.Fatalf("expected 2 repo rows, got %d from %q", len(rows), out)
	}
	s0 := descColumnStart(rows[0], "ahead")
	s1 := descColumnStart(rows[1], "ahead")
	if s0 != s1 {
		t.Errorf("ahead column misaligned: %d vs %d (%q / %q)", s0, s1, rows[0], rows[1])
	}
	b0 := descColumnStart(rows[0], "behind")
	b1 := descColumnStart(rows[1], "behind")
	if b0 != b1 {
		t.Errorf("behind column misaligned: %d vs %d (%q / %q)", b0, b1, rows[0], rows[1])
	}
}

func branchLines(out string, keys ...string) []string {
	var rows []string
	for _, line := range strings.Split(out, "\n") {
		for _, k := range keys {
			if strings.Contains(line, k) {
				rows = append(rows, line)
				break
			}
		}
	}
	return rows
}

func descColumnStart(line, marker string) int {
	idx := strings.Index(line, marker)
	if idx < 0 {
		return -1
	}
	return lipgloss.Width(line[:idx])
}

func setupLabeledWorkspaceProject(t *testing.T, labels map[string]string, repoNames ...string) (string, string) {
	t.Helper()
	root := t.TempDir()
	cloneRoot := filepath.Join(root, "clones")
	worktreeRoot := filepath.Join(root, "trees")
	manifestPath := filepath.Join(root, "workspace.yaml")

	body := "project: p\ndefaultBaseline: main\nrepos:\n"
	for _, name := range repoNames {
		bare := makeBareOrigin(t, root, name)
		body += "  - name: " + name + "\n    remote: " + bare + "\n    baseline: main\n"
		if lbl := labels[name]; lbl != "" {
			body += "    label: " + lbl + "\n"
		}
	}
	if err := os.WriteFile(manifestPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	cfgBody := "current: p\nprojects:\n  p:\n    manifest: " + manifestPath +
		"\n    cloneRoot: " + cloneRoot + "\n    worktreeRoot: " + worktreeRoot + "\n"
	cfgPath := writeConfig(t, cfgBody)

	if _, err := run(t, "--config", cfgPath, "bootstrap"); err != nil {
		t.Fatalf("bootstrap error = %v", err)
	}
	return cfgPath, worktreeRoot
}

func setupSyncProject(t *testing.T, repoNames ...string) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	cloneRoot := filepath.Join(root, "clones")
	worktreeRoot := filepath.Join(root, "trees")
	manifestPath := filepath.Join(root, "workspace.yaml")

	body := "project: p\ndefaultBaseline: main\nrepos:\n"
	for _, name := range repoNames {
		bare := makeBareOrigin(t, root, name)
		body += "  - name: " + name + "\n    remote: " + bare + "\n    baseline: main\n"
	}
	if err := os.WriteFile(manifestPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgBody := "current: p\nprojects:\n  p:\n    manifest: " + manifestPath +
		"\n    cloneRoot: " + cloneRoot + "\n    worktreeRoot: " + worktreeRoot + "\n"
	cfgPath := writeConfig(t, cfgBody)
	if _, err := run(t, "--config", cfgPath, "bootstrap"); err != nil {
		t.Fatalf("bootstrap error = %v", err)
	}
	return cfgPath, worktreeRoot, root
}

func pushToOrigin(t *testing.T, root, name, content string) {
	t.Helper()
	bare := filepath.Join(root, "origins", name+".git")
	work := filepath.Join(t.TempDir(), "push-"+name)
	gitCmd(t, t.TempDir(), "clone", bare, work)
	os.WriteFile(filepath.Join(work, "README.md"), []byte(content), 0o644)
	gitCmd(t, work, "add", ".")
	gitCmd(t, work, "commit", "-m", "upstream")
	gitCmd(t, work, "push", "origin", "main")
}

func TestSyncSyncedOutput(t *testing.T) {
	cfgPath, worktreeRoot, root := setupSyncProject(t, "a", "b")
	if _, err := run(t, "--config", cfgPath, "open", "feat/x"); err != nil {
		t.Fatalf("open error = %v", err)
	}
	pushToOrigin(t, root, "a", "upstream change\n")

	out, err := run(t, "--config", cfgPath, "sync", "feat/x")
	if err != nil {
		t.Fatalf("sync error = %v\n%s", err, out)
	}
	if strings.Contains(out, "\t") {
		t.Errorf("sync output must not contain tabs: %q", out)
	}
	if !strings.Contains(out, "名称") || !strings.Contains(out, "结果") {
		t.Errorf("sync header should include 名称/结果 columns: %q", out)
	}
	if !lineMatching(out, "a", "synced") {
		t.Errorf("repo a should be synced: %q", out)
	}
	if !lineMatching(out, "b", "up-to-date") {
		t.Errorf("repo b should be up-to-date: %q", out)
	}
	_ = worktreeRoot
}

func TestSyncConflictNonZeroExit(t *testing.T) {
	cfgPath, worktreeRoot, root := setupSyncProject(t, "a")
	if _, err := run(t, "--config", cfgPath, "open", "feat/x"); err != nil {
		t.Fatalf("open error = %v", err)
	}
	wtA := filepath.Join(worktreeRoot, "feat/x", "a")
	os.WriteFile(filepath.Join(wtA, "README.md"), []byte("local edit\n"), 0o644)
	gitCmd(t, wtA, "add", ".")
	gitCmd(t, wtA, "commit", "-m", "local")
	pushToOrigin(t, root, "a", "upstream edit\n")

	out, err := run(t, "--config", cfgPath, "sync", "feat/x")
	if err == nil {
		t.Fatalf("sync with conflict should return error, out = %q", out)
	}
	if !lineMatching(out, "a", "conflict") {
		t.Errorf("repo a should be conflict: %q", out)
	}
}

func TestSyncCwdDetectsBranch(t *testing.T) {
	cfgPath, worktreeRoot, root := setupSyncProject(t, "a")
	if _, err := run(t, "--config", cfgPath, "open", "feat/x"); err != nil {
		t.Fatalf("open error = %v", err)
	}
	pushToOrigin(t, root, "a", "upstream change\n")

	t.Chdir(filepath.Join(worktreeRoot, "feat/x", "a"))
	out, err := run(t, "--config", cfgPath, "sync")
	if err != nil {
		t.Fatalf("sync (cwd) error = %v\n%s", err, out)
	}
	if !lineMatching(out, "a", "synced") {
		t.Errorf("cwd-detected sync should sync repo a: %q", out)
	}
}

func TestSyncNoArgOutsideWorktreeErrors(t *testing.T) {
	cfgPath, _, _ := setupSyncProject(t, "a")
	t.Chdir(t.TempDir())
	if _, err := run(t, "--config", cfgPath, "sync"); err == nil {
		t.Error("sync without branch outside any worktree should error")
	}
}

func TestStatusShowsLabelColumn(t *testing.T) {
	cfgPath, _ := setupLabeledWorkspaceProject(t, map[string]string{"a": "店长管理小程序"}, "a", "b")
	if _, err := run(t, "--config", cfgPath, "open", "feat/x"); err != nil {
		t.Fatalf("open error = %v", err)
	}
	out, err := run(t, "--config", cfgPath, "status", "feat/x")
	if err != nil {
		t.Fatalf("status error = %v\n%s", err, out)
	}
	if strings.Contains(out, "\t") {
		t.Errorf("status output must not contain tabs: %q", out)
	}
	if !lineMatching(out, "店长管理小程序", "a", "feat/x", "clean", "ahead 0") {
		t.Errorf("status should show label 店长管理小程序 + repo a row: %q", out)
	}
	if !lineMatching(out, "b", "feat/x", "clean", "ahead 0") {
		t.Errorf("status should fall back to repo name b when no label: %q", out)
	}
	if !strings.Contains(out, "名称") || !strings.Contains(out, "仓库") {
		t.Errorf("status header should include 名称 and 仓库 columns: %q", out)
	}
}

func TestStatusLabelColumnsAligned(t *testing.T) {
	cfgPath, _ := setupLabeledWorkspaceProject(t, map[string]string{"short": "短名"}, "short", "a-very-long-repo-name")
	if _, err := run(t, "--config", cfgPath, "open", "feat/x"); err != nil {
		t.Fatalf("open error = %v", err)
	}
	out, err := run(t, "--config", cfgPath, "status", "feat/x")
	if err != nil {
		t.Fatalf("status error = %v\n%s", err, out)
	}
	rows := branchLines(out, "短名", "a-very-long-repo-name")
	if len(rows) != 2 {
		t.Fatalf("expected 2 repo rows, got %d from %q", len(rows), out)
	}
	s0 := descColumnStart(rows[0], "ahead")
	s1 := descColumnStart(rows[1], "ahead")
	if s0 != s1 {
		t.Errorf("ahead column misaligned with CJK label: %d vs %d (%q / %q)", s0, s1, rows[0], rows[1])
	}
}

func TestDoctorShowsLabelColumn(t *testing.T) {
	cfgPath, worktreeRoot := setupLabeledWorkspaceProject(t, map[string]string{"a": "店长管理小程序"}, "a", "b")
	if _, err := run(t, "--config", cfgPath, "open", "feat/x"); err != nil {
		t.Fatalf("open error = %v", err)
	}
	if err := os.RemoveAll(filepath.Join(worktreeRoot, "feat/x", "a")); err != nil {
		t.Fatal(err)
	}
	out, err := run(t, "--config", cfgPath, "doctor")
	if err == nil {
		t.Fatalf("doctor with prunable should return error, out = %q", out)
	}
	if strings.Contains(out, "\t") {
		t.Errorf("doctor output must not contain tabs: %q", out)
	}
	if !lineMatching(out, "店长管理小程序", "a", "僵尸 worktree") {
		t.Errorf("doctor should show label 店长管理小程序 next to repo a: %q", out)
	}
}
