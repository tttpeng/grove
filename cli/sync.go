package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tttpeng/grove/core/workspace"
	"github.com/tttpeng/grove/internal/tablefmt"
)

func newSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync [<branch>]",
		Short: "把各仓库 worktree 合并到基线（省略 branch 用 cwd 探测）",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rp, m, err := resolveCurrent(cmd)
			if err != nil {
				return err
			}
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
			if conflicts > 0 {
				return fmt.Errorf("%d 个仓库存在冲突，待手动解决", conflicts)
			}
			return nil
		},
	}
}
