package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tttpeng/grove/core/doctor"
)

func newPruneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "prune",
		Short: "清理所有仓库的僵尸 worktree",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rp, m, err := resolveCurrent(cmd)
			if err != nil {
				return err
			}
			results, err := doctor.Prune(rp, m)
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			total := 0
			for _, r := range results {
				if r.Err != nil {
					fmt.Fprintf(w, "failed  %s\t%v\n", r.Repo, r.Err)
					continue
				}
				if len(r.Pruned) == 0 {
					continue
				}
				total += len(r.Pruned)
				fmt.Fprintf(w, "%s\t清理 %d 个\n", r.Repo, len(r.Pruned))
				for _, p := range r.Pruned {
					fmt.Fprintf(w, "  %s\n", p)
				}
			}
			if total == 0 {
				fmt.Fprintln(w, "无僵尸 worktree")
			}
			return nil
		},
	}
}
