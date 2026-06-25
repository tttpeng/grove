package workspace

import (
	"github.com/tttpeng/grove/core/config"
	"github.com/tttpeng/grove/core/git"
	"github.com/tttpeng/grove/core/manifest"
)

type rootUnit struct {
	name     string
	cloneDir string
	isHost   bool
}

func rootUnits(rp config.ResolvedProject, m *manifest.Manifest) []rootUnit {
	var units []rootUnit
	if m.Host != nil {
		units = append(units, rootUnit{name: m.Host.Name, cloneDir: rp.CloneRoot, isHost: true})
	}
	for _, repo := range m.Repos {
		units = append(units, rootUnit{name: repo.Name, cloneDir: MemberCloneDir(rp, m, repo.Name), isHost: false})
	}
	return units
}

func RootWorkspace(rp config.ResolvedProject, m *manifest.Manifest) Workspace {
	ws := Workspace{IsRoot: true}
	for _, u := range rootUnits(rp, m) {
		st := RepoStatus{Repo: u.name, Path: u.cloneDir}
		if git.IsRepo(u.cloneDir) {
			st.Exists = true
			if b, err := git.CurrentBranch(u.cloneDir); err == nil {
				st.Branch = b
			}
			var exclude []string
			if u.isHost {
				exclude = HostExcludeDirs(m)
			}
			if dirty, err := git.HasTrackedChanges(u.cloneDir, exclude); err == nil {
				st.Dirty = dirty
			}
			if up, err := git.Upstream(u.cloneDir); err == nil {
				if a, err := git.Ahead(u.cloneDir, up); err == nil {
					st.Ahead = a
				}
				if b, err := git.Behind(u.cloneDir, up); err == nil {
					st.Behind = b
				}
			}
		}
		ws.Repos = append(ws.Repos, st)
	}
	return ws
}

func SyncRoot(rp config.ResolvedProject, m *manifest.Manifest) ([]RepoResult, error) {
	units := rootUnits(rp, m)
	results := make([]RepoResult, 0, len(units))
	for _, u := range units {
		result := RepoResult{Repo: u.name, Path: u.cloneDir}
		if !git.IsRepo(u.cloneDir) {
			result.Action = "skipped"
			result.Note = "未 clone"
			results = append(results, result)
			continue
		}
		branch, err := git.CurrentBranch(u.cloneDir)
		if err != nil || branch == "" || branch == "HEAD" {
			result.Action = "skipped"
			result.Note = "detached HEAD"
			results = append(results, result)
			continue
		}
		upstream, err := git.Upstream(u.cloneDir)
		if err != nil {
			result.Action = "skipped"
			result.Note = "无上游"
			results = append(results, result)
			continue
		}
		var exclude []string
		if u.isHost {
			exclude = HostExcludeDirs(m)
		}
		if dirty, derr := git.HasTrackedChanges(u.cloneDir, exclude); derr == nil && dirty {
			result.Action = "skipped"
			result.Note = "有未提交改动"
			results = append(results, result)
			continue
		}
		if ferr := git.Fetch(u.cloneDir, "origin", branch); ferr != nil {
			result.Action = "fetch-failed"
			result.Note = ferr.Error()
			result.Err = ferr
			results = append(results, result)
			continue
		}
		res, ffErr := git.FastForward(u.cloneDir, upstream)
		switch res {
		case git.FFUpToDate:
			result.Action = "up-to-date"
		case git.FFUpdated:
			result.Action = "updated"
		case git.FFDiverged:
			result.Action = "skipped"
			result.Note = "无法快进（本地已分叉或有冲突）"
			result.Err = ffErr
		}
		results = append(results, result)
	}
	return results, nil
}
