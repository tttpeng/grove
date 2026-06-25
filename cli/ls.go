package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/tttpeng/grove/core/config"
	"github.com/tttpeng/grove/core/manifest"
	"github.com/tttpeng/grove/core/workspace"
	"github.com/tttpeng/grove/internal/tablefmt"
)

func newLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "列出所有 workspace",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rp, m, err := resolveCurrent(cmd)
			if err != nil {
				return err
			}
			return printWorkspaceList(cmd, rp, m)
		},
	}
}

func printWorkspaceList(cmd *cobra.Command, rp config.ResolvedProject, m *manifest.Manifest) error {
	workspaces, err := workspace.List(rp, m)
	if err != nil {
		return err
	}
	w := cmd.OutOrStdout()
	hasRegular := false
	rows := [][]string{{"分支", "仓库数", "描述"}}
	for _, ws := range workspaces {
		if !ws.IsRoot {
			hasRegular = true
		}
		desc := ws.Description
		if desc == "" {
			desc = "—"
		}
		rows = append(rows, []string{ws.DisplayName(), strconv.Itoa(len(ws.Repos)), desc})
	}
	for _, line := range tablefmt.Columns(rows, []tablefmt.Align{tablefmt.Left, tablefmt.Right, tablefmt.Left}) {
		fmt.Fprintln(w, line)
	}
	if !hasRegular {
		fmt.Fprintln(w, "（暂无 feature workspace，用 grove open 创建）")
	}
	return nil
}
