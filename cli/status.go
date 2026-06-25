package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/tttpeng/grove/core/manifest"
	"github.com/tttpeng/grove/core/workspace"
	"github.com/tttpeng/grove/internal/tablefmt"
)

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status [<branch>]",
		Short: "查看某 workspace 各仓库状态（省略 branch 等同 ls；--root 看 root）",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rp, m, err := resolveCurrent(cmd)
			if err != nil {
				return err
			}
			rootMode, _ := cmd.Flags().GetBool("root")
			if rootMode {
				if len(args) > 0 {
					return fmt.Errorf("--root 不接受 branch 参数")
				}
				ws := workspace.RootWorkspace(rp, m)
				return printStatusWorkspace(cmd, m, ws)
			}
			if len(args) == 0 {
				return printWorkspaceList(cmd, rp, m)
			}
			ws, err := workspace.Status(rp, m, args[0])
			if err != nil {
				return err
			}
			return printStatusWorkspace(cmd, m, *ws)
		},
	}
	cmd.Flags().Bool("root", false, "查看 root（各 clone 主工作树相对上游的状态）")
	return cmd
}

func printStatusWorkspace(cmd *cobra.Command, m *manifest.Manifest, ws workspace.Workspace) error {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "%s\n", ws.DisplayName())
	rows := [][]string{{"名称", "仓库", "分支", "状态", "ahead", "behind"}}
	for _, r := range ws.Repos {
		if !r.Exists {
			rows = append(rows, []string{m.LabelFor(r.Repo), r.Repo, "-", "缺失", "-", "-"})
			continue
		}
		dirty := "clean"
		if r.Dirty {
			dirty = "dirty"
		}
		rows = append(rows, []string{m.LabelFor(r.Repo), r.Repo, r.Branch, dirty, "ahead " + strconv.Itoa(r.Ahead), "behind " + strconv.Itoa(r.Behind)})
	}
	for _, line := range tablefmt.Columns(rows, []tablefmt.Align{tablefmt.Left, tablefmt.Left, tablefmt.Left, tablefmt.Left, tablefmt.Left, tablefmt.Left}) {
		fmt.Fprintln(w, "  "+line)
	}
	return nil
}
