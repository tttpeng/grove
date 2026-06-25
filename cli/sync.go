package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tttpeng/grove/core/config"
	"github.com/tttpeng/grove/core/manifest"
	"github.com/tttpeng/grove/core/workspace"
	"github.com/tttpeng/grove/internal/tablefmt"
)

func NewSyncCmd() *cobra.Command {
	return newSyncCmd()
}

func newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync [<branch>]",
		Short: "把各仓库 worktree 合并到基线（--root 改为快进各 clone 主工作树）",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rp, m, err := resolveCurrent(cmd)
			if err != nil {
				return err
			}
			rootMode, _ := cmd.Flags().GetBool("root")
			if rootMode {
				if len(args) > 0 {
					return fmt.Errorf("--root 不接受 branch 参数")
				}
				return runSyncRoot(cmd, rp, m)
			}
			return runSyncBranch(cmd, rp, m, args)
		},
	}
	cmd.Flags().Bool("root", false, "同步 root：把 host 与各成员 clone 的主工作树快进到上游")
	return cmd
}

func runSyncBranch(cmd *cobra.Command, rp config.ResolvedProject, m *manifest.Manifest, args []string) error {
	branch := ""
	if len(args) == 1 {
		branch = args[0]
	} else if cwd, werr := os.Getwd(); werr == nil {
		branch = detectBranch(cwd, rp.WorktreeRoot)
	}
	if branch == "" {
		return fmt.Errorf("未指定 branch，且当前目录不在任何 worktree 内")
	}
	results, syncErr := workspace.Sync(rp, m, branch)
	if syncErr != nil {
		return syncErr
	}
	conflicts := printSyncTable(cmd, m, results)
	if conflicts > 0 {
		return fmt.Errorf("%d 个仓库存在冲突，待手动解决", conflicts)
	}
	return nil
}

func runSyncRoot(cmd *cobra.Command, rp config.ResolvedProject, m *manifest.Manifest) error {
	results, err := workspace.SyncRoot(rp, m)
	if err != nil {
		return err
	}
	failed := 0
	for _, r := range results {
		if r.Action == "fetch-failed" {
			failed++
		}
	}
	printSyncTable(cmd, m, results)
	if failed > 0 {
		return fmt.Errorf("%d 个仓库 fetch 失败", failed)
	}
	return nil
}

func printSyncTable(cmd *cobra.Command, m *manifest.Manifest, results []workspace.RepoResult) int {
	w := cmd.OutOrStdout()
	rows := [][]string{{"名称", "仓库", "结果", "说明"}}
	conflicts := 0
	for _, r := range results {
		note := r.Note
		if note == "" {
			note = "—"
		}
		rows = append(rows, []string{m.LabelFor(r.Repo), r.Repo, r.Action, note})
		if r.Action == "conflict" {
			conflicts++
		}
	}
	for _, line := range tablefmt.Columns(rows, []tablefmt.Align{tablefmt.Left, tablefmt.Left, tablefmt.Left, tablefmt.Left}) {
		fmt.Fprintln(w, "  "+line)
	}
	return conflicts
}
