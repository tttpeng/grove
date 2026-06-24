# grove

> **English** · [中文](README.zh-CN.md)

A general-purpose, cross-repo **git worktree workspace manager** (CLI / TUI).

For one feature that spans multiple git repositories, grove opens, closes and audits a *group* of same-named worktrees from a single manifest — atomically and idempotently. The name: a worktree is a "tree" in git, so one feature = a group of worktrees = a small **grove**.

## Why

In multi-repo projects a single feature often spans several repositories. Opening and closing same-named worktrees by hand in each repo is repetitive and error-prone (branch drift, non-atomic teardown, zombie worktrees). grove treats "a group of worktrees" as one atomic unit to create, tear down and verify.

## Install

### Homebrew (macOS / Linux)

```sh
brew install tttpeng/tap/grove
```

### go install

```sh
go install github.com/tttpeng/grove@latest
```

### Install script

```sh
curl -fsSL https://raw.githubusercontent.com/tttpeng/grove/main/install.sh | sh
```

Or grab a prebuilt binary from the [Releases](https://github.com/tttpeng/grove/releases) page.

## Quick start

```sh
# 1. Register a project from an index repo that holds workspace.yaml
grove project add demo --from git@example.com:demo/index.git

# 2. Clone all member repos declared in the manifest
grove bootstrap

# 3. Open a group of same-named worktrees for a feature
grove open feat/login

# 4. See the whole picture / per-feature detail
grove ls
grove status feat/login

# 5. Pull each repo's baseline (e.g. stage) into the feature branch
grove sync feat/login

# 6. Tear the group down (dirty / unpushed work is blocked)
grove close feat/login

# Or just run `grove` with no args for the interactive TUI
grove
```

## Commands

| Command | Purpose |
|---|---|
| `grove init` | Scan the current directory's sub-repos and scaffold a `workspace.yaml` |
| `grove project add <name> --from <git-url>` | Clone the index repo, read its manifest, register the project |
| `grove project list` · `grove use <name>` · `grove project remove <name>` | Manage registered projects |
| `grove bootstrap` | Clone every repo declared in the manifest |
| `grove open <branch> [--baseline <ref>] [--no-fetch] [-m <desc>]` | Create a same-named worktree in each repo (compensating, idempotent) |
| `grove close <branch> [--force] [--delete-branch]` | Tear down the whole group (blocks on dirty / unpushed) |
| `grove sync [<branch>]` | Merge each repo's baseline into the current branch (auto-stash, conflicts kept in place) |
| `grove ls` · `grove status [<branch>]` | Whole-machine workspace overview / per-feature repo detail (ahead/behind vs baseline) |
| `grove doctor [<branch>]` · `grove prune` | Consistency audit (drift / behind / dirty / zombie) / clean zombie worktrees |
| `grove describe <branch> [<desc>]` | Set/read a workspace description (stored as a git branch description) |
| `grove` (no args) | Interactive TUI (Bubble Tea): list / detail / doctor + open/close/sync/prune. Launched inside a worktree, it jumps straight to that workspace's detail. |

## Manifest

A project is described by a `workspace.yaml` living in an "index" repo (shared, travels with the project). Example:

```yaml
project: demo
defaultBaseline: stage
# Optional host: a repo that physically contains the others as a nested layout.
host:
  name: demo
  label: Demo Monorepo
  remote: git@example.com:demo/demo.git
  baseline: main
repos:
  - name: api
    label: Backend API
    remote: git@example.com:demo/api.git
  - name: web
    label: Web Frontend
    remote: git@example.com:demo/web.git
    baseline: main
```

- **`label`** — optional human-friendly display name shown beside the repo name.
- **`host`** (optional) — marks a repo that nests the others under a `repos/` subfolder, mirroring a physical mono-repo layout; members land in `repos/<repo>` so the host's own files stay clean.

Personal physical layout (clone/worktree roots) lives in `~/.grove/config.yaml`, keyed per project — separate from the shared manifest.

## How it works

- **Core / frontend split**: all business logic lives in `core` (no printing, no CLI/TUI imports). CLI and TUI are thin frontends.
- **git via subprocess**: shells out to the system `git` for a zero-dependency single-binary distribution.
- **git as the source of truth**: no separate state file. A workspace *is* the group of same-named worktrees across repos.

## Build & test

```sh
go build -o grove .
go test ./...
```

## Tech

Go · [Cobra](https://github.com/spf13/cobra) (CLI) · [Bubble Tea](https://github.com/charmbracelet/bubbletea) (TUI). Compiles to a zero-dependency, cross-platform single binary.

## License

[MIT](LICENSE)
