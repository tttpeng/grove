package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tttpeng/grove/core/doctor"
	"github.com/tttpeng/grove/internal/tablefmt"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor [<branch>]",
		Short: "体检 workspace 各仓库（漂移/未提交/落后/僵尸 worktree）",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rp, m, err := resolveCurrent(cmd)
			if err != nil {
				return err
			}
			branch := ""
			if len(args) == 1 {
				branch = args[0]
			}
			findings, err := doctor.Check(rp, m, branch)
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			if len(findings) == 0 {
				fmt.Fprintln(w, "✓ 无问题")
				return nil
			}
			byKind := map[string][]doctor.Finding{}
			order := []string{}
			for _, f := range findings {
				if _, ok := byKind[f.Kind]; !ok {
					order = append(order, f.Kind)
				}
				byKind[f.Kind] = append(byKind[f.Kind], f)
			}
			for _, kind := range order {
				fmt.Fprintf(w, "%s:\n", kind)
				rows := make([][]string, 0, len(byKind[kind]))
				for _, f := range byKind[kind] {
					branch := f.Branch
					if branch == "" {
						branch = "—"
					}
					rows = append(rows, []string{m.LabelFor(f.Repo), f.Repo, branch, f.Detail})
				}
				for _, line := range tablefmt.Columns(rows, []tablefmt.Align{tablefmt.Left, tablefmt.Left, tablefmt.Left, tablefmt.Left}) {
					fmt.Fprintln(w, "  "+line)
				}
			}
			return fmt.Errorf("发现 %d 个问题", len(findings))
		},
	}
}
