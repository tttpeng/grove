package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tttpeng/grove/core/workspace"
	"github.com/tttpeng/grove/internal/tablefmt"
)

func newOpenCmd() *cobra.Command {
	var baseline string
	var noFetch bool
	var description string
	cmd := &cobra.Command{
		Use:   "open <branch>",
		Short: "在所有仓库创建同名 worktree",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rp, m, err := resolveCurrent(cmd)
			if err != nil {
				return err
			}
			branch := args[0]
			results, openErr := workspace.Open(rp, m, branch, workspace.OpenOptions{
				Baseline:    baseline,
				NoFetch:     noFetch,
				Description: description,
			})
			w := cmd.OutOrStdout()
			rows := make([][]string, 0, len(results))
			for _, r := range results {
				switch {
				case r.Err != nil:
					rows = append(rows, []string{"failed", r.Repo, fmt.Sprintf("%v", r.Err)})
				case r.Action == "rolled-back":
					rows = append(rows, []string{"回滚", r.Repo, r.Path})
				case r.Action == "reused":
					rows = append(rows, []string{"reused", r.Repo, r.Path})
				default:
					rows = append(rows, []string{"created", r.Repo, r.Path})
				}
			}
			for _, line := range tablefmt.Columns(rows, []tablefmt.Align{tablefmt.Left, tablefmt.Left, tablefmt.Left}) {
				fmt.Fprintln(w, line)
			}
			return openErr
		},
	}
	cmd.Flags().StringVar(&baseline, "baseline", "", "覆盖各仓库基线（默认取 manifest）")
	cmd.Flags().BoolVar(&noFetch, "no-fetch", false, "创建前不 fetch")
	cmd.Flags().StringVarP(&description, "description", "m", "", "workspace 描述")
	return cmd
}
