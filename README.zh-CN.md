# grove

> [English](README.md) · **中文**

通用的跨仓库 **git worktree 工作空间管理工具**（CLI / TUI）。

按一份清单，为一个需求在多个 git 仓库**开 / 收一组同名 worktree**，并校验一致性——原子、幂等。命名取义：worktree 在 git 中即“树”，一个需求 = 一组 worktree = 一片小树林 = **grove**。

## 为什么

多仓库项目里，一个功能需求往往横跨多个 git 仓库。在每个仓库手动开 / 收同名 worktree 既重复又易错（分支漂移、回收不原子、僵尸 worktree）。grove 把“一组 worktree”当作一个原子单位来创建、回收和校验。

## 安装

### Homebrew（macOS / Linux）

```sh
brew install tttpeng/tap/grove
```

### go install

```sh
go install github.com/tttpeng/grove@latest
```

### 安装脚本

```sh
curl -fsSL https://raw.githubusercontent.com/tttpeng/grove/main/install.sh | sh
```

或从 [Releases](https://github.com/tttpeng/grove/releases) 页面下载预编译二进制。

## 快速上手

```sh
# 1. 从持有 workspace.yaml 的索引仓库注册一个项目
grove project add demo --from git@example.com:demo/index.git

# 2. 按 manifest 拉齐所有成员仓库
grove bootstrap

# 3. 为一个需求在每个仓库开一组同名 worktree
grove open feat/login

# 4. 查看全貌 / 单需求逐仓库详情
grove ls
grove status feat/login

# 5. 把各仓库的基线（如 stage）合并进当前分支
grove sync feat/login

# 6. 回收整组（脏 / 未推送会被拦截）
grove close feat/login

# 或直接 `grove`（无参）进入交互式 TUI
grove
```

## 命令

| 命令 | 作用 |
|---|---|
| `grove init` | 扫描当前目录子仓库，生成 `workspace.yaml` 草稿 |
| `grove project add <name> --from <git-url>` | clone 索引仓库、读 manifest、注册项目 |
| `grove project list` · `grove use <project>` · `grove project remove <name>` | 项目管理 |
| `grove bootstrap` | 按 manifest 拉齐全部仓库 |
| `grove open <branch> [--baseline <ref>] [--no-fetch] [-m <描述>]` | 为需求在每个仓库建同名 worktree（补偿、幂等） |
| `grove close <branch> [--force] [--delete-branch]` | 回收整组 worktree（脏 / 未推送拦截） |
| `grove sync [<branch>]` | 把各仓库基线合并进当前分支（自动 stash，冲突保留现场） |
| `grove ls` · `grove status [<branch>]` | 本机 workspace 全貌 / 单需求逐仓库详情（相对基线的 ahead/behind） |
| `grove doctor [<branch>]` · `grove prune` | 一致性校验（漂移 / 落后 / 脏 / 僵尸）/ 清理僵尸 worktree |
| `grove describe <branch> [<描述>]` | 设置 / 读取 workspace 描述（存为 git branch description） |
| `grove`（无参） | 交互式 TUI（Bubble Tea）：列表 / 详情 / doctor + open/close/sync/prune；在某 worktree 目录内启动会直达该 workspace 详情 |

## Manifest

项目由一份 `workspace.yaml` 描述，它放在一个“索引”仓库里（共享，跟着项目走）。示例：

```yaml
project: demo
defaultBaseline: stage
# 可选 host：物理上以嵌套布局包含其他仓库的主仓库。
host:
  name: demo
  label: 示例主仓库
  remote: git@example.com:demo/demo.git
  baseline: main
repos:
  - name: api
    label: 后端服务
    remote: git@example.com:demo/api.git
  - name: web
    label: 前端 Web
    remote: git@example.com:demo/web.git
    baseline: main
```

- **`label`**：可选的真实显示名，展示在工程名旁。
- **`host`**（可选）：标记一个把其他仓库嵌套在 `repos/` 子目录下的主仓库，镜像物理 mono-repo 布局；成员落在 `repos/<repo>`，让主仓库自身内容保持干净。

个人物理布局（clone / worktree 根目录）放在 `~/.grove/config.yaml`，按项目分别配置——与共享 manifest 分离。

## 工作原理

- **core / 前端分离**：业务逻辑全部在 `core`（不打印、不引入 CLI/TUI）。CLI 与 TUI 都是薄前端。
- **git 子进程调用**：直接调系统 `git`，实现零依赖单二进制分发。
- **git 即状态来源**：无独立状态文件。一个 workspace 就是各仓库中同名 worktree 的集合。

## 构建与测试

```sh
go build -o grove .
go test ./...
```

## 技术栈

Go · [Cobra](https://github.com/spf13/cobra)（CLI）· [Bubble Tea](https://github.com/charmbracelet/bubbletea)（TUI）。编译为跨平台单二进制，零依赖分发。

## 许可证

[MIT](LICENSE)
