package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tttpeng/grove/core/config"
	"github.com/tttpeng/grove/core/manifest"
	"github.com/tttpeng/grove/tui"
)

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "grove",
		Short:         "跨仓库工作空间管理工具",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			rp, m, err := resolveCurrent(cmd)
			if err != nil {
				return err
			}
			initialBranch := ""
			if cwd, werr := os.Getwd(); werr == nil {
				initialBranch = detectBranch(cwd, rp.WorktreeRoot)
			}
			return tui.Run(rp, m, initialBranch)
		},
	}
	root.PersistentFlags().String("config", "", "个人配置路径（默认 ~/.grove/config.yaml）")
	root.AddCommand(newVersionCmd())
	root.AddCommand(newProjectCmd())
	root.AddCommand(newUseCmd())
	root.AddCommand(newInitCmd())
	root.AddCommand(newBootstrapCmd())
	root.AddCommand(newOpenCmd())
	root.AddCommand(newDescribeCmd())
	root.AddCommand(newCloseCmd())
	root.AddCommand(newLsCmd())
	root.AddCommand(newStatusCmd())
	root.AddCommand(newSyncCmd())
	root.AddCommand(newDoctorCmd())
	root.AddCommand(newPruneCmd())
	return root
}

func Execute() error {
	return NewRootCmd().Execute()
}

func loadConfig(cmd *cobra.Command) (string, *config.Config, error) {
	path, _ := cmd.Flags().GetString("config")
	if path == "" {
		var err error
		path, err = config.DefaultPath()
		if err != nil {
			return "", nil, err
		}
	}
	cfg, err := config.Load(path)
	if err != nil {
		return "", nil, err
	}
	return path, cfg, nil
}

func resolveCurrent(cmd *cobra.Command) (config.ResolvedProject, *manifest.Manifest, error) {
	_, cfg, err := loadConfig(cmd)
	if err != nil {
		return config.ResolvedProject{}, nil, err
	}
	if cfg.Current == "" {
		return config.ResolvedProject{}, nil, fmt.Errorf("未设置当前项目，用 grove use <project>")
	}
	rp, err := cfg.Resolve(cfg.Current)
	if err != nil {
		return config.ResolvedProject{}, nil, err
	}
	m, err := manifest.Load(rp.Manifest)
	if err != nil {
		return config.ResolvedProject{}, nil, err
	}
	return rp, m, nil
}
