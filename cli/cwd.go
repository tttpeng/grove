package cli

import (
	"github.com/tttpeng/grove/core/git"
	"github.com/tttpeng/grove/core/workspace"
)

func detectBranch(dir, worktreeRoot string) string {
	if !workspace.UnderRoot(dir, worktreeRoot) {
		return ""
	}
	if !git.IsRepo(dir) {
		return ""
	}
	branch, err := git.CurrentBranch(dir)
	if err != nil {
		return ""
	}
	return branch
}
