package project

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tttpeng/grove/core/config"
	"github.com/tttpeng/grove/core/git"
	"github.com/tttpeng/grove/core/manifest"
	"github.com/tttpeng/grove/core/workspace"
)

type BootstrapResult struct {
	Repo    string
	Path    string
	Skipped bool
	Err     error
}

func Scan(root string) (*manifest.Manifest, error) {
	m := &manifest.Manifest{
		Project:         filepath.Base(root),
		DefaultBaseline: "stage",
	}

	memberRoot := root
	if git.IsRepoRoot(root) {
		host := manifest.Repo{Name: filepath.Base(root)}
		if remote, err := git.RemoteURL(root, "origin"); err == nil {
			host.Remote = remote
		}
		if baseline, err := git.DefaultBranch(root); err == nil && baseline != m.DefaultBaseline {
			host.Baseline = baseline
		}
		m.Host = &host
		memberRoot = filepath.Join(root, workspace.MemberDir)
	}

	entries, err := os.ReadDir(memberRoot)
	if err != nil {
		if m.Host != nil && os.IsNotExist(err) {
			return m, nil
		}
		return nil, fmt.Errorf("scan %s: %w", memberRoot, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		sub := filepath.Join(memberRoot, entry.Name())
		if !git.IsRepoRoot(sub) {
			continue
		}
		repo := manifest.Repo{Name: entry.Name()}
		if remote, err := git.RemoteURL(sub, "origin"); err == nil {
			repo.Remote = remote
		}
		if baseline, err := git.DefaultBranch(sub); err == nil && baseline != m.DefaultBaseline {
			repo.Baseline = baseline
		}
		m.Repos = append(m.Repos, repo)
	}
	return m, nil
}

func Register(cfg *config.Config, name, manifestPath string, p config.Project, force bool) error {
	if _, ok := cfg.Projects[name]; ok && !force {
		return fmt.Errorf("project %q already registered", name)
	}
	if cfg.Projects == nil {
		cfg.Projects = map[string]config.Project{}
	}
	p.Manifest = manifestPath
	cfg.Projects[name] = p
	if cfg.Current == "" {
		cfg.Current = name
	}
	return nil
}

func Bootstrap(rp config.ResolvedProject, m *manifest.Manifest) ([]BootstrapResult, error) {
	results := make([]BootstrapResult, 0, len(m.Repos))
	for _, repo := range m.Repos {
		path := workspace.MemberCloneDir(rp, m, repo.Name)
		result := BootstrapResult{Repo: repo.Name, Path: path}
		if git.IsRepo(path) {
			result.Skipped = true
		} else {
			result.Err = git.Clone(repo.Remote, path)
		}
		results = append(results, result)
	}
	return results, nil
}
