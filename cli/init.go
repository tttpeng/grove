package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/tttpeng/grove/core/project"
	"gopkg.in/yaml.v3"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "扫描当前目录生成 workspace.yaml 草稿",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := os.Getwd()
			if err != nil {
				return err
			}
			out := filepath.Join(root, "workspace.yaml")
			if _, err := os.Stat(out); err == nil {
				return fmt.Errorf("workspace.yaml 已存在，拒绝覆盖：%s", out)
			}
			m, err := project.Scan(root)
			if err != nil {
				return err
			}
			data, err := yaml.Marshal(m)
			if err != nil {
				return err
			}
			if err := os.WriteFile(out, data, 0o644); err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			if m.Host != nil {
				fmt.Fprintf(w, "已生成 %s，探测到 host %s + %d 个仓库\n", out, m.Host.Name, len(m.Repos))
				if m.Host.Remote == "" {
					fmt.Fprintf(w, "  需人工补全：host %s（remote=%q baseline=%q）\n", m.Host.Name, m.Host.Remote, m.Host.Baseline)
				}
			} else {
				fmt.Fprintf(w, "已生成 %s，探测到 %d 个仓库\n", out, len(m.Repos))
			}
			for _, repo := range m.Repos {
				if repo.Remote == "" || repo.Baseline == "" {
					fmt.Fprintf(w, "  需人工补全：%s（remote=%q baseline=%q）\n", repo.Name, repo.Remote, repo.Baseline)
				}
			}
			return nil
		},
	}
}
