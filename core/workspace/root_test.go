package workspace_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tttpeng/grove/core/git"
	"github.com/tttpeng/grove/core/workspace"
)

func advanceRemote(t *testing.T, root, bare, body string) {
	t.Helper()
	work := filepath.Join(t.TempDir(), "adv")
	gitCmd(t, root, "clone", bare, work)
	if err := os.WriteFile(filepath.Join(work, "README.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, work, "commit", "-am", "advance")
	gitCmd(t, work, "push", "origin", "main")
}

func TestSyncRootFastForwards(t *testing.T) {
	rp, m := setupHost(t, "a", "b")
	cloneMembers(t, rp, m)

	advanceRemote(t, rp.CloneRoot, m.Host.Remote, "host-v2\n")
	advanceRemote(t, rp.CloneRoot, m.Repos[0].Remote, "a-v2\n")

	results, err := workspace.SyncRoot(rp, m)
	if err != nil {
		t.Fatalf("SyncRoot: %v", err)
	}
	byName := map[string]workspace.RepoResult{}
	for _, r := range results {
		byName[r.Repo] = r
	}
	if got := byName["erp-main"].Action; got != "updated" {
		t.Errorf("host action = %q, want updated", got)
	}
	if got := byName["a"].Action; got != "updated" {
		t.Errorf("member a action = %q, want updated", got)
	}
	if got := byName["b"].Action; got != "up-to-date" {
		t.Errorf("member b action = %q, want up-to-date", got)
	}

	hostHead := gitCmd(t, rp.CloneRoot, "rev-parse", "HEAD")
	hostUpstream := gitCmd(t, rp.CloneRoot, "rev-parse", "origin/main")
	if hostHead != hostUpstream {
		t.Errorf("host HEAD %q != origin/main %q after sync", hostHead, hostUpstream)
	}
}

func TestSyncRootSkipsDirty(t *testing.T) {
	rp, m := setupHost(t, "a")
	cloneMembers(t, rp, m)
	advanceRemote(t, rp.CloneRoot, m.Repos[0].Remote, "a-v2\n")

	memberClone := workspace.MemberCloneDir(rp, m, "a")
	if err := os.WriteFile(filepath.Join(memberClone, "README.md"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := workspace.SyncRoot(rp, m)
	if err != nil {
		t.Fatalf("SyncRoot: %v", err)
	}
	for _, r := range results {
		if r.Repo == "a" {
			if r.Action != "skipped" {
				t.Errorf("dirty member action = %q, want skipped", r.Action)
			}
		}
	}
	if !git.IsRepo(memberClone) {
		t.Error("dirty member clone should be untouched")
	}
}

func TestRootWorkspaceStatus(t *testing.T) {
	rp, m := setupHost(t, "a", "b")
	cloneMembers(t, rp, m)
	advanceRemote(t, rp.CloneRoot, m.Repos[0].Remote, "a-v2\n")
	gitCmd(t, workspace.MemberCloneDir(rp, m, "a"), "fetch", "origin", "main")

	ws := workspace.RootWorkspace(rp, m)
	if !ws.IsRoot {
		t.Error("RootWorkspace.IsRoot should be true")
	}
	if ws.DisplayName() != "root" {
		t.Errorf("DisplayName = %q, want root", ws.DisplayName())
	}
	if len(ws.Repos) != 3 {
		t.Fatalf("root repos = %d, want 3 (host + 2 members)", len(ws.Repos))
	}
	byName := map[string]int{}
	for _, r := range ws.Repos {
		if r.Branch != "main" {
			t.Errorf("%s branch = %q, want main", r.Repo, r.Branch)
		}
		byName[r.Repo] = r.Behind
	}
	if byName["a"] == 0 {
		t.Error("member a should be behind after remote advanced")
	}
}

func TestListPutsRootFirst(t *testing.T) {
	rp, m := setupHost(t, "a")
	cloneMembers(t, rp, m)

	wss, err := workspace.List(rp, m)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(wss) == 0 || !wss[0].IsRoot {
		t.Fatalf("List[0] should be the root workspace, got %+v", wss)
	}
}
