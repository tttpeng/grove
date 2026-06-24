package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tttpeng/grove/core/workspace"
)

func newDescribeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "describe <branch> <text>",
		Short: "设置 workspace 描述",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			rp, m, err := resolveCurrent(cmd)
			if err != nil {
				return err
			}
			branch, desc := args[0], args[1]
			if err := workspace.SetDescription(rp, m, branch, desc); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "已更新 %s 描述：%s\n", branch, desc)
			return nil
		},
	}
}
