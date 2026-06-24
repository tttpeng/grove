package config_test

import (
	"path/filepath"
	"testing"

	"github.com/tttpeng/grove/core/config"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "config.yaml")
	cfg := &config.Config{
		Current: "erp",
		Projects: map[string]config.Project{
			"erp": {Manifest: "~/x/workspace.yaml"},
		},
	}
	if err := config.Save(path, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Current != "erp" {
		t.Errorf("Current = %q, want erp", got.Current)
	}
	if got.Projects["erp"].Manifest != "~/x/workspace.yaml" {
		t.Errorf("Manifest = %q", got.Projects["erp"].Manifest)
	}
}

func TestLoadEnsuresProjectsMap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := config.Save(path, &config.Config{}); err != nil {
		t.Fatal(err)
	}
	got, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Projects == nil {
		t.Error("Projects should be non-nil after Load")
	}
}

func TestDefaultPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	p, err := config.DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".grove", "config.yaml")
	if p != want {
		t.Errorf("DefaultPath = %q, want %q", p, want)
	}
}

func TestResolveFillsDefaults(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg := &config.Config{
		Projects: map[string]config.Project{
			"erp": {Manifest: "~/erp/workspace.yaml"},
		},
	}
	rp, err := cfg.Resolve("erp")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if rp.Manifest != filepath.Join(home, "erp", "workspace.yaml") {
		t.Errorf("Manifest = %q", rp.Manifest)
	}
	if rp.CloneRoot != filepath.Join(home, ".grove", "erp", "clones") {
		t.Errorf("CloneRoot = %q", rp.CloneRoot)
	}
	if rp.WorktreeRoot != filepath.Join(home, ".grove", "erp", "trees") {
		t.Errorf("WorktreeRoot = %q", rp.WorktreeRoot)
	}
	if rp.Layout != config.DefaultLayout {
		t.Errorf("Layout = %q, want default", rp.Layout)
	}
}

func TestResolveUsesExplicitPaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg := &config.Config{
		Projects: map[string]config.Project{
			"erp": {
				Manifest:     "~/erp/workspace.yaml",
				CloneRoot:    "~/custom/clones",
				WorktreeRoot: "/abs/trees",
				Layout:       "{worktreeRoot}/{repo}@{branch}",
			},
		},
	}
	rp, err := cfg.Resolve("erp")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if rp.CloneRoot != filepath.Join(home, "custom", "clones") {
		t.Errorf("CloneRoot = %q, want %q", rp.CloneRoot, filepath.Join(home, "custom", "clones"))
	}
	if rp.WorktreeRoot != "/abs/trees" {
		t.Errorf("WorktreeRoot = %q, want /abs/trees", rp.WorktreeRoot)
	}
	if rp.Layout != "{worktreeRoot}/{repo}@{branch}" {
		t.Errorf("Layout = %q, want custom layout", rp.Layout)
	}
}

func TestResolveUnknownProject(t *testing.T) {
	cfg := &config.Config{Projects: map[string]config.Project{}}
	if _, err := cfg.Resolve("ghost"); err == nil {
		t.Error("Resolve(ghost) expected error")
	}
}

func TestWorktreePath(t *testing.T) {
	rp := config.ResolvedProject{WorktreeRoot: "/wt", Layout: config.DefaultLayout}
	got := rp.WorktreePath("feat/x", "erp-main")
	if got != "/wt/feat/x/erp-main" {
		t.Errorf("WorktreePath = %q, want /wt/feat/x/erp-main", got)
	}
}

func TestWorktreePathCustomLayout(t *testing.T) {
	rp := config.ResolvedProject{WorktreeRoot: "/wt", Layout: "{worktreeRoot}/{repo}@{branch}"}
	got := rp.WorktreePath("feat/x", "erp-main")
	if got != "/wt/erp-main@feat/x" {
		t.Errorf("WorktreePath = %q, want /wt/erp-main@feat/x", got)
	}
}
