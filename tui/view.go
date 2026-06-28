package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/tttpeng/grove/core/doctor"
	"github.com/tttpeng/grove/internal/tablefmt"
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	cursorStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	headerStyle = lipgloss.NewStyle().Faint(true)
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	helpStyle   = lipgloss.NewStyle().Faint(true)
	dirtyStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	cleanStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
)

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("grove · " + m.rp.Name))
	b.WriteString("\n\n")

	switch m.state {
	case viewList:
		b.WriteString(m.viewList())
	case viewDetail:
		b.WriteString(m.viewDetail())
	case viewDoctor:
		b.WriteString(m.viewDoctor())
	case viewOpen:
		b.WriteString(m.viewOpen())
	case viewOpenDesc:
		b.WriteString(m.viewOpenDesc())
	case viewConfirmClose:
		b.WriteString(m.viewConfirmClose())
	}

	b.WriteString("\n\n")
	b.WriteString(m.footer())
	return b.String()
}

func (m Model) viewList() string {
	if len(m.workspaces) == 0 {
		if m.busy {
			return "加载中…"
		}
		return "暂无工作空间。按 o 新建。"
	}

	rows := [][]string{{"分支", "仓库数", "描述"}}
	for _, ws := range m.workspaces {
		desc := ws.Description
		if desc == "" {
			desc = "—"
		}
		rows = append(rows, []string{ws.DisplayName(), strconv.Itoa(len(ws.Repos)), desc})
	}
	lines := tablefmt.Columns(rows, []tablefmt.Align{tablefmt.Left, tablefmt.Right, tablefmt.Left})

	var b strings.Builder
	b.WriteString("  ")
	b.WriteString(headerStyle.Render(lines[0]))
	b.WriteString("\n")
	for i, line := range lines[1:] {
		if i == m.cursor {
			b.WriteString("▸ ")
			b.WriteString(cursorStyle.Render(line))
		} else {
			b.WriteString("  ")
			b.WriteString(line)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func (m Model) viewDetail() string {
	if m.busy || m.detail == nil {
		return "加载中…"
	}
	rows := [][]string{{"名称", "仓库", "分支", "状态", "ahead", "behind"}}
	for _, r := range m.detail.Repos {
		if !r.Exists {
			rows = append(rows, []string{m.m.LabelFor(r.Repo), r.Repo, "-", "缺失", "-", "-"})
			continue
		}
		state := cleanStyle.Render("clean")
		if r.Dirty {
			state = dirtyStyle.Render("dirty")
		}
		rows = append(rows, []string{m.m.LabelFor(r.Repo), r.Repo, r.Branch, state, fmt.Sprintf("ahead %d", r.Ahead), fmt.Sprintf("behind %d", r.Behind)})
	}
	lines := tablefmt.Columns(rows, []tablefmt.Align{tablefmt.Left, tablefmt.Left, tablefmt.Left, tablefmt.Left, tablefmt.Left, tablefmt.Left})

	var b strings.Builder
	b.WriteString(titleStyle.Render(m.detail.DisplayName()))
	b.WriteString("\n\n")
	b.WriteString(headerStyle.Render(lines[0]))
	b.WriteString("\n")
	for _, line := range lines[1:] {
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

func (m Model) viewDoctor() string {
	if m.busy {
		return "检查中…"
	}
	if len(m.findings) == 0 {
		return cleanStyle.Render("✓ 无问题")
	}
	byKind := map[string][]doctor.Finding{}
	for _, f := range m.findings {
		byKind[f.Kind] = append(byKind[f.Kind], f)
	}
	kinds := make([]string, 0, len(byKind))
	for k := range byKind {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)

	var b strings.Builder
	for _, k := range kinds {
		b.WriteString(titleStyle.Render(k))
		b.WriteString("\n")
		rows := make([][]string, 0, len(byKind[k]))
		for _, f := range byKind[k] {
			branch := f.Branch
			if branch == "" {
				branch = "—"
			}
			rows = append(rows, []string{m.m.LabelFor(f.Repo), f.Repo, branch, f.Detail})
		}
		for _, line := range tablefmt.Columns(rows, []tablefmt.Align{tablefmt.Left, tablefmt.Left, tablefmt.Left, tablefmt.Left}) {
			b.WriteString("  ")
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (m Model) viewOpen() string {
	s := "新工作空间分支名:\n\n" + m.input.View()
	if m.openErr != "" {
		s += "\n\n" + dirtyStyle.Render("失败: "+m.openErr)
	}
	return s
}

func (m Model) viewOpenDesc() string {
	return fmt.Sprintf("分支 %s 的描述（可空）:\n\n%s", titleStyle.Render(m.pendingBranch), m.input.View())
}

func (m Model) viewConfirmClose() string {
	return fmt.Sprintf("回收 %s？(y/n)", titleStyle.Render(m.pendingClose))
}

func (m Model) footer() string {
	var bottom string
	if m.busy {
		bottom = statusStyle.Render("处理中…")
	} else if m.status != "" {
		bottom = statusStyle.Render(m.status)
	}

	var help string
	switch m.state {
	case viewList:
		help = "↑/↓ 移动 · enter 详情 · o 新建 · c 回收 · d 体检 · p 清理 · r 刷新 · q 退出"
	case viewDetail:
		help = "c 回收 · d 体检 · s 同步 · r 刷新 · esc/q 返回"
	case viewDoctor:
		help = "esc/q 返回"
	case viewOpen:
		help = "enter 下一步 · esc 取消"
	case viewOpenDesc:
		help = "enter 确认 · esc 取消"
	case viewConfirmClose:
		help = "y 确认 · n/esc 取消"
	}

	if bottom != "" {
		return bottom + "\n" + helpStyle.Render(help)
	}
	return helpStyle.Render(help)
}
