package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/tttpeng/grove/core/config"
	"github.com/tttpeng/grove/core/doctor"
	"github.com/tttpeng/grove/core/manifest"
	"github.com/tttpeng/grove/core/workspace"
)

type viewState int

const (
	viewList viewState = iota
	viewDetail
	viewDoctor
	viewOpen
	viewOpenDesc
	viewConfirmClose
)

type Model struct {
	rp            config.ResolvedProject
	m             *manifest.Manifest
	state         viewState
	workspaces    []workspace.Workspace
	cursor        int
	detail        *workspace.Workspace
	findings      []doctor.Finding
	input         textinput.Model
	pendingBranch string
	pendingDesc   string
	status        string
	busy          bool
	quitting      bool
	width         int
	height        int
	initialBranch string
}

func New(rp config.ResolvedProject, m *manifest.Manifest) Model {
	return NewWithBranch(rp, m, "")
}

func NewWithBranch(rp config.ResolvedProject, m *manifest.Manifest, initialBranch string) Model {
	ti := textinput.New()
	ti.Placeholder = "feat/xxx"
	ti.CharLimit = 200
	state := viewList
	if initialBranch != "" {
		state = viewDetail
	}
	return Model{
		rp:            rp,
		m:             m,
		state:         state,
		input:         ti,
		busy:          true,
		initialBranch: initialBranch,
	}
}

func (m Model) Init() tea.Cmd {
	if m.initialBranch != "" {
		return tea.Batch(listCmd(m.rp, m.m), detailCmd(m.rp, m.m, m.initialBranch), textinput.Blink)
	}
	return tea.Batch(listCmd(m.rp, m.m), textinput.Blink)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case listMsg:
		m.busy = false
		if msg.err != nil {
			m.status = "错误: " + msg.err.Error()
			return m, nil
		}
		m.workspaces = msg.ws
		if m.cursor >= len(m.workspaces) {
			m.cursor = len(m.workspaces) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
		return m, nil
	case detailMsg:
		m.busy = false
		if msg.err != nil {
			m.status = "错误: " + msg.err.Error()
			m.state = viewList
			return m, nil
		}
		m.detail = msg.ws
		return m, nil
	case doctorMsg:
		m.busy = false
		if msg.err != nil {
			m.status = "错误: " + msg.err.Error()
			m.state = viewList
			return m, nil
		}
		m.findings = msg.fs
		m.status = fmt.Sprintf("doctor：%d 个问题", len(msg.fs))
		return m, nil
	case openMsg:
		m.busy = false
		if msg.err != nil {
			m.status = "错误: " + msg.err.Error()
			return m, nil
		}
		m.status = "open：" + summarizeRepoResults(msg.res)
		m.busy = true
		return m, listCmd(m.rp, m.m)
	case closeMsg:
		m.busy = false
		if msg.err != nil {
			m.status = "错误: " + msg.err.Error()
			return m, nil
		}
		m.status = "close：" + summarizeRepoResults(msg.res)
		m.busy = true
		return m, listCmd(m.rp, m.m)
	case pruneMsg:
		m.busy = false
		if msg.err != nil {
			m.status = "错误: " + msg.err.Error()
			return m, nil
		}
		m.status = "prune：" + summarizePruneResults(msg.res)
		m.busy = true
		return m, listCmd(m.rp, m.m)
	case syncMsg:
		m.busy = false
		if msg.err != nil {
			m.status = "错误: " + msg.err.Error()
			return m, nil
		}
		m.status = "sync: " + summarizeSyncResults(msg.res)
		m.state = viewDetail
		if m.detail != nil {
			m.busy = true
			if m.detail.IsRoot {
				return m, detailRootCmd(m.rp, m.m)
			}
			return m, detailCmd(m.rp, m.m, m.detail.Branch)
		}
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	if m.state == viewOpen || m.state == viewOpenDesc {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.state == viewOpen {
		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "enter":
			branch := strings.TrimSpace(m.input.Value())
			if branch == "" {
				return m, nil
			}
			m.pendingBranch = branch
			m.state = viewOpenDesc
			m.input.Reset()
			m.input.Placeholder = "可空，例如：修复登录"
			m.input.Focus()
			return m, textinput.Blink
		case "esc":
			m.state = viewList
			return m, nil
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	if m.state == viewOpenDesc {
		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "enter":
			m.pendingDesc = strings.TrimSpace(m.input.Value())
			branch := m.pendingBranch
			m.state = viewList
			m.busy = true
			return m, openCmd(m.rp, m.m, branch, m.pendingDesc)
		case "esc":
			m.state = viewList
			return m, nil
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	if msg.String() == "ctrl+c" {
		m.quitting = true
		return m, tea.Quit
	}

	switch m.state {
	case viewList:
		if msg.String() == "q" {
			m.quitting = true
			return m, tea.Quit
		}
		return m.handleListKey(msg)
	case viewDetail:
		switch msg.String() {
		case "esc", "q":
			m.state = viewList
			return m, nil
		case "d":
			if m.detail == nil {
				return m, nil
			}
			m.busy = true
			m.state = viewDoctor
			m.findings = nil
			return m, doctorCmd(m.rp, m.m, m.detail.Branch)
		case "s":
			if m.detail == nil {
				return m, nil
			}
			m.busy = true
			if m.detail.IsRoot {
				return m, syncRootCmd(m.rp, m.m)
			}
			return m, syncCmd(m.rp, m.m, m.detail.Branch)
		}
	case viewDoctor:
		switch msg.String() {
		case "esc", "q":
			m.state = viewList
			return m, nil
		}
	case viewConfirmClose:
		switch msg.String() {
		case "y":
			m.state = viewList
			ws := m.selected()
			if ws == nil {
				return m, nil
			}
			m.busy = true
			return m, closeCmd(m.rp, m.m, ws.Branch, workspace.CloseOptions{})
		case "n", "esc":
			m.state = viewList
			return m, nil
		}
	}
	return m, nil
}

func (m Model) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.workspaces)-1 {
			m.cursor++
		}
	case "enter":
		ws := m.selected()
		if ws == nil {
			return m, nil
		}
		m.busy = true
		m.state = viewDetail
		m.detail = nil
		if ws.IsRoot {
			return m, detailRootCmd(m.rp, m.m)
		}
		return m, detailCmd(m.rp, m.m, ws.Branch)
	case "o":
		m.state = viewOpen
		m.pendingBranch = ""
		m.pendingDesc = ""
		m.input.Reset()
		m.input.Placeholder = "feat/xxx"
		m.input.Focus()
		return m, textinput.Blink
	case "c":
		if ws := m.selected(); ws != nil && !ws.IsRoot {
			m.state = viewConfirmClose
		}
	case "d":
		m.busy = true
		m.state = viewDoctor
		m.findings = nil
		return m, doctorCmd(m.rp, m.m, "")
	case "p":
		m.busy = true
		return m, pruneCmd(m.rp, m.m)
	case "r":
		m.busy = true
		return m, listCmd(m.rp, m.m)
	}
	return m, nil
}

func (m Model) selected() *workspace.Workspace {
	if m.cursor < 0 || m.cursor >= len(m.workspaces) {
		return nil
	}
	return &m.workspaces[m.cursor]
}

func summarizeRepoResults(res []workspace.RepoResult) string {
	counts := map[string]int{}
	for _, r := range res {
		if r.Err != nil {
			counts["failed"]++
			continue
		}
		counts[r.Action]++
	}
	return joinCounts(counts)
}

func summarizeSyncResults(res []workspace.RepoResult) string {
	counts := map[string]int{}
	for _, r := range res {
		if r.Err != nil {
			if r.Action == "fetch-failed" {
				counts["fetch-failed"]++
			} else {
				counts["sync-failed"]++
			}
			continue
		}
		counts[r.Action]++
	}
	if len(counts) == 0 {
		return "无改动"
	}
	order := []string{"synced", "updated", "up-to-date", "conflict", "skipped", "sync-failed", "fetch-failed"}
	var parts []string
	for _, k := range order {
		if n, ok := counts[k]; ok {
			parts = append(parts, fmt.Sprintf("%d %s", n, k))
		}
	}
	return strings.Join(parts, ", ")
}

func summarizePruneResults(res []doctor.PruneResult) string {
	pruned := 0
	failed := 0
	for _, r := range res {
		if r.Err != nil {
			failed++
		}
		pruned += len(r.Pruned)
	}
	parts := []string{fmt.Sprintf("%d pruned", pruned)}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", failed))
	}
	return strings.Join(parts, "，")
}

func joinCounts(counts map[string]int) string {
	if len(counts) == 0 {
		return "无改动"
	}
	order := []string{"created", "reused", "removed", "skipped", "rolled-back", "failed"}
	var parts []string
	for _, k := range order {
		if n, ok := counts[k]; ok {
			parts = append(parts, fmt.Sprintf("%d %s", n, k))
		}
	}
	return strings.Join(parts, "，")
}

func Run(rp config.ResolvedProject, m *manifest.Manifest, initialBranch string) error {
	p := tea.NewProgram(NewWithBranch(rp, m, initialBranch), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
