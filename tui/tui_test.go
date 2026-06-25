package tui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/tttpeng/grove/core/config"
	"github.com/tttpeng/grove/core/doctor"
	"github.com/tttpeng/grove/core/manifest"
	"github.com/tttpeng/grove/core/workspace"
)

func testProject() config.ResolvedProject {
	return config.ResolvedProject{
		Name:         "demo",
		Manifest:     "/tmp/grove.yaml",
		CloneRoot:    "/tmp/clone",
		WorktreeRoot: "/tmp/wt",
		Layout:       "nested",
	}
}

func testManifest() *manifest.Manifest {
	return &manifest.Manifest{
		Project:         "demo",
		DefaultBaseline: "main",
		Repos: []manifest.Repo{
			{Name: "api", Remote: "git@example.com:demo/api.git"},
			{Name: "web", Remote: "git@example.com:demo/web.git"},
		},
	}
}

func twoWorkspaces() []workspace.Workspace {
	return []workspace.Workspace{
		{
			Branch:      "feat/alpha",
			Description: "修复登录",
			Repos: []workspace.RepoStatus{
				{Repo: "api", Exists: true, Branch: "feat/alpha", Dirty: false, Ahead: 1},
				{Repo: "web", Exists: true, Branch: "feat/alpha", Dirty: true, Ahead: 0},
			},
		},
		{
			Branch: "feat/beta",
			Repos: []workspace.RepoStatus{
				{Repo: "api", Exists: true, Branch: "feat/beta", Dirty: false, Ahead: 0},
			},
		},
	}
}

func newModel() Model {
	return New(testProject(), testManifest())
}

func update(m Model, msg tea.Msg) (Model, tea.Cmd) {
	next, cmd := m.Update(msg)
	return next.(Model), cmd
}

func key(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func isQuit(t *testing.T, cmd tea.Cmd) bool {
	t.Helper()
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func TestModelInit(t *testing.T) {
	m := newModel()
	if cmd := m.Init(); cmd == nil {
		t.Fatal("Init() 应返回非 nil cmd")
	}
}

func TestModelDirAwareInitLoadsList(t *testing.T) {
	m := NewWithBranch(testProject(), testManifest(), "feat/alpha")
	if m.state != viewDetail {
		t.Fatalf("目录感知启动初始 state 应为 viewDetail，实际 %d", m.state)
	}
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init() 应返回非 nil cmd")
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok {
		t.Fatalf("Init() 应返回 BatchMsg，实际 %T", cmd())
	}
	if len(batch) < 3 {
		t.Fatalf("目录感知启动应同时加载列表与详情（list+detail+blink≥3），实际 %d 个 cmd", len(batch))
	}
}

func TestModelDirAwareCanReturnToList(t *testing.T) {
	m := NewWithBranch(testProject(), testManifest(), "feat/alpha")
	ws := twoWorkspaces()
	m, _ = update(m, detailMsg{ws: &ws[0]})
	m, _ = update(m, listMsg{ws: ws})
	if m.state != viewDetail {
		t.Fatalf("listMsg 不应把 state 弹回列表，仍应 viewDetail，实际 %d", m.state)
	}
	if len(m.workspaces) != 2 {
		t.Fatalf("目录感知启动也应加载列表，workspaces 期望 2，实际 %d", len(m.workspaces))
	}
	m, _ = update(m, key("q"))
	if m.state != viewList {
		t.Fatalf("详情按 q 应返回 viewList，实际 %d", m.state)
	}
	if len(m.workspaces) != 2 {
		t.Fatalf("返回列表后应仍有 2 个 workspaces，实际 %d", len(m.workspaces))
	}
}

func TestModelListMsgPopulates(t *testing.T) {
	m := newModel()
	m, _ = update(m, listMsg{ws: twoWorkspaces()})
	if len(m.workspaces) != 2 {
		t.Fatalf("workspaces 应为 2，实际 %d", len(m.workspaces))
	}
	if m.busy {
		t.Fatal("listMsg 后 busy 应为 false")
	}
}

func TestModelListMsgError(t *testing.T) {
	m := newModel()
	m, _ = update(m, listMsg{err: errStub("boom")})
	if m.busy {
		t.Fatal("错误后 busy 应为 false")
	}
	if m.status == "" {
		t.Fatal("错误后 status 应非空")
	}
}

func TestModelListCursorDown(t *testing.T) {
	m := newModel()
	m, _ = update(m, listMsg{ws: twoWorkspaces()})
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyDown})
	if m.cursor != 1 {
		t.Fatalf("down 后 cursor 应为 1，实际 %d", m.cursor)
	}
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyDown})
	if m.cursor != 1 {
		t.Fatalf("到边界后 cursor 不应越界，应为 1，实际 %d", m.cursor)
	}
}

func TestModelListCursorUp(t *testing.T) {
	m := newModel()
	m, _ = update(m, listMsg{ws: twoWorkspaces()})
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyDown})
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyUp})
	if m.cursor != 0 {
		t.Fatalf("up 后 cursor 应为 0，实际 %d", m.cursor)
	}
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyUp})
	if m.cursor != 0 {
		t.Fatalf("到上边界后 cursor 不应越界，应为 0，实际 %d", m.cursor)
	}
}

func TestModelEnterToDetail(t *testing.T) {
	m := newModel()
	m, _ = update(m, listMsg{ws: twoWorkspaces()})
	m, cmd := update(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.state != viewDetail {
		t.Fatalf("enter 后 state 应为 viewDetail，实际 %d", m.state)
	}
	if !m.busy {
		t.Fatal("enter 进 detail 时 busy 应为 true")
	}
	if cmd == nil {
		t.Fatal("enter 应返回 detailCmd 非 nil")
	}
}

func TestModelOpenView(t *testing.T) {
	m := newModel()
	m, _ = update(m, listMsg{ws: twoWorkspaces()})
	m, cmd := update(m, key("o"))
	if m.state != viewOpen {
		t.Fatalf("o 后 state 应为 viewOpen，实际 %d", m.state)
	}
	if !m.input.Focused() {
		t.Fatal("viewOpen 时 input 应 focused")
	}
	if cmd == nil {
		t.Fatal("o 应返回 textinput.Blink 非 nil")
	}
}

func TestModelOpenBranchAdvancesToDesc(t *testing.T) {
	m := newModel()
	m, _ = update(m, listMsg{ws: twoWorkspaces()})
	m, _ = update(m, key("o"))
	m.input.SetValue("feat/x")
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.state != viewOpenDesc {
		t.Fatalf("分支非空 enter 后 state 应为 viewOpenDesc，实际 %d", m.state)
	}
	if m.busy {
		t.Fatal("进描述步不应置 busy")
	}
}

func TestModelOpenBranchEmptyStays(t *testing.T) {
	m := newModel()
	m, _ = update(m, listMsg{ws: twoWorkspaces()})
	m, _ = update(m, key("o"))
	m, cmd := update(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.state != viewOpen {
		t.Fatalf("空分支 enter 不应前进，state 应为 viewOpen，实际 %d", m.state)
	}
	if m.busy {
		t.Fatal("空分支不应置 busy")
	}
	if cmd != nil {
		t.Fatal("空分支不应返回 cmd")
	}
}

func TestModelOpenDescSubmit(t *testing.T) {
	m := newModel()
	m, _ = update(m, listMsg{ws: twoWorkspaces()})
	m, _ = update(m, key("o"))
	m.input.SetValue("feat/x")
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEnter})
	m.input.SetValue("修复支付")
	m, cmd := update(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.state != viewList {
		t.Fatalf("描述提交后 state 应为 viewList，实际 %d", m.state)
	}
	if !m.busy {
		t.Fatal("描述提交后 busy 应为 true")
	}
	if cmd == nil {
		t.Fatal("描述提交后应返回 openCmd 非 nil")
	}
	if m.pendingDesc != "修复支付" {
		t.Fatalf("提交的描述应为 修复支付，实际 %q", m.pendingDesc)
	}
}

func TestModelOpenDescEmptySubmits(t *testing.T) {
	m := newModel()
	m, _ = update(m, listMsg{ws: twoWorkspaces()})
	m, _ = update(m, key("o"))
	m.input.SetValue("feat/x")
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEnter})
	m, cmd := update(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.state != viewList {
		t.Fatalf("空描述提交后 state 应为 viewList，实际 %d", m.state)
	}
	if !m.busy {
		t.Fatal("空描述提交后 busy 应为 true")
	}
	if cmd == nil {
		t.Fatal("空描述提交后应返回 openCmd 非 nil")
	}
	if m.pendingDesc != "" {
		t.Fatalf("空描述应为空，实际 %q", m.pendingDesc)
	}
}

func TestModelOpenEsc(t *testing.T) {
	m := newModel()
	m, _ = update(m, listMsg{ws: twoWorkspaces()})
	m, _ = update(m, key("o"))
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.state != viewList {
		t.Fatalf("esc 后 state 应为 viewList，实际 %d", m.state)
	}
}

func TestModelOpenDescEsc(t *testing.T) {
	m := newModel()
	m, _ = update(m, listMsg{ws: twoWorkspaces()})
	m, _ = update(m, key("o"))
	m.input.SetValue("feat/x")
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEnter})
	m, cmd := update(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.state != viewList {
		t.Fatalf("描述步 esc 后 state 应为 viewList，实际 %d", m.state)
	}
	if m.busy {
		t.Fatal("描述步 esc 取消不应置 busy")
	}
	if cmd != nil {
		t.Fatal("描述步 esc 取消不应返回 cmd")
	}
}

func TestModelCtrlCQuitsFromOpenDesc(t *testing.T) {
	m := newModel()
	m, _ = update(m, listMsg{ws: twoWorkspaces()})
	m, _ = update(m, key("o"))
	m.input.SetValue("feat/x")
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEnter})
	m, cmd := update(m, tea.KeyMsg{Type: tea.KeyCtrlC})
	if !m.quitting {
		t.Fatal("viewOpenDesc 下 ctrl+c 应退出")
	}
	if !isQuit(t, cmd) {
		t.Fatal("viewOpenDesc 下 ctrl+c 的 cmd 应为 tea.Quit")
	}
}

func TestModelDoctor(t *testing.T) {
	m := newModel()
	m, _ = update(m, listMsg{ws: twoWorkspaces()})
	m, cmd := update(m, key("d"))
	if m.state != viewDoctor {
		t.Fatalf("d 后 state 应为 viewDoctor，实际 %d", m.state)
	}
	if !m.busy {
		t.Fatal("进 doctor 时 busy 应为 true")
	}
	if cmd == nil {
		t.Fatal("d 应返回 doctorCmd 非 nil")
	}
	fs := []doctor.Finding{
		{Repo: "api", Kind: "orphan", Detail: "孤立 worktree"},
		{Repo: "web", Branch: "feat/alpha", Kind: "drift", Detail: "漂移"},
	}
	m, _ = update(m, doctorMsg{fs: fs})
	if len(m.findings) != 2 {
		t.Fatalf("doctorMsg 后 findings 应为 2，实际 %d", len(m.findings))
	}
	if m.busy {
		t.Fatal("doctorMsg 后 busy 应为 false")
	}
}

func TestModelConfirmCloseCancel(t *testing.T) {
	m := newModel()
	m, _ = update(m, listMsg{ws: twoWorkspaces()})
	m, _ = update(m, key("c"))
	if m.state != viewConfirmClose {
		t.Fatalf("c 后 state 应为 viewConfirmClose，实际 %d", m.state)
	}
	m, cmd := update(m, key("n"))
	if m.state != viewList {
		t.Fatalf("n 后 state 应回 viewList，实际 %d", m.state)
	}
	if m.busy {
		t.Fatal("取消不应置 busy")
	}
	if cmd != nil {
		t.Fatal("取消不应返回 cmd")
	}
}

func TestModelConfirmCloseConfirm(t *testing.T) {
	m := newModel()
	m, _ = update(m, listMsg{ws: twoWorkspaces()})
	m, _ = update(m, key("c"))
	m, cmd := update(m, key("y"))
	if m.state != viewList {
		t.Fatalf("y 后 state 应回 viewList，实际 %d", m.state)
	}
	if !m.busy {
		t.Fatal("确认回收后 busy 应为 true")
	}
	if cmd == nil {
		t.Fatal("确认回收应返回 closeCmd 非 nil")
	}
}

func TestModelConfirmCloseNoSelection(t *testing.T) {
	m := newModel()
	m, _ = update(m, key("c"))
	if m.state != viewList {
		t.Fatalf("无选中时 c 不应进 confirm，state 应为 viewList，实际 %d", m.state)
	}
}

func TestModelOpenMsgRefresh(t *testing.T) {
	m := newModel()
	m, _ = update(m, listMsg{ws: twoWorkspaces()})
	res := []workspace.RepoResult{
		{Repo: "api", Action: "created"},
		{Repo: "web", Action: "created"},
	}
	m, cmd := update(m, openMsg{res: res})
	if m.status == "" {
		t.Fatal("openMsg 后 status 应含摘要")
	}
	if !m.busy {
		t.Fatal("openMsg 后应触发刷新，busy 应为 true")
	}
	if cmd == nil {
		t.Fatal("openMsg 后应返回刷新 cmd 非 nil")
	}
	if _, ok := cmd().(listMsg); !ok {
		t.Fatal("openMsg 后刷新 cmd 应产出 listMsg")
	}
}

func TestModelCloseMsgRefresh(t *testing.T) {
	m := newModel()
	res := []workspace.RepoResult{{Repo: "api", Action: "removed"}}
	m, cmd := update(m, closeMsg{res: res})
	if m.status == "" {
		t.Fatal("closeMsg 后 status 应含摘要")
	}
	if cmd == nil {
		t.Fatal("closeMsg 后应返回刷新 cmd")
	}
	if _, ok := cmd().(listMsg); !ok {
		t.Fatal("closeMsg 刷新 cmd 应产出 listMsg")
	}
}

func TestModelPruneMsgRefresh(t *testing.T) {
	m := newModel()
	res := []doctor.PruneResult{{Repo: "api", Pruned: []string{"feat/old"}}}
	m, cmd := update(m, pruneMsg{res: res})
	if m.status == "" {
		t.Fatal("pruneMsg 后 status 应含摘要")
	}
	if cmd == nil {
		t.Fatal("pruneMsg 后应返回刷新 cmd")
	}
	if _, ok := cmd().(listMsg); !ok {
		t.Fatal("pruneMsg 刷新 cmd 应产出 listMsg")
	}
}

func TestModelQuitInList(t *testing.T) {
	m := newModel()
	m, _ = update(m, listMsg{ws: twoWorkspaces()})
	m, cmd := update(m, key("q"))
	if !m.quitting {
		t.Fatal("viewList 下 q 应置 quitting")
	}
	if !isQuit(t, cmd) {
		t.Fatal("viewList 下 q 的 cmd 应为 tea.Quit")
	}
}

func TestModelQuitInDetailReturnsList(t *testing.T) {
	m := newModel()
	m, _ = update(m, listMsg{ws: twoWorkspaces()})
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = update(m, detailMsg{ws: &m.workspaces[0]})
	m, cmd := update(m, key("q"))
	if m.state != viewList {
		t.Fatalf("viewDetail 下 q 应回 viewList，实际 %d", m.state)
	}
	if m.quitting {
		t.Fatal("viewDetail 下 q 不应退出")
	}
	if isQuit(t, cmd) {
		t.Fatal("viewDetail 下 q 不应返回 tea.Quit")
	}
}

func TestModelEscInDetailReturnsList(t *testing.T) {
	m := newModel()
	m, _ = update(m, listMsg{ws: twoWorkspaces()})
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.state != viewList {
		t.Fatalf("viewDetail 下 esc 应回 viewList，实际 %d", m.state)
	}
}

func TestModelQuitInDoctorReturnsList(t *testing.T) {
	m := newModel()
	m, _ = update(m, listMsg{ws: twoWorkspaces()})
	m, _ = update(m, key("d"))
	m, _ = update(m, doctorMsg{fs: nil})
	m, cmd := update(m, key("q"))
	if m.state != viewList {
		t.Fatalf("viewDoctor 下 q 应回 viewList，实际 %d", m.state)
	}
	if m.quitting {
		t.Fatal("viewDoctor 下 q 不应退出")
	}
	if isQuit(t, cmd) {
		t.Fatal("viewDoctor 下 q 不应返回 tea.Quit")
	}
}

func TestModelCtrlCQuitsGlobally(t *testing.T) {
	m := newModel()
	m, _ = update(m, listMsg{ws: twoWorkspaces()})
	m, _ = update(m, key("d"))
	m, cmd := update(m, tea.KeyMsg{Type: tea.KeyCtrlC})
	if !m.quitting {
		t.Fatal("ctrl+c 应全局退出")
	}
	if !isQuit(t, cmd) {
		t.Fatal("ctrl+c 的 cmd 应为 tea.Quit")
	}
}

func TestModelCtrlCQuitsFromOpen(t *testing.T) {
	m := newModel()
	m, _ = update(m, listMsg{ws: twoWorkspaces()})
	m, _ = update(m, key("o"))
	m, cmd := update(m, tea.KeyMsg{Type: tea.KeyCtrlC})
	if !m.quitting {
		t.Fatal("viewOpen 下 ctrl+c 应退出")
	}
	if !isQuit(t, cmd) {
		t.Fatal("viewOpen 下 ctrl+c 的 cmd 应为 tea.Quit")
	}
}

func TestModelWindowSize(t *testing.T) {
	m := newModel()
	m, _ = update(m, tea.WindowSizeMsg{Width: 100, Height: 40})
	if m.width != 100 || m.height != 40 {
		t.Fatalf("窗口尺寸应更新，实际 %dx%d", m.width, m.height)
	}
}

func TestModelRefreshKey(t *testing.T) {
	m := newModel()
	m, _ = update(m, listMsg{ws: twoWorkspaces()})
	m, cmd := update(m, key("r"))
	if !m.busy {
		t.Fatal("r 刷新应置 busy")
	}
	if cmd == nil {
		t.Fatal("r 刷新应返回 listCmd")
	}
}

func TestModelPruneKey(t *testing.T) {
	m := newModel()
	m, _ = update(m, listMsg{ws: twoWorkspaces()})
	m, cmd := update(m, key("p"))
	if !m.busy {
		t.Fatal("p 清理应置 busy")
	}
	if cmd == nil {
		t.Fatal("p 清理应返回 pruneCmd")
	}
}

func TestViewListHasDescriptionColumnNoTab(t *testing.T) {
	m := newModel()
	m, _ = update(m, listMsg{ws: twoWorkspaces()})
	out := m.View()
	if strings.Contains(out, "\t") {
		t.Fatal("列表不应含制表符 \\t")
	}
	if !strings.Contains(out, "描述") {
		t.Fatal("列表表头应含 描述 列")
	}
	if !strings.Contains(out, "修复登录") {
		t.Fatal("列表应渲染 workspace 描述")
	}
	if !strings.Contains(out, "—") {
		t.Fatal("空描述应显示 —")
	}
}

func TestViewListColumnsAligned(t *testing.T) {
	m := newModel()
	m, _ = update(m, listMsg{ws: twoWorkspaces()})
	out := m.View()
	lines := branchLines(out, []string{"feat/alpha", "feat/beta"})
	if len(lines) != 2 {
		t.Fatalf("应找到 2 行 workspace，实际 %d", len(lines))
	}
	col0 := branchColumn(lines[0], "feat/alpha")
	col1 := branchColumn(lines[1], "feat/beta")
	if col0 != col1 {
		t.Fatalf("列表分支列起始应对齐：col %d (%q) vs col %d (%q)", col0, lines[0], col1, lines[1])
	}
}

func TestViewDetailAheadAligned(t *testing.T) {
	m := newModel()
	m.state = viewDetail
	m.busy = false
	m.detail = &workspace.Workspace{
		Branch: "feat/alpha",
		Repos: []workspace.RepoStatus{
			{Repo: "api", Exists: true, Branch: "feat/alpha", Dirty: false, Ahead: 1, Behind: 2},
			{Repo: "longrepo", Exists: true, Branch: "feat/alpha", Dirty: true, Ahead: 12, Behind: 0},
		},
	}
	out := m.View()
	if strings.Contains(out, "\t") {
		t.Fatal("详情不应含制表符 \\t")
	}
	if !strings.Contains(out, "ahead") {
		t.Fatal("详情应含 ahead 列")
	}
	if !strings.Contains(out, "behind") {
		t.Fatal("详情应含 behind 列")
	}
}

func branchLines(out string, branches []string) []string {
	var res []string
	for _, line := range strings.Split(out, "\n") {
		for _, br := range branches {
			if strings.Contains(line, br) {
				res = append(res, line)
				break
			}
		}
	}
	return res
}

func branchColumn(line, branch string) int {
	plain := stripANSI(line)
	idx := strings.Index(plain, branch)
	if idx < 0 {
		return -1
	}
	return lipgloss.Width(plain[:idx])
}

func lineMatching(out string, subs ...string) bool {
	for _, line := range strings.Split(out, "\n") {
		all := true
		for _, s := range subs {
			if !strings.Contains(line, s) {
				all = false
				break
			}
		}
		if all {
			return true
		}
	}
	return false
}

func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		if inEsc {
			if r == 'm' {
				inEsc = false
			}
			continue
		}
		if r == 0x1b {
			inEsc = true
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func TestTeatestInteraction(t *testing.T) {
	m := newModel()
	ti := textinput.New()
	m.input = ti
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))

	out := tm.Output()

	tm.Send(listMsg{ws: twoWorkspaces()})

	teatest.WaitFor(t, out, func(b []byte) bool {
		return bytes.Contains(b, []byte("feat/alpha")) && bytes.Contains(b, []byte("feat/beta"))
	}, teatest.WithDuration(3*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	tm.Send(detailMsg{ws: &[]workspace.Workspace{{
		Branch: "feat/beta",
		Repos:  []workspace.RepoStatus{{Repo: "api", Exists: true, Branch: "feat/beta"}},
	}}[0]})

	teatest.WaitFor(t, out, func(b []byte) bool {
		return bytes.Contains(b, []byte("esc/q 返回"))
	}, teatest.WithDuration(3*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})

	teatest.WaitFor(t, out, func(b []byte) bool {
		return bytes.Contains(b, []byte("enter 详情"))
	}, teatest.WithDuration(3*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})

	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestTeatestOpenFlowWithDescription(t *testing.T) {
	m := newModel()
	m.input = textinput.New()
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))
	out := tm.Output()

	tm.Send(listMsg{ws: twoWorkspaces()})
	teatest.WaitFor(t, out, func(b []byte) bool {
		return bytes.Contains(b, []byte("feat/alpha"))
	}, teatest.WithDuration(3*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	teatest.WaitFor(t, out, func(b []byte) bool {
		return bytes.Contains(b, []byte("分支"))
	}, teatest.WithDuration(3*time.Second))

	tm.Type("feat/gamma")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	teatest.WaitFor(t, out, func(b []byte) bool {
		return bytes.Contains(b, []byte("描述"))
	}, teatest.WithDuration(3*time.Second))

	tm.Type("加购物车")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	ws := append(twoWorkspaces(), workspace.Workspace{
		Branch:      "feat/gamma",
		Description: "加购物车",
		Repos:       []workspace.RepoStatus{{Repo: "api", Exists: true, Branch: "feat/gamma"}},
	})
	tm.Send(listMsg{ws: ws})

	teatest.WaitFor(t, out, func(b []byte) bool {
		return bytes.Contains(b, []byte("feat/gamma")) && bytes.Contains(b, []byte("加购物车"))
	}, teatest.WithDuration(3*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func labeledManifest() *manifest.Manifest {
	return &manifest.Manifest{
		Project:         "demo",
		DefaultBaseline: "main",
		Repos: []manifest.Repo{
			{Name: "api", Label: "核心接口", Remote: "git@example.com:demo/api.git"},
			{Name: "web", Remote: "git@example.com:demo/web.git"},
		},
	}
}

func TestViewDetailShowsLabelColumn(t *testing.T) {
	m := New(testProject(), labeledManifest())
	m.state = viewDetail
	m.busy = false
	m.detail = &workspace.Workspace{
		Branch: "feat/alpha",
		Repos: []workspace.RepoStatus{
			{Repo: "api", Exists: true, Branch: "feat/alpha", Dirty: false, Ahead: 1},
			{Repo: "web", Exists: true, Branch: "feat/alpha", Dirty: true, Ahead: 0},
		},
	}
	out := stripANSI(m.View())
	if strings.Contains(out, "\t") {
		t.Fatal("详情不应含制表符 \\t")
	}
	if !strings.Contains(out, "名称") || !strings.Contains(out, "仓库") {
		t.Errorf("详情应含 名称 与 仓库 列: %q", out)
	}
	if !lineMatching(out, "核心接口", "api", "feat/alpha") {
		t.Errorf("详情应在 api 行显示 label 核心接口: %q", out)
	}
	if !lineMatching(out, "web", "feat/alpha") {
		t.Errorf("详情 web 行应回退工程名 web: %q", out)
	}
}

func TestViewDetailLabelColumnsAligned(t *testing.T) {
	m := New(testProject(), &manifest.Manifest{
		Project:         "demo",
		DefaultBaseline: "main",
		Repos: []manifest.Repo{
			{Name: "api", Label: "短"},
			{Name: "longrepo"},
		},
	})
	m.state = viewDetail
	m.busy = false
	m.detail = &workspace.Workspace{
		Branch: "feat/alpha",
		Repos: []workspace.RepoStatus{
			{Repo: "api", Exists: true, Branch: "feat/alpha", Dirty: false, Ahead: 1},
			{Repo: "longrepo", Exists: true, Branch: "feat/alpha", Dirty: true, Ahead: 12},
		},
	}
	out := stripANSI(m.View())
	rows := branchLines(out, []string{"短", "longrepo"})
	if len(rows) != 2 {
		t.Fatalf("expected 2 repo rows, got %d from %q", len(rows), out)
	}
	if c0, c1 := branchColumn(rows[0], "feat/alpha"), branchColumn(rows[1], "feat/alpha"); c0 != c1 {
		t.Errorf("分支列 CJK label 错位: %d vs %d (%q / %q)", c0, c1, rows[0], rows[1])
	}
}

func TestViewDoctorShowsLabelColumn(t *testing.T) {
	m := New(testProject(), labeledManifest())
	m.state = viewDoctor
	m.busy = false
	m.findings = []doctor.Finding{
		{Repo: "api", Branch: "feat/alpha", Kind: "drift", Detail: "漂移"},
		{Repo: "web", Kind: "orphan", Detail: "孤立 worktree"},
	}
	out := stripANSI(m.View())
	if strings.Contains(out, "\t") {
		t.Fatal("doctor 不应含制表符 \\t")
	}
	if !lineMatching(out, "核心接口", "api", "漂移") {
		t.Errorf("doctor 应在 api 行显示 label 核心接口: %q", out)
	}
	if !lineMatching(out, "web", "孤立 worktree") {
		t.Errorf("doctor web 行应回退工程名 web: %q", out)
	}
}

func TestModelInitWithInitialBranchEntersDetail(t *testing.T) {
	m := NewWithBranch(testProject(), testManifest(), "feat/alpha")
	if m.state != viewDetail {
		t.Fatalf("有 initialBranch 时初始 state 应为 viewDetail，实际 %d", m.state)
	}
	if cmd := m.Init(); cmd == nil {
		t.Fatal("有 initialBranch 时 Init 应返回 detailCmd 非 nil")
	}
}

func TestModelInitWithoutInitialBranchListsView(t *testing.T) {
	m := NewWithBranch(testProject(), testManifest(), "")
	if m.state != viewList {
		t.Fatalf("无 initialBranch 时初始 state 应为 viewList，实际 %d", m.state)
	}
	if cmd := m.Init(); cmd == nil {
		t.Fatal("Init 应返回非 nil cmd")
	}
}

func TestModelDetailDoctorKey(t *testing.T) {
	m := newModel()
	m, _ = update(m, listMsg{ws: twoWorkspaces()})
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = update(m, detailMsg{ws: &m.workspaces[0]})
	m, cmd := update(m, key("d"))
	if m.state != viewDoctor {
		t.Fatalf("详情页 d 后 state 应为 viewDoctor，实际 %d", m.state)
	}
	if !m.busy {
		t.Fatal("详情页 d 进体检时 busy 应为 true")
	}
	if cmd == nil {
		t.Fatal("详情页 d 应返回 doctorCmd 非 nil")
	}
}

func TestModelDetailSyncKey(t *testing.T) {
	m := newModel()
	m, _ = update(m, listMsg{ws: twoWorkspaces()})
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = update(m, detailMsg{ws: &m.workspaces[0]})
	m, cmd := update(m, key("s"))
	if !m.busy {
		t.Fatal("详情页 s 同步时 busy 应为 true")
	}
	if cmd == nil {
		t.Fatal("详情页 s 应返回 syncCmd 非 nil")
	}
}

func TestModelSyncMsgSummaryClearsBusy(t *testing.T) {
	m := newModel()
	m, _ = update(m, listMsg{ws: twoWorkspaces()})
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = update(m, detailMsg{ws: &m.workspaces[0]})
	m, _ = update(m, key("s"))
	res := []workspace.RepoResult{
		{Repo: "api", Action: "synced"},
		{Repo: "web", Action: "conflict"},
	}
	m, cmd := update(m, syncMsg{res: res})
	if m.status == "" {
		t.Fatal("syncMsg 后 status 应含摘要")
	}
	if !strings.Contains(m.status, "synced") || !strings.Contains(m.status, "conflict") {
		t.Fatalf("syncMsg 摘要应含 synced/conflict，实际 %q", m.status)
	}
	if cmd == nil {
		t.Fatal("syncMsg 后应返回刷新 detailCmd")
	}
}

func TestRootRowBlocksClose(t *testing.T) {
	m := newModel()
	m.state = viewList
	m.workspaces = []workspace.Workspace{
		{IsRoot: true},
		{Branch: "feat/x"},
	}
	m.cursor = 0
	updated, _ := m.handleListKey(key("c"))
	if updated.(Model).state == viewConfirmClose {
		t.Error("selecting root then pressing c must not enter confirm-close")
	}

	m2 := newModel()
	m2.state = viewList
	m2.workspaces = []workspace.Workspace{{IsRoot: true}, {Branch: "feat/x"}}
	m2.cursor = 1
	updated2, _ := m2.handleListKey(key("c"))
	if updated2.(Model).state != viewConfirmClose {
		t.Error("selecting a normal workspace then pressing c should enter confirm-close")
	}
}

func TestRootDisplayNameInList(t *testing.T) {
	if (workspace.Workspace{IsRoot: true}).DisplayName() != "root" {
		t.Error("root workspace DisplayName should be 'root'")
	}
}

type errStub string

func (e errStub) Error() string { return string(e) }
