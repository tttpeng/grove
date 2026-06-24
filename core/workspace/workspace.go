package workspace

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tttpeng/grove/core/config"
	"github.com/tttpeng/grove/core/git"
	"github.com/tttpeng/grove/core/manifest"
)

type RepoResult struct {
	Repo   string
	Path   string
	Action string
	Note   string
	Err    error
}

type RepoStatus struct {
	Repo   string
	Path   string
	Exists bool
	Branch string
	Dirty  bool
	Ahead  int
	Behind int
}

type Workspace struct {
	Branch      string
	Description string
	Repos       []RepoStatus
}

type OpenOptions struct {
	Baseline    string
	NoFetch     bool
	Description string
}

type CloseOptions struct {
	Force        bool
	DeleteBranch bool
}

type createdEntry struct {
	cloneDir  string
	path      string
	resultIdx int
}

type openUnit struct {
	name     string
	cloneDir string
	wtPath   string
}

const MemberDir = "repos"

func MemberCloneDir(rp config.ResolvedProject, m *manifest.Manifest, repo string) string {
	if m.Host != nil {
		return filepath.Join(rp.CloneRoot, MemberDir, repo)
	}
	return filepath.Join(rp.CloneRoot, repo)
}

func MemberWorktreePath(rp config.ResolvedProject, m *manifest.Manifest, branch, repo string) string {
	if m.Host != nil {
		return filepath.Join(rp.WorktreeRoot, branch, MemberDir, repo)
	}
	return rp.WorktreePath(branch, repo)
}

func hostWorktreePath(rp config.ResolvedProject, branch string) string {
	return filepath.Join(rp.WorktreeRoot, branch)
}

func HostExcludeDirs(m *manifest.Manifest) []string {
	return []string{MemberDir}
}

func descRepoDir(rp config.ResolvedProject, m *manifest.Manifest) string {
	if m.Host != nil {
		return rp.CloneRoot
	}
	return MemberCloneDir(rp, m, m.Repos[0].Name)
}

func openUnits(rp config.ResolvedProject, m *manifest.Manifest, branch string) []openUnit {
	var units []openUnit
	if m.Host != nil {
		units = append(units, openUnit{
			name:     m.Host.Name,
			cloneDir: rp.CloneRoot,
			wtPath:   hostWorktreePath(rp, branch),
		})
	}
	for _, repo := range m.Repos {
		units = append(units, openUnit{
			name:     repo.Name,
			cloneDir: MemberCloneDir(rp, m, repo.Name),
			wtPath:   MemberWorktreePath(rp, m, branch, repo.Name),
		})
	}
	return units
}

func Open(rp config.ResolvedProject, m *manifest.Manifest, branch string, opts OpenOptions) ([]RepoResult, error) {
	units := openUnits(rp, m, branch)
	for _, u := range units {
		if !git.IsRepo(u.cloneDir) {
			return nil, fmt.Errorf("仓库 %s 未 clone，请先 grove bootstrap", u.name)
		}
	}

	results := make([]RepoResult, 0, len(units))
	var created []createdEntry

	for _, u := range units {
		result := RepoResult{Repo: u.name, Path: u.wtPath}

		if git.IsRepo(u.wtPath) {
			result.Action = "reused"
			results = append(results, result)
			continue
		}

		baseline := opts.Baseline
		if baseline == "" {
			b, err := m.BaselineFor(u.name)
			if err != nil {
				result.Err = err
				results = append(results, result)
				return rollback(created, results, u.name, err)
			}
			baseline = b
		}

		startPoint := baseline
		if !opts.NoFetch {
			ref, degraded, err := fetchOrLocal(u.cloneDir, baseline)
			if err != nil {
				result.Err = err
				results = append(results, result)
				return rollback(created, results, u.name, err)
			}
			startPoint = ref
			if degraded {
				result.Note = degradedNote(baseline)
			}
		} else if git.RevExists(u.cloneDir, "origin/"+baseline) {
			startPoint = "origin/" + baseline
		}

		if err := git.AddWorktree(u.cloneDir, u.wtPath, branch, startPoint); err != nil {
			result.Err = err
			results = append(results, result)
			return rollback(created, results, u.name, err)
		}

		result.Action = "created"
		results = append(results, result)
		created = append(created, createdEntry{cloneDir: u.cloneDir, path: u.wtPath, resultIdx: len(results) - 1})
	}

	if opts.Description != "" {
		git.SetBranchDescription(descRepoDir(rp, m), branch, opts.Description)
	}

	return results, nil
}

func fetchOrLocal(cloneDir, baseline string) (string, bool, error) {
	if err := git.Fetch(cloneDir, "origin", baseline); err != nil {
		if git.RevExists(cloneDir, "origin/"+baseline) {
			return "origin/" + baseline, true, nil
		}
		return "", false, err
	}
	return "origin/" + baseline, false, nil
}

func degradedNote(baseline string) string {
	return fmt.Sprintf("fetch 失败，基于本地 origin/%s", baseline)
}

func appendNote(existing, add string) string {
	if existing == "" {
		return add
	}
	return existing + "；" + add
}

type SyncOptions struct{}

func Sync(rp config.ResolvedProject, m *manifest.Manifest, branch string) ([]RepoResult, error) {
	units := openUnits(rp, m, branch)
	results := make([]RepoResult, 0, len(units))

	for _, u := range units {
		result := RepoResult{Repo: u.name, Path: u.wtPath}

		if !git.IsRepo(u.wtPath) {
			result.Action = "skipped"
			results = append(results, result)
			continue
		}

		baseline, err := m.BaselineFor(u.name)
		if err != nil {
			result.Action = "sync-failed"
			result.Note = err.Error()
			result.Err = err
			results = append(results, result)
			continue
		}

		startRef, degraded, err := fetchOrLocal(u.cloneDir, baseline)
		if err != nil {
			result.Action = "sync-failed"
			result.Note = err.Error()
			result.Err = err
			results = append(results, result)
			continue
		}
		if degraded {
			result.Note = appendNote(result.Note, degradedNote(baseline))
		}

		stashed := false
		if dirty, derr := git.IsDirty(u.wtPath); derr == nil && dirty {
			stashed, _ = git.Stash(u.wtPath)
		}

		res, merr := git.Merge(u.wtPath, startRef)
		if merr != nil {
			result.Action = "sync-failed"
			result.Note = appendNote(result.Note, merr.Error())
			result.Err = merr
			results = append(results, result)
			continue
		}

		switch res {
		case git.MergeConflict:
			result.Action = "conflict"
			if stashed {
				result.Note = appendNote(result.Note, "已 stash，解决冲突后手动 git stash pop")
			}
		case git.MergeUpToDate:
			result.Action = "up-to-date"
			if stashed {
				if conflict, _ := git.StashPop(u.wtPath); conflict {
					result.Note = appendNote(result.Note, "stash pop 冲突待解决")
				}
			}
		case git.MergeMerged:
			result.Action = "synced"
			if stashed {
				if conflict, _ := git.StashPop(u.wtPath); conflict {
					result.Note = appendNote(result.Note, "stash pop 冲突待解决")
				}
			}
		}

		results = append(results, result)
	}

	return results, nil
}

func rollback(created []createdEntry, results []RepoResult, failedRepo string, cause error) ([]RepoResult, error) {
	for i := len(created) - 1; i >= 0; i-- {
		git.RemoveWorktree(created[i].cloneDir, created[i].path, true)
		results[created[i].resultIdx].Action = "rolled-back"
	}
	return results, fmt.Errorf("仓库 %s 失败：%w；已回滚本次新建的 worktree（可重新 open 续做）", failedRepo, cause)
}

func Close(rp config.ResolvedProject, m *manifest.Manifest, branch string, opts CloseOptions) ([]RepoResult, error) {
	if !opts.Force {
		var blocked []string
		for _, repo := range m.Repos {
			cloneDir := MemberCloneDir(rp, m, repo.Name)
			wtPath := MemberWorktreePath(rp, m, branch, repo.Name)
			if !git.IsRepo(wtPath) {
				continue
			}
			baseline, _ := m.BaselineFor(repo.Name)
			var reasons []string
			if dirty, err := git.IsDirty(wtPath); err == nil && dirty {
				reasons = append(reasons, "工作区有未提交改动")
			}
			if hasUnpushedWork(cloneDir, wtPath, branch, baseline) {
				reasons = append(reasons, "有未推送的 commit")
			}
			if len(reasons) > 0 {
				blocked = append(blocked, fmt.Sprintf("%s（%s）", repo.Name, strings.Join(reasons, "、")))
			}
		}
		if m.Host != nil {
			hostWt := hostWorktreePath(rp, branch)
			if git.IsRepo(hostWt) {
				var reasons []string
				if dirty, err := git.IsDirtyExcluding(hostWt, HostExcludeDirs(m)); err == nil && dirty {
					reasons = append(reasons, "工作区有未提交改动")
				}
				if hasUnpushedWork(rp.CloneRoot, hostWt, branch, baselineOf(m, m.Host.Name)) {
					reasons = append(reasons, "有未推送的 commit")
				}
				if len(reasons) > 0 {
					blocked = append(blocked, fmt.Sprintf("%s（%s）", m.Host.Name, strings.Join(reasons, "、")))
				}
			}
		}
		if len(blocked) > 0 {
			return nil, fmt.Errorf("以下仓库阻止回收（用 --force 强制）：%s", strings.Join(blocked, "；"))
		}
	}

	results := make([]RepoResult, 0, len(m.Repos)+1)
	for _, repo := range m.Repos {
		cloneDir := MemberCloneDir(rp, m, repo.Name)
		wtPath := MemberWorktreePath(rp, m, branch, repo.Name)
		results = append(results, removeOne(cloneDir, repo.Name, wtPath, branch, opts))
	}

	if m.Host != nil {
		results = append(results, removeOne(rp.CloneRoot, m.Host.Name, hostWorktreePath(rp, branch), branch, opts))
	}

	for _, repo := range m.Repos {
		git.PruneWorktrees(MemberCloneDir(rp, m, repo.Name))
	}
	if m.Host != nil {
		git.PruneWorktrees(rp.CloneRoot)
	}

	return results, nil
}

func removeOne(cloneDir, name, wtPath, branch string, opts CloseOptions) RepoResult {
	result := RepoResult{Repo: name, Path: wtPath}
	if !git.IsRepo(wtPath) {
		result.Action = "skipped"
		return result
	}
	if err := git.RemoveWorktree(cloneDir, wtPath, opts.Force); err != nil {
		result.Err = err
		return result
	}
	result.Action = "removed"
	if opts.DeleteBranch {
		if err := git.DeleteBranch(cloneDir, branch, opts.Force); err != nil {
			result.Err = err
		}
	}
	return result
}

func baselineOf(m *manifest.Manifest, name string) string {
	b, _ := m.BaselineFor(name)
	return b
}

func hasUnpushedWork(cloneDir, wtPath, branch, baseline string) bool {
	if git.RevExists(cloneDir, "origin/"+branch) {
		ahead, err := git.Ahead(wtPath, "origin/"+branch)
		if err != nil {
			return false
		}
		return ahead > 0
	}
	if baseline != "" && git.RevExists(cloneDir, "origin/"+baseline) {
		ahead, err := git.Ahead(wtPath, "origin/"+baseline)
		if err != nil {
			return false
		}
		return ahead > 0
	}
	return false
}

func List(rp config.ResolvedProject, m *manifest.Manifest) ([]Workspace, error) {
	root := resolvePath(rp.WorktreeRoot)
	byBranch := map[string][]RepoStatus{}

	for _, repo := range m.Repos {
		cloneDir := MemberCloneDir(rp, m, repo.Name)
		wts, err := git.ListWorktrees(cloneDir)
		if err != nil {
			return nil, err
		}
		for _, w := range wts {
			if w.Branch == "" {
				continue
			}
			if !underRoot(resolvePath(w.Path), root) {
				continue
			}
			byBranch[w.Branch] = append(byBranch[w.Branch], RepoStatus{
				Repo:   repo.Name,
				Path:   w.Path,
				Exists: true,
				Branch: w.Branch,
			})
		}
	}

	if m.Host != nil {
		wts, err := git.ListWorktrees(rp.CloneRoot)
		if err == nil {
			for _, w := range wts {
				if w.Branch == "" {
					continue
				}
				hostWt := hostWorktreePath(rp, w.Branch)
				if resolvePath(w.Path) != resolvePath(hostWt) {
					continue
				}
				byBranch[w.Branch] = append(byBranch[w.Branch], RepoStatus{
					Repo:   m.Host.Name,
					Path:   w.Path,
					Exists: true,
					Branch: w.Branch,
				})
			}
		}
	}

	branches := make([]string, 0, len(byBranch))
	for b := range byBranch {
		branches = append(branches, b)
	}
	sort.Strings(branches)

	descDir := descRepoDir(rp, m)
	workspaces := make([]Workspace, 0, len(branches))
	for _, b := range branches {
		repos := byBranch[b]
		sort.Slice(repos, func(i, j int) bool { return repos[i].Repo < repos[j].Repo })
		desc, _ := git.BranchDescription(descDir, b)
		workspaces = append(workspaces, Workspace{Branch: b, Description: desc, Repos: repos})
	}
	return workspaces, nil
}

func SetDescription(rp config.ResolvedProject, m *manifest.Manifest, branch, desc string) error {
	return git.SetBranchDescription(descRepoDir(rp, m), branch, desc)
}

func Status(rp config.ResolvedProject, m *manifest.Manifest, branch string) (*Workspace, error) {
	ws := &Workspace{Branch: branch}

	if m.Host != nil {
		hostWt := hostWorktreePath(rp, branch)
		st := RepoStatus{Repo: m.Host.Name, Path: hostWt}
		if git.IsRepo(hostWt) {
			st.Exists = true
			if b, err := git.CurrentBranch(hostWt); err == nil {
				st.Branch = b
			}
			if dirty, err := git.IsDirtyExcluding(hostWt, HostExcludeDirs(m)); err == nil {
				st.Dirty = dirty
			}
			st.Ahead, st.Behind = aheadBehind(rp.CloneRoot, hostWt, m.Host.Name, m)
		}
		ws.Repos = append(ws.Repos, st)
	}

	for _, repo := range m.Repos {
		cloneDir := MemberCloneDir(rp, m, repo.Name)
		wtPath := MemberWorktreePath(rp, m, branch, repo.Name)
		st := RepoStatus{Repo: repo.Name, Path: wtPath}
		if git.IsRepo(wtPath) {
			st.Exists = true
			if b, err := git.CurrentBranch(wtPath); err == nil {
				st.Branch = b
			}
			if dirty, err := git.IsDirty(wtPath); err == nil {
				st.Dirty = dirty
			}
			st.Ahead, st.Behind = aheadBehind(cloneDir, wtPath, repo.Name, m)
		}
		ws.Repos = append(ws.Repos, st)
	}
	return ws, nil
}

func aheadBehind(cloneDir, wtPath, repoName string, m *manifest.Manifest) (int, int) {
	baseline, err := m.BaselineFor(repoName)
	if err != nil {
		return 0, 0
	}
	ref := "origin/" + baseline
	if !git.RevExists(cloneDir, ref) {
		return 0, 0
	}
	ahead := 0
	if a, err := git.Ahead(wtPath, ref); err == nil {
		ahead = a
	}
	behind := 0
	if b, err := git.Behind(wtPath, ref); err == nil {
		behind = b
	}
	return ahead, behind
}

func resolvePath(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

func underRoot(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func UnderRoot(path, root string) bool {
	return underRoot(resolvePath(path), resolvePath(root))
}
