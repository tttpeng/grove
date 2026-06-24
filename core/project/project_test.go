package project_test

import (
	"testing"

	"github.com/tttpeng/grove/core/config"
	"github.com/tttpeng/grove/core/project"
)

func newCfg() *config.Config {
	return &config.Config{
		Current: "erp",
		Projects: map[string]config.Project{
			"erp":  {Manifest: "/a"},
			"nova": {Manifest: "/b"},
		},
	}
}

func TestListSortedWithCurrent(t *testing.T) {
	entries := project.List(newCfg())
	if len(entries) != 2 {
		t.Fatalf("len = %d, want 2", len(entries))
	}
	if entries[0].Name != "erp" || entries[1].Name != "nova" {
		t.Errorf("order = %q,%q, want erp,nova", entries[0].Name, entries[1].Name)
	}
	if !entries[0].Current {
		t.Error("erp should be marked current")
	}
	if entries[1].Current {
		t.Error("nova should not be current")
	}
}

func TestUse(t *testing.T) {
	cfg := newCfg()
	if err := project.Use(cfg, "nova"); err != nil {
		t.Fatalf("Use() error = %v", err)
	}
	if cfg.Current != "nova" {
		t.Errorf("Current = %q, want nova", cfg.Current)
	}
	if err := project.Use(cfg, "ghost"); err == nil {
		t.Error("Use(ghost) expected error")
	}
}

func TestRemoveClearsCurrent(t *testing.T) {
	cfg := newCfg()
	if err := project.Remove(cfg, "erp"); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if _, ok := cfg.Projects["erp"]; ok {
		t.Error("erp should be removed")
	}
	if cfg.Current != "" {
		t.Errorf("Current = %q, want empty", cfg.Current)
	}
	if err := project.Remove(cfg, "ghost"); err == nil {
		t.Error("Remove(ghost) expected error")
	}
}

func TestRemoveKeepsOtherCurrent(t *testing.T) {
	cfg := newCfg()
	if err := project.Remove(cfg, "nova"); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if cfg.Current != "erp" {
		t.Errorf("Current = %q, want erp", cfg.Current)
	}
}
