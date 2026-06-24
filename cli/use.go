package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tttpeng/grove/core/config"
	"github.com/tttpeng/grove/core/project"
)

func newUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use <project>",
		Short: "切换当前项目",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			if err := project.Use(cfg, args[0]); err != nil {
				return err
			}
			if err := config.Save(path, cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "当前项目切换为 %s\n", args[0])
			return nil
		},
	}
}
