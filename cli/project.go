package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tttpeng/grove/core/config"
	"github.com/tttpeng/grove/core/git"
	"github.com/tttpeng/grove/core/project"
)

func newProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "管理已注册的项目",
	}
	cmd.AddCommand(newProjectListCmd())
	cmd.AddCommand(newProjectRemoveCmd())
	cmd.AddCommand(newProjectAddCmd())
	return cmd
}

func newProjectAddCmd() *cobra.Command {
	var from, manifestOverride, cloneRoot string
	var force bool
	cmd := &cobra.Command{
		Use:   "add <name> --from <git-url>",
		Short: "克隆索引仓库并注册项目",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if from == "" {
				return fmt.Errorf("必须提供 --from <git-url>")
			}
			path, cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			if _, ok := cfg.Projects[name]; ok && !force {
				return fmt.Errorf("project %q already registered", name)
			}
			root, err := deriveCloneRoot(name, cloneRoot)
			if err != nil {
				return err
			}
			indexDir := filepath.Join(root, repoBasename(from))
			if !git.IsRepo(indexDir) {
				if err := git.Clone(from, indexDir); err != nil {
					return err
				}
			}
			manifestPath := manifestOverride
			if manifestPath == "" {
				manifestPath = filepath.Join(indexDir, "workspace.yaml")
			}
			p := config.Project{CloneRoot: root}
			if err := project.Register(cfg, name, manifestPath, p, force); err != nil {
				return err
			}
			if err := config.Save(path, cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "已注册项目 %s（manifest=%s）\n", name, manifestPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "索引仓库 git 地址")
	cmd.Flags().StringVar(&manifestOverride, "manifest", "", "manifest 路径（默认取克隆内的 workspace.yaml）")
	cmd.Flags().StringVar(&cloneRoot, "clone-root", "", "克隆根目录")
	cmd.Flags().BoolVar(&force, "force", false, "覆盖同名项目")
	return cmd
}

func deriveCloneRoot(name, override string) (string, error) {
	if override != "" {
		return override, nil
	}
	rp, err := (&config.Config{Projects: map[string]config.Project{name: {}}}).Resolve(name)
	if err != nil {
		return "", err
	}
	return rp.CloneRoot, nil
}

func repoBasename(url string) string {
	url = strings.TrimRight(url, "/")
	if i := strings.LastIndexAny(url, "/:"); i >= 0 {
		url = url[i+1:]
	}
	return strings.TrimSuffix(url, ".git")
}

func newProjectListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "列出本机已注册的项目",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			entries := project.List(cfg)
			if len(entries) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "（无已注册项目）")
				return nil
			}
			for _, e := range entries {
				marker := "  "
				if e.Current {
					marker = "* "
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s%s\t%s\n", marker, e.Name, e.Manifest)
			}
			return nil
		},
	}
}

func newProjectRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "注销项目（仅删配置条目，不动磁盘）",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			if err := project.Remove(cfg, args[0]); err != nil {
				return err
			}
			if err := config.Save(path, cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "已注销项目 %s\n", args[0])
			return nil
		},
	}
}
