package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tttpeng/grove/core/workspace"
)

func newCloseCmd() *cobra.Command {
	var force bool
	var deleteBranch bool
	cmd := &cobra.Command{
		Use:   "close <branch>",
		Short: "回收所有仓库的同名 worktree",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rp, m, err := resolveCurrent(cmd)
			if err != nil {
				return err
			}
			branch := args[0]
			results, closeErr := workspace.Close(rp, m, branch, workspace.CloseOptions{
				Force:        force,
				DeleteBranch: deleteBranch,
			})
			w := cmd.OutOrStdout()
			if closeErr != nil {
				fmt.Fprintf(w, "%v\n", closeErr)
				return closeErr
			}
			for _, r := range results {
				switch {
				case r.Err != nil:
					fmt.Fprintf(w, "failed  %s\t%v\n", r.Repo, r.Err)
				case r.Action == "skipped":
					fmt.Fprintf(w, "skipped %s\t（无 worktree）\n", r.Repo)
				default:
					fmt.Fprintf(w, "removed %s\t%s\n", r.Repo, r.Path)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "强制回收脏树/未推送的 worktree")
	cmd.Flags().BoolVar(&deleteBranch, "delete-branch", false, "回收后删除本地分支")
	return cmd
}
