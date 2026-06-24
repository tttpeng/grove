package project_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/tttpeng/grove/core/config"
	"github.com/tttpeng/grove/core/manifest"
	"github.com/tttpeng/grove/core/project"
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

func TestScanDetectsRepos(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "repo-a")
	mkRepo(t, a)
	origin := filepath.Join(t.TempDir(), "o")
	mkRepo(t, origin)
	gitCmd(t, a, "remote", "add", "origin", origin)
	os.MkdirAll(filepath.Join(root, "not-a-repo"), 0o755)

	m, err := project.Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if m.Project != filepath.Base(root) {
		t.Errorf("Project = %q", m.Project)
	}
	if len(m.Repos) != 1 {
		t.Fatalf("want 1 repo, got %d: %+v", len(m.Repos), m.Repos)
	}
	if m.Repos[0].Name != "repo-a" {
		t.Errorf("repo name = %q", m.Repos[0].Name)
	}
	if m.Repos[0].Remote != origin {
		t.Errorf("remote = %q want %q", m.Repos[0].Remote, origin)
	}
}

func TestScanIgnoresParentRepoSubdirs(t *testing.T) {
	parent := t.TempDir()
	mkRepo(t, parent)
	docs := filepath.Join(parent, "docs")
	os.MkdirAll(docs, 0o755)
	os.WriteFile(filepath.Join(docs, "guide.md"), []byte("doc"), 0o644)
	gitCmd(t, parent, "add", ".")
	gitCmd(t, parent, "commit", "-m", "docs")

	sub := filepath.Join(parent, "repos", "sub")
	mkRepo(t, sub)
	origin := filepath.Join(t.TempDir(), "o")
	mkRepo(t, origin)
	gitCmd(t, sub, "remote", "add", "origin", origin)

	m, err := project.Scan(parent)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Repos) != 1 {
		t.Fatalf("want 1 repo (sub only), got %d: %+v", len(m.Repos), m.Repos)
	}
	if m.Repos[0].Name != "sub" {
		t.Errorf("repo name = %q, want sub", m.Repos[0].Name)
	}
	for _, r := range m.Repos {
		if r.Name == "docs" {
			t.Error("docs is a plain subdir of parent and must not be detected as a repo")
		}
	}
}

func TestRegister(t *testing.T) {
	cfg := &config.Config{Projects: map[string]config.Project{}}
	err := project.Register(cfg, "erp", "/m/workspace.yaml", config.Project{Manifest: "/m/workspace.yaml"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Projects["erp"].Manifest != "/m/workspace.yaml" {
		t.Error("not registered")
	}
	if cfg.Current != "erp" {
		t.Error("current should default to first project")
	}
	if err := project.Register(cfg, "erp", "/m/workspace.yaml", config.Project{}, false); err == nil {
		t.Error("duplicate without force should error")
	}
}

func TestScanHostScansReposSubdir(t *testing.T) {
	root := t.TempDir()
	mkRepo(t, root)
	hostOrigin := filepath.Join(t.TempDir(), "host-o")
	mkRepo(t, hostOrigin)
	gitCmd(t, root, "remote", "add", "origin", hostOrigin)

	member := filepath.Join(root, "repos", "member-a")
	mkRepo(t, member)
	memberOrigin := filepath.Join(t.TempDir(), "member-o")
	mkRepo(t, memberOrigin)
	gitCmd(t, member, "remote", "add", "origin", memberOrigin)

	bogus := filepath.Join(root, "tool-b")
	mkRepo(t, bogus)

	m, err := project.Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if m.Host == nil {
		t.Fatal("Host should be detected when cwd is a repo")
	}
	if m.Host.Name != filepath.Base(root) {
		t.Errorf("Host.Name = %q, want %q", m.Host.Name, filepath.Base(root))
	}
	if len(m.Repos) != 1 {
		t.Fatalf("want 1 member under repos/, got %d: %+v", len(m.Repos), m.Repos)
	}
	if m.Repos[0].Name != "member-a" {
		t.Errorf("member name = %q, want member-a", m.Repos[0].Name)
	}
	if m.Repos[0].Remote != memberOrigin {
		t.Errorf("member remote = %q, want %q", m.Repos[0].Remote, memberOrigin)
	}
	for _, r := range m.Repos {
		if r.Name == "tool-b" {
			t.Error("repo under host root (not repos/) must not be detected as member")
		}
	}
}

func TestScanHostWithoutReposIsHostOnly(t *testing.T) {
	root := t.TempDir()
	mkRepo(t, root)
	hostOrigin := filepath.Join(t.TempDir(), "host-o")
	mkRepo(t, hostOrigin)
	gitCmd(t, root, "remote", "add", "origin", hostOrigin)

	m, err := project.Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if m.Host == nil {
		t.Fatal("Host should be detected when cwd is a repo")
	}
	if len(m.Repos) != 0 {
		t.Fatalf("want 0 members (host-only draft), got %d: %+v", len(m.Repos), m.Repos)
	}
}

func TestBootstrapHostClonesMembersUnderRepos(t *testing.T) {
	root := t.TempDir()
	memberOrigin := filepath.Join(root, "origin-member")
	mkRepo(t, memberOrigin)
	cloneRoot := filepath.Join(root, "clones")
	if err := os.MkdirAll(cloneRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	rp := config.ResolvedProject{Name: "erp-main", CloneRoot: cloneRoot}
	m := &manifest.Manifest{
		Project:         "erp-main",
		DefaultBaseline: "main",
		Host:            &manifest.Repo{Name: "erp-main", Remote: filepath.Join(root, "origin-host")},
		Repos:           []manifest.Repo{{Name: "member-a", Remote: memberOrigin}},
	}

	res, err := project.Bootstrap(rp, m)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 {
		t.Fatalf("host mode should clone only members, got %d results: %+v", len(res), res)
	}
	if res[0].Repo != "member-a" || res[0].Err != nil {
		t.Fatalf("member bootstrap failed: %+v", res)
	}
	wantPath := filepath.Join(cloneRoot, "repos", "member-a")
	if res[0].Path != wantPath {
		t.Errorf("member path = %q, want %q", res[0].Path, wantPath)
	}
	if _, err := os.Stat(filepath.Join(wantPath, ".git")); err != nil {
		t.Errorf("member not cloned under repos/: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cloneRoot, "member-a")); !os.IsNotExist(err) {
		t.Errorf("member must not be cloned at cloneRoot root, stat err = %v", err)
	}

	res2, _ := project.Bootstrap(rp, m)
	if !res2[0].Skipped {
		t.Error("second bootstrap should skip existing member under repos/")
	}
}

func TestBootstrapClones(t *testing.T) {
	root := t.TempDir()
	origin := filepath.Join(root, "origin-a")
	mkRepo(t, origin)
	cloneRoot := filepath.Join(root, "clones")
	rp := config.ResolvedProject{Name: "p", CloneRoot: cloneRoot}
	m := &manifest.Manifest{Project: "p", DefaultBaseline: "main",
		Repos: []manifest.Repo{{Name: "a", Remote: origin}}}
	res, err := project.Bootstrap(rp, m)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Err != nil {
		t.Fatalf("bootstrap failed: %+v", res)
	}
	if _, err := os.Stat(filepath.Join(cloneRoot, "a", ".git")); err != nil {
		t.Errorf("repo a not cloned: %v", err)
	}
	res2, _ := project.Bootstrap(rp, m)
	if !res2[0].Skipped {
		t.Error("second bootstrap should skip existing")
	}
}
