package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const DefaultLayout = "{worktreeRoot}/{branch}/{repo}"

type Project struct {
	Manifest     string `yaml:"manifest"`
	CloneRoot    string `yaml:"cloneRoot,omitempty"`
	WorktreeRoot string `yaml:"worktreeRoot,omitempty"`
	Layout       string `yaml:"layout,omitempty"`
}

type Config struct {
	Current  string             `yaml:"current"`
	Projects map[string]Project `yaml:"projects"`
}

func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".grove", "config.yaml"), nil
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if c.Projects == nil {
		c.Projects = map[string]Project{}
	}
	return &c, nil
}

func Save(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir config dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write config %s: %w", path, err)
	}
	return nil
}

type ResolvedProject struct {
	Name         string
	Manifest     string
	CloneRoot    string
	WorktreeRoot string
	Layout       string
}

func expandTilde(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, path[2:]), nil
}

func (c *Config) Resolve(name string) (ResolvedProject, error) {
	p, ok := c.Projects[name]
	if !ok {
		return ResolvedProject{}, fmt.Errorf("config: unknown project %q", name)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ResolvedProject{}, err
	}
	cloneRoot := p.CloneRoot
	if cloneRoot == "" {
		cloneRoot = filepath.Join(home, ".grove", name, "clones")
	}
	worktreeRoot := p.WorktreeRoot
	if worktreeRoot == "" {
		worktreeRoot = filepath.Join(home, ".grove", name, "trees")
	}
	layout := p.Layout
	if layout == "" {
		layout = DefaultLayout
	}
	manifestPath, err := expandTilde(p.Manifest)
	if err != nil {
		return ResolvedProject{}, err
	}
	cloneRoot, err = expandTilde(cloneRoot)
	if err != nil {
		return ResolvedProject{}, err
	}
	worktreeRoot, err = expandTilde(worktreeRoot)
	if err != nil {
		return ResolvedProject{}, err
	}
	return ResolvedProject{
		Name:         name,
		Manifest:     manifestPath,
		CloneRoot:    cloneRoot,
		WorktreeRoot: worktreeRoot,
		Layout:       layout,
	}, nil
}

func (p ResolvedProject) WorktreePath(branch, repo string) string {
	return strings.NewReplacer(
		"{worktreeRoot}", p.WorktreeRoot,
		"{branch}", branch,
		"{repo}", repo,
	).Replace(p.Layout)
}
