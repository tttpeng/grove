package manifest_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tttpeng/grove/core/manifest"
)

func TestParseValidManifest(t *testing.T) {
	data := []byte(`project: erp
defaultBaseline: stage
repos:
  - { name: erp-main, remote: git@example.com:erp-main.git, baseline: master }
  - { name: erp-lt-vv, remote: git@example.com:erp-lt-vv.git }
`)
	m, err := manifest.Parse(data)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if m.Project != "erp" {
		t.Errorf("Project = %q, want erp", m.Project)
	}
	if m.DefaultBaseline != "stage" {
		t.Errorf("DefaultBaseline = %q, want stage", m.DefaultBaseline)
	}
	if len(m.Repos) != 2 {
		t.Fatalf("len(Repos) = %d, want 2", len(m.Repos))
	}
	if m.Repos[0].Name != "erp-main" {
		t.Errorf("Repos[0].Name = %q, want erp-main", m.Repos[0].Name)
	}
	if m.Repos[0].Remote != "git@example.com:erp-main.git" {
		t.Errorf("Repos[0].Remote = %q, want git@example.com:erp-main.git", m.Repos[0].Remote)
	}
	if m.Repos[0].Baseline != "master" {
		t.Errorf("Repos[0].Baseline = %q, want master", m.Repos[0].Baseline)
	}
	if m.Repos[1].Baseline != "" {
		t.Errorf("Repos[1].Baseline = %q, want empty", m.Repos[1].Baseline)
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "workspace.yaml")
	data := []byte(`project: erp
defaultBaseline: stage
repos:
  - { name: erp-main, remote: git@example.com:erp-main.git }
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := manifest.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if m.Project != "erp" {
		t.Errorf("Project = %q, want erp", m.Project)
	}
	if len(m.Repos) != 1 {
		t.Fatalf("len(Repos) = %d, want 1", len(m.Repos))
	}
	if _, err := manifest.Load(filepath.Join(dir, "missing.yaml")); err == nil {
		t.Error("Load(missing) expected error")
	}
}

func TestLoadRejectsInvalidManifest(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		"no-baseline.yaml": "project: erp\nrepos:\n  - { name: a, remote: x }\n",
		"no-repos.yaml":    "project: erp\ndefaultBaseline: stage\n",
		"dup-name.yaml":    "project: erp\ndefaultBaseline: stage\nrepos:\n  - { name: a, remote: x }\n  - { name: a, remote: y }\n",
		"no-remote.yaml":   "project: erp\ndefaultBaseline: stage\nrepos:\n  - { name: a }\n",
	}
	for name, body := range cases {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := manifest.Load(path); err == nil {
			t.Errorf("Load(%s) expected validation error", name)
		}
	}
}

func TestBaselineForInheritsDefault(t *testing.T) {
	m := &manifest.Manifest{
		DefaultBaseline: "stage",
		Repos: []manifest.Repo{
			{Name: "erp-main", Remote: "x", Baseline: "master"},
			{Name: "erp-lt-vv", Remote: "x"},
		},
	}
	got, err := m.BaselineFor("erp-lt-vv")
	if err != nil {
		t.Fatalf("BaselineFor error = %v", err)
	}
	if got != "stage" {
		t.Errorf("BaselineFor(erp-lt-vv) = %q, want stage", got)
	}
	got, err = m.BaselineFor("erp-main")
	if err != nil {
		t.Fatalf("BaselineFor error = %v", err)
	}
	if got != "master" {
		t.Errorf("BaselineFor(erp-main) = %q, want master", got)
	}
	if _, err := m.BaselineFor("ghost"); err == nil {
		t.Error("BaselineFor(ghost) expected error")
	}
}

func TestParseManifestWithHost(t *testing.T) {
	data := []byte(`project: erp-main
defaultBaseline: stage
host:
  name: erp-main
  remote: git@example.com:erp-main.git
  baseline: master
repos:
  - { name: erp-lt-vv, remote: git@example.com:erp-lt-vv.git }
`)
	m, err := manifest.Parse(data)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if m.Host == nil {
		t.Fatal("Host should be parsed")
	}
	if m.Host.Name != "erp-main" {
		t.Errorf("Host.Name = %q, want erp-main", m.Host.Name)
	}
	if m.Host.Remote != "git@example.com:erp-main.git" {
		t.Errorf("Host.Remote = %q", m.Host.Remote)
	}
	if m.Host.Baseline != "master" {
		t.Errorf("Host.Baseline = %q, want master", m.Host.Baseline)
	}
}

func TestHostName(t *testing.T) {
	m := &manifest.Manifest{}
	if m.HostName() != "" {
		t.Errorf("HostName() = %q, want empty for nil host", m.HostName())
	}
	m.Host = &manifest.Repo{Name: "erp-main", Remote: "x"}
	if m.HostName() != "erp-main" {
		t.Errorf("HostName() = %q, want erp-main", m.HostName())
	}
}

func TestValidateHost(t *testing.T) {
	tests := []struct {
		name    string
		m       manifest.Manifest
		wantErr bool
	}{
		{
			name: "valid with host",
			m: manifest.Manifest{Project: "p", DefaultBaseline: "stage",
				Host:  &manifest.Repo{Name: "erp-main", Remote: "x", Baseline: "master"},
				Repos: []manifest.Repo{{Name: "a", Remote: "x"}}},
			wantErr: false,
		},
		{
			name: "host empty name",
			m: manifest.Manifest{Project: "p", DefaultBaseline: "stage",
				Host:  &manifest.Repo{Remote: "x"},
				Repos: []manifest.Repo{{Name: "a", Remote: "x"}}},
			wantErr: true,
		},
		{
			name: "host empty remote",
			m: manifest.Manifest{Project: "p", DefaultBaseline: "stage",
				Host:  &manifest.Repo{Name: "erp-main"},
				Repos: []manifest.Repo{{Name: "a", Remote: "x"}}},
			wantErr: true,
		},
		{
			name: "host name collides with repo",
			m: manifest.Manifest{Project: "p", DefaultBaseline: "stage",
				Host:  &manifest.Repo{Name: "a", Remote: "x"},
				Repos: []manifest.Repo{{Name: "a", Remote: "x"}}},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.m.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBaselineForHost(t *testing.T) {
	m := &manifest.Manifest{
		DefaultBaseline: "stage",
		Host:            &manifest.Repo{Name: "erp-main", Remote: "x", Baseline: "master"},
		Repos:           []manifest.Repo{{Name: "a", Remote: "x"}},
	}
	got, err := m.BaselineFor("erp-main")
	if err != nil {
		t.Fatalf("BaselineFor(host) error = %v", err)
	}
	if got != "master" {
		t.Errorf("BaselineFor(erp-main) = %q, want master", got)
	}

	m.Host.Baseline = ""
	got, err = m.BaselineFor("erp-main")
	if err != nil {
		t.Fatalf("BaselineFor(host) error = %v", err)
	}
	if got != "stage" {
		t.Errorf("BaselineFor(erp-main) without baseline = %q, want stage (default)", got)
	}
}

func TestParseManifestWithLabel(t *testing.T) {
	data := []byte(`project: erp-main
defaultBaseline: stage
host:
  name: erp-main
  remote: git@example.com:erp-main.git
  label: 主收银台
repos:
  - { name: erp-lt-vv, remote: git@example.com:erp-lt-vv.git, label: 店长管理小程序 }
  - { name: erp-lt-mini, remote: git@example.com:erp-lt-mini.git }
`)
	m, err := manifest.Parse(data)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if m.Host == nil {
		t.Fatal("Host should be parsed")
	}
	if m.Host.Label != "主收银台" {
		t.Errorf("Host.Label = %q, want 主收银台", m.Host.Label)
	}
	if m.Repos[0].Label != "店长管理小程序" {
		t.Errorf("Repos[0].Label = %q, want 店长管理小程序", m.Repos[0].Label)
	}
	if m.Repos[1].Label != "" {
		t.Errorf("Repos[1].Label = %q, want empty", m.Repos[1].Label)
	}
	if err := m.Validate(); err != nil {
		t.Errorf("Validate() error = %v", err)
	}
}

func TestLabelFor(t *testing.T) {
	m := &manifest.Manifest{
		Project:         "erp-main",
		DefaultBaseline: "stage",
		Host:            &manifest.Repo{Name: "erp-main", Remote: "x", Label: "主收银台"},
		Repos: []manifest.Repo{
			{Name: "erp-lt-vv", Remote: "x", Label: "店长管理小程序"},
			{Name: "erp-lt-mini", Remote: "x"},
		},
	}
	if got := m.LabelFor("erp-lt-vv"); got != "店长管理小程序" {
		t.Errorf("LabelFor(erp-lt-vv) = %q, want 店长管理小程序", got)
	}
	if got := m.LabelFor("erp-lt-mini"); got != "erp-lt-mini" {
		t.Errorf("LabelFor(erp-lt-mini) = %q, want erp-lt-mini (no label fallback)", got)
	}
	if got := m.LabelFor("ghost"); got != "ghost" {
		t.Errorf("LabelFor(ghost) = %q, want ghost (unknown fallback)", got)
	}
	if got := m.LabelFor("erp-main"); got != "主收银台" {
		t.Errorf("LabelFor(erp-main) = %q, want 主收银台 (host label)", got)
	}
}

func TestLabelForNilHostAndEmptyLabel(t *testing.T) {
	m := &manifest.Manifest{
		Project:         "erp",
		DefaultBaseline: "stage",
		Repos: []manifest.Repo{
			{Name: "erp-main", Remote: "x"},
		},
	}
	if got := m.LabelFor("erp-main"); got != "erp-main" {
		t.Errorf("LabelFor(erp-main) = %q, want erp-main", got)
	}

	m.Host = &manifest.Repo{Name: "host-only", Remote: "x"}
	if got := m.LabelFor("host-only"); got != "host-only" {
		t.Errorf("LabelFor(host-only) = %q, want host-only (empty host label fallback)", got)
	}
}

func TestLegacyManifestWithoutLabel(t *testing.T) {
	data := []byte(`project: erp
defaultBaseline: stage
host:
  name: erp-main
  remote: git@example.com:erp-main.git
repos:
  - { name: erp-lt-vv, remote: git@example.com:erp-lt-vv.git }
`)
	m, err := manifest.Parse(data)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if err := m.Validate(); err != nil {
		t.Errorf("Validate() error = %v", err)
	}
	if m.Host.Label != "" {
		t.Errorf("Host.Label = %q, want empty", m.Host.Label)
	}
	if m.Repos[0].Label != "" {
		t.Errorf("Repos[0].Label = %q, want empty", m.Repos[0].Label)
	}
	if got := m.LabelFor("erp-main"); got != "erp-main" {
		t.Errorf("LabelFor(erp-main) = %q, want erp-main", got)
	}
	if got := m.LabelFor("erp-lt-vv"); got != "erp-lt-vv" {
		t.Errorf("LabelFor(erp-lt-vv) = %q, want erp-lt-vv", got)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		m       manifest.Manifest
		wantErr bool
	}{
		{
			name: "valid",
			m: manifest.Manifest{Project: "p", DefaultBaseline: "stage",
				Repos: []manifest.Repo{{Name: "a", Remote: "x"}}},
			wantErr: false,
		},
		{
			name:    "no project",
			m:       manifest.Manifest{DefaultBaseline: "stage", Repos: []manifest.Repo{{Name: "a", Remote: "x"}}},
			wantErr: true,
		},
		{
			name:    "no defaultBaseline",
			m:       manifest.Manifest{Project: "p", Repos: []manifest.Repo{{Name: "a", Remote: "x"}}},
			wantErr: true,
		},
		{
			name:    "no repos",
			m:       manifest.Manifest{Project: "p", DefaultBaseline: "stage"},
			wantErr: true,
		},
		{
			name: "duplicate name",
			m: manifest.Manifest{Project: "p", DefaultBaseline: "stage",
				Repos: []manifest.Repo{{Name: "a", Remote: "x"}, {Name: "a", Remote: "y"}}},
			wantErr: true,
		},
		{
			name: "missing remote",
			m: manifest.Manifest{Project: "p", DefaultBaseline: "stage",
				Repos: []manifest.Repo{{Name: "a"}}},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.m.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
