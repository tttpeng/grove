package doctor

import (
	"fmt"
	"path/filepath"

	"github.com/tttpeng/grove/core/config"
	"github.com/tttpeng/grove/core/git"
	"github.com/tttpeng/grove/core/manifest"
	"github.com/tttpeng/grove/core/workspace"
)

type Finding struct {
	Repo   string
	Branch string
	Path   string
	Kind   string
	Detail string
}

type PruneResult struct {
	Repo   string
	Pruned []string
	Err    error
}

func Check(rp config.ResolvedProject, m *manifest.Manifest, branch string) ([]Finding, error) {
	branches, err := targetBranches(rp, m, branch)
	if err != nil {
		return nil, err
	}

	var findings []Finding
	for _, b := range branches {
		if m.Host != nil {
			findings = append(findings, checkHost(rp, m, b)...)
		}
		for _, repo := range m.Repos {
			cloneDir := workspace.MemberCloneDir(rp, m, repo.Name)
			wtPath := workspace.MemberWorktreePath(rp, m, b, repo.Name)
			if !git.IsRepo(wtPath) {
				continue
			}
			if actual, err := git.CurrentBranch(wtPath); err == nil && actual != b {
				findings = append(findings, Finding{
					Repo:   repo.Name,
					Branch: b,
					Path:   wtPath,
					Kind:   "drift",
					Detail: fmt.Sprintf("期望分支 %s，实际 %s", b, actual),
				})
			}
			if dirty, err := git.IsDirty(wtPath); err == nil && dirty {
				findings = append(findings, Finding{
					Repo:   repo.Name,
					Branch: b,
					Path:   wtPath,
					Kind:   "dirty",
					Detail: "工作区有未提交改动",
				})
			}
			baseline, err := m.BaselineFor(repo.Name)
			if err == nil && git.RevExists(cloneDir, "origin/"+baseline) {
				if n, err := git.Behind(wtPath, "origin/"+baseline); err == nil && n > 0 {
					findings = append(findings, Finding{
						Repo:   repo.Name,
						Branch: b,
						Path:   wtPath,
						Kind:   "behind",
						Detail: fmt.Sprintf("落后 origin/%s %d 个 commit", baseline, n),
					})
				}
			}
		}
	}

	for _, repo := range m.Repos {
		cloneDir := workspace.MemberCloneDir(rp, m, repo.Name)
		if !git.IsRepo(cloneDir) {
			continue
		}
		wts, err := git.ListWorktrees(cloneDir)
		if err != nil {
			continue
		}
		var expected string
		if branch != "" {
			expected = resolvePath(workspace.MemberWorktreePath(rp, m, branch, repo.Name))
		}
		for _, w := range wts {
			if !w.Prunable {
				continue
			}
			if branch != "" && resolvePath(w.Path) != expected {
				continue
			}
			findings = append(findings, Finding{
				Repo:   repo.Name,
				Path:   w.Path,
				Kind:   "prunable",
				Detail: "僵尸 worktree（目录已不存在）",
			})
		}
	}

	return findings, nil
}

func checkHost(rp config.ResolvedProject, m *manifest.Manifest, branch string) []Finding {
	hostWt := filepath.Join(rp.WorktreeRoot, branch)
	if !git.IsRepo(hostWt) {
		return nil
	}
	name := m.Host.Name
	var findings []Finding
	if actual, err := git.CurrentBranch(hostWt); err == nil && actual != branch {
		findings = append(findings, Finding{
			Repo:   name,
			Branch: branch,
			Path:   hostWt,
			Kind:   "drift",
			Detail: fmt.Sprintf("期望分支 %s，实际 %s", branch, actual),
		})
	}
	if dirty, err := git.IsDirtyExcluding(hostWt, workspace.HostExcludeDirs(m)); err == nil && dirty {
		findings = append(findings, Finding{
			Repo:   name,
			Branch: branch,
			Path:   hostWt,
			Kind:   "dirty",
			Detail: "工作区有未提交改动",
		})
	}
	baseline, err := m.BaselineFor(name)
	if err == nil && git.RevExists(rp.CloneRoot, "origin/"+baseline) {
		if n, err := git.Behind(hostWt, "origin/"+baseline); err == nil && n > 0 {
			findings = append(findings, Finding{
				Repo:   name,
				Branch: branch,
				Path:   hostWt,
				Kind:   "behind",
				Detail: fmt.Sprintf("落后 origin/%s %d 个 commit", baseline, n),
			})
		}
	}
	return findings
}

func resolvePath(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	dir, base := filepath.Split(filepath.Clean(p))
	dir = filepath.Clean(dir)
	if dir == p || base == "" {
		return p
	}
	return filepath.Join(resolvePath(dir), base)
}

func Prune(rp config.ResolvedProject, m *manifest.Manifest) ([]PruneResult, error) {
	results := make([]PruneResult, 0, len(m.Repos))
	for _, repo := range m.Repos {
		cloneDir := workspace.MemberCloneDir(rp, m, repo.Name)
		result := PruneResult{Repo: repo.Name}
		if !git.IsRepo(cloneDir) {
			results = append(results, result)
			continue
		}
		wts, err := git.ListWorktrees(cloneDir)
		if err != nil {
			result.Err = err
			results = append(results, result)
			continue
		}
		var prunable []string
		for _, w := range wts {
			if w.Prunable {
				prunable = append(prunable, w.Path)
			}
		}
		if err := git.PruneWorktrees(cloneDir); err != nil {
			result.Err = err
			results = append(results, result)
			continue
		}
		result.Pruned = prunable
		results = append(results, result)
	}
	return results, nil
}

func targetBranches(rp config.ResolvedProject, m *manifest.Manifest, branch string) ([]string, error) {
	if branch != "" {
		return []string{branch}, nil
	}
	wss, err := workspace.List(rp, m)
	if err != nil {
		return nil, err
	}
	branches := make([]string, 0, len(wss))
	for _, ws := range wss {
		branches = append(branches, ws.Branch)
	}
	return branches, nil
}
