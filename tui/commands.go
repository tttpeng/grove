package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/tttpeng/grove/core/config"
	"github.com/tttpeng/grove/core/doctor"
	"github.com/tttpeng/grove/core/manifest"
	"github.com/tttpeng/grove/core/workspace"
)

type listMsg struct {
	ws  []workspace.Workspace
	err error
}

type detailMsg struct {
	ws  *workspace.Workspace
	err error
}

type doctorMsg struct {
	fs  []doctor.Finding
	err error
}

type openMsg struct {
	res []workspace.RepoResult
	err error
}

type closeMsg struct {
	res []workspace.RepoResult
	err error
}

type pruneMsg struct {
	res []doctor.PruneResult
	err error
}

type syncMsg struct {
	res []workspace.RepoResult
	err error
}

func listCmd(rp config.ResolvedProject, m *manifest.Manifest) tea.Cmd {
	return func() tea.Msg {
		ws, err := workspace.List(rp, m)
		return listMsg{ws: ws, err: err}
	}
}

func detailCmd(rp config.ResolvedProject, m *manifest.Manifest, branch string) tea.Cmd {
	return func() tea.Msg {
		ws, err := workspace.Status(rp, m, branch)
		return detailMsg{ws: ws, err: err}
	}
}

func doctorCmd(rp config.ResolvedProject, m *manifest.Manifest, branch string) tea.Cmd {
	return func() tea.Msg {
		fs, err := doctor.Check(rp, m, branch)
		return doctorMsg{fs: fs, err: err}
	}
}

func syncCmd(rp config.ResolvedProject, m *manifest.Manifest, branch string) tea.Cmd {
	return func() tea.Msg {
		res, err := workspace.Sync(rp, m, branch)
		return syncMsg{res: res, err: err}
	}
}

func openCmd(rp config.ResolvedProject, m *manifest.Manifest, branch, desc string) tea.Cmd {
	return func() tea.Msg {
		res, err := workspace.Open(rp, m, branch, workspace.OpenOptions{Description: desc})
		return openMsg{res: res, err: err}
	}
}

func closeCmd(rp config.ResolvedProject, m *manifest.Manifest, branch string, opts workspace.CloseOptions) tea.Cmd {
	return func() tea.Msg {
		res, err := workspace.Close(rp, m, branch, opts)
		return closeMsg{res: res, err: err}
	}
}

func pruneCmd(rp config.ResolvedProject, m *manifest.Manifest) tea.Cmd {
	return func() tea.Msg {
		res, err := doctor.Prune(rp, m)
		return pruneMsg{res: res, err: err}
	}
}

func detailRootCmd(rp config.ResolvedProject, m *manifest.Manifest) tea.Cmd {
	return func() tea.Msg {
		ws := workspace.RootWorkspace(rp, m)
		return detailMsg{ws: &ws}
	}
}

func syncRootCmd(rp config.ResolvedProject, m *manifest.Manifest) tea.Cmd {
	return func() tea.Msg {
		res, err := workspace.SyncRoot(rp, m)
		return syncMsg{res: res, err: err}
	}
}
