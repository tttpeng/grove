package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tttpeng/grove/core/project"
)

func newBootstrapCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "bootstrap",
		Short: "按当前项目 manifest clone 全部仓库",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rp, m, err := resolveCurrent(cmd)
			if err != nil {
				return err
			}
			results, err := project.Bootstrap(rp, m)
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			for _, r := range results {
				switch {
				case r.Err != nil:
					fmt.Fprintf(w, "failed  %s\t%v\n", r.Repo, r.Err)
				case r.Skipped:
					fmt.Fprintf(w, "skipped %s\t%s\n", r.Repo, r.Path)
				default:
					fmt.Fprintf(w, "cloned  %s\t%s\n", r.Repo, r.Path)
				}
			}
			return nil
		},
	}
}
