package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type Worktree struct {
	Path     string
	Branch   string
	Head     string
	Prunable bool
}

func run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
	}
	return strings.TrimSpace(out.String()), nil
}

func IsRepo(dir string) bool {
	out, err := run(dir, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		return false
	}
	return out == "true"
}

func IsRepoRoot(dir string) bool {
	out, err := run(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return false
	}
	top, err1 := filepath.EvalSymlinks(out)
	if err1 != nil {
		top = out
	}
	want, err2 := filepath.EvalSymlinks(dir)
	if err2 != nil {
		want = dir
	}
	return top == want
}

func Clone(remote, dir string) error {
	_, err := run("", "clone", remote, dir)
	return err
}

func RemoteURL(dir, remote string) (string, error) {
	return run(dir, "remote", "get-url", remote)
}

func DefaultBranch(dir string) (string, error) {
	out, err := run(dir, "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
	if err == nil {
		return strings.TrimPrefix(out, "origin/"), nil
	}
	return run(dir, "rev-parse", "--abbrev-ref", "HEAD")
}

func Fetch(dir, remote, ref string) error {
	_, err := run(dir, "fetch", remote, ref)
	return err
}

func CurrentBranch(dir string) (string, error) {
	return run(dir, "rev-parse", "--abbrev-ref", "HEAD")
}

func LocalBranchExists(dir, branch string) bool {
	_, err := run(dir, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}

func RevExists(dir, rev string) bool {
	_, err := run(dir, "rev-parse", "--verify", "--quiet", rev)
	return err == nil
}

func IsDirty(dir string) (bool, error) {
	out, err := run(dir, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return out != "", nil
}

func statusDirtyExcluding(dir string, excludeNames []string, skipUntracked bool) (bool, error) {
	out, err := run(dir, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	if out == "" {
		return false, nil
	}
	exclude := map[string]bool{}
	for _, n := range excludeNames {
		exclude[n] = true
	}
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		if skipUntracked && strings.HasPrefix(line, "??") {
			continue
		}
		path := line
		if len(line) > 3 {
			path = line[3:]
		}
		if idx := strings.Index(path, " -> "); idx >= 0 {
			path = path[idx+4:]
		}
		path = strings.Trim(path, "\"")
		first := path
		if idx := strings.IndexByte(path, '/'); idx >= 0 {
			first = path[:idx]
		}
		if exclude[first] {
			continue
		}
		return true, nil
	}
	return false, nil
}

func IsDirtyExcluding(dir string, excludeNames []string) (bool, error) {
	return statusDirtyExcluding(dir, excludeNames, false)
}

func Ahead(dir, upstream string) (int, error) {
	out, err := run(dir, "rev-list", "--count", upstream+"..HEAD")
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(out)
}

func Behind(dir, ref string) (int, error) {
	out, err := run(dir, "rev-list", "--count", "HEAD.."+ref)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(out)
}

func AddWorktree(repoDir, path, branch, startPoint string) error {
	if LocalBranchExists(repoDir, branch) {
		_, err := run(repoDir, "worktree", "add", path, branch)
		return err
	}
	_, err := run(repoDir, "worktree", "add", "-b", branch, path, startPoint)
	return err
}

func ListWorktrees(repoDir string) ([]Worktree, error) {
	out, err := run(repoDir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	var worktrees []Worktree
	var cur *Worktree
	flush := func() {
		if cur != nil {
			worktrees = append(worktrees, *cur)
			cur = nil
		}
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			flush()
			continue
		}
		switch {
		case strings.HasPrefix(line, "worktree "):
			flush()
			cur = &Worktree{Path: strings.TrimPrefix(line, "worktree ")}
		case cur == nil:
			continue
		case strings.HasPrefix(line, "HEAD "):
			cur.Head = strings.TrimPrefix(line, "HEAD ")
		case strings.HasPrefix(line, "branch refs/heads/"):
			cur.Branch = strings.TrimPrefix(line, "branch refs/heads/")
		case line == "detached":
			cur.Branch = ""
		case line == "prunable" || strings.HasPrefix(line, "prunable "):
			cur.Prunable = true
		}
	}
	flush()
	return worktrees, nil
}

func RemoveWorktree(repoDir, path string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)
	_, err := run(repoDir, args...)
	return err
}

func PruneWorktrees(repoDir string) error {
	_, err := run(repoDir, "worktree", "prune")
	return err
}

func DeleteBranch(repoDir, branch string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	_, err := run(repoDir, "branch", flag, branch)
	return err
}

func runExit(dir string, args ...string) (string, int, error) {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", ee.ExitCode(), fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
		}
		return "", -1, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
	}
	return strings.TrimSpace(out.String()), 0, nil
}

type MergeResult int

const (
	MergeUpToDate MergeResult = iota
	MergeMerged
	MergeConflict
)

func hasUnmerged(dir string) (bool, error) {
	out, _, err := runExit(dir, "ls-files", "-u", "--")
	if err != nil {
		return false, err
	}
	return out != "", nil
}

func Merge(dir, ref string) (MergeResult, error) {
	out, code, err := runExit(dir, "merge", "--no-edit", ref)
	if code == 0 {
		if strings.Contains(strings.ToLower(out), "up to date") {
			return MergeUpToDate, nil
		}
		return MergeMerged, nil
	}
	unmerged, uerr := hasUnmerged(dir)
	if uerr != nil {
		return 0, uerr
	}
	if unmerged {
		return MergeConflict, nil
	}
	return 0, err
}

func Stash(dir string) (bool, error) {
	out, code, err := runExit(dir, "stash", "push", "-u")
	if code != 0 {
		return false, err
	}
	if strings.Contains(out, "No local changes to save") {
		return false, nil
	}
	return true, nil
}

func StashPop(dir string) (bool, error) {
	_, code, err := runExit(dir, "stash", "pop")
	unmerged, uerr := hasUnmerged(dir)
	if uerr != nil {
		return false, uerr
	}
	if unmerged {
		return true, nil
	}
	if code != 0 {
		return false, err
	}
	return false, nil
}

func BranchDescription(dir, branch string) (string, error) {
	out, code, err := runExit(dir, "config", "branch."+branch+".description")
	if err != nil {
		if code == 1 {
			return "", nil
		}
		return "", err
	}
	return out, nil
}

func SetBranchDescription(dir, branch, desc string) error {
	key := "branch." + branch + ".description"
	if desc != "" {
		_, err := run(dir, "config", key, desc)
		return err
	}
	_, code, err := runExit(dir, "config", "--unset", key)
	if err != nil && code != 5 {
		return err
	}
	return nil
}

func Upstream(dir string) (string, error) {
	return run(dir, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
}

func HasTrackedChanges(dir string, excludeNames []string) (bool, error) {
	return statusDirtyExcluding(dir, excludeNames, true)
}

type FFResult int

const (
	FFUpToDate FFResult = iota
	FFUpdated
	FFDiverged
)

func FastForward(dir, ref string) (FFResult, error) {
	out, code, err := runExit(dir, "merge", "--ff-only", ref)
	if code == 0 {
		if strings.Contains(strings.ToLower(out), "up to date") {
			return FFUpToDate, nil
		}
		return FFUpdated, nil
	}
	if code > 0 {
		return FFDiverged, nil
	}
	return FFDiverged, err
}
