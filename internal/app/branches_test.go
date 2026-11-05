package app

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/jesseduffield/lazygit/pkg/commands/models"

	"laziergit/internal/testrepo"
)

// cleanRepo is testrepo.New without the working-tree changes.
func cleanRepo(t *testing.T) string {
	t.Helper()
	dir := testrepo.New(t)
	git(t, dir, "reset", "--hard", "HEAD")
	git(t, dir, "clean", "-fd")
	return dir
}

func commitFile(t *testing.T, dir, name, body, msg string) {
	t.Helper()
	testrepo.Write(t, dir, name, body)
	git(t, dir, "add", name)
	git(t, dir, "commit", "-qm", msg)
}

func rev(t *testing.T, dir, ref string) string {
	t.Helper()
	return strings.TrimSpace(git(t, dir, "rev-parse", ref))
}

// selectBranchByKeys focuses the branches panel and walks to name.
func selectBranchByKeys(t *testing.T, a *App, tt *ui.Tester, name string) {
	t.Helper()
	press(tt, "3")
	pressUntil(t, tt, "j", func() bool { b := a.selectedBranch(); return b != nil && b.Name == name })
}

func menuLabels(m *menuOverlay) []string {
	var out []string
	for _, it := range m.items {
		out = append(out, it.label)
	}
	return out
}

func TestRenameBranch(t *testing.T) {
	dir := cleanRepo(t)
	a, tt := openApp(t, dir)
	selectBranchByKeys(t, a, tt, "feature")
	press(tt, "R")
	o, ok := a.topOverlay().(*promptOverlay)
	if !ok || o.value != "feature" {
		t.Fatalf("rename prompt: %T %+v", a.topOverlay(), o)
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("my topic")
	press(tt, "enter")
	if out := git(t, dir, "branch", "--list", "my-topic"); !strings.Contains(out, "my-topic") {
		t.Fatalf("branches: %s", git(t, dir, "branch"))
	}
	if b := a.selectedBranch(); b == nil || b.Name != "my-topic" {
		t.Fatalf("selection did not follow the rename: %+v", b)
	}
}

func TestRenameTrackingBranchWarns(t *testing.T) {
	dir := cleanRepo(t)
	git(t, dir, "config", "branch.feature.remote", "origin")
	git(t, dir, "config", "branch.feature.merge", "refs/heads/feature")
	a, tt := openApp(t, dir)
	selectBranchByKeys(t, a, tt, "feature")
	press(tt, "R")
	if _, ok := a.topOverlay().(*confirmOverlay); !ok {
		t.Fatalf("expected warning, got %T", a.topOverlay())
	}
	press(tt, "enter")
	if _, ok := a.topOverlay().(*promptOverlay); !ok {
		t.Fatalf("expected prompt, got %T", a.topOverlay())
	}
}

func TestMergeFastForward(t *testing.T) {
	dir := cleanRepo(t)
	git(t, dir, "checkout", "-q", "feature")
	commitFile(t, dir, "b.txt", "b\n", "feature work")
	git(t, dir, "checkout", "-q", "main")
	a, tt := openApp(t, dir)
	selectBranchByKeys(t, a, tt, "feature")
	press(tt, "M")
	m, ok := a.topOverlay().(*menuOverlay)
	if !ok {
		t.Fatalf("merge menu: %T", a.topOverlay())
	}
	if l := menuLabels(m); l[0] != "Regular merge (fast-forward)" || l[1] != "Regular merge (with merge commit)" {
		t.Fatalf("labels %q", l)
	}
	press(tt, "m")
	if rev(t, dir, "main") != rev(t, dir, "feature") {
		t.Fatal("main was not fast-forwarded")
	}
	if !strings.Contains(a.status, "Merged feature into main") {
		t.Fatalf("status %q", a.status)
	}
}

func TestMergeDivergedDisablesFastForward(t *testing.T) {
	dir := cleanRepo(t)
	git(t, dir, "checkout", "-q", "feature")
	commitFile(t, dir, "b.txt", "b\n", "feature work")
	git(t, dir, "checkout", "-q", "main")
	commitFile(t, dir, "c.txt", "c\n", "main work")
	a, tt := openApp(t, dir)
	selectBranchByKeys(t, a, tt, "feature")
	press(tt, "M")
	m := a.topOverlay().(*menuOverlay)
	if m.items[0].label != "Regular merge (with merge commit)" || m.items[1].disabled == "" {
		t.Fatalf("items %+v", m.items)
	}
	if out := shotDir(); out != "" {
		shot(t, tt, out+"/merge-menu.png")
	}
	press(tt, "f") // disabled: stays open with the reason
	if a.topOverlay() != m || !a.statusErr {
		t.Fatalf("disabled item ran: %T %q", a.topOverlay(), a.status)
	}
	press(tt, "S")
	if msg := git(t, dir, "log", "-1", "--format=%s"); !strings.Contains(msg, "feature") {
		t.Fatalf("squash commit message %q", msg)
	}
	if n := strings.TrimSpace(git(t, dir, "rev-list", "--count", "--merges", "HEAD")); n != "0" {
		t.Fatal("squash merge made a merge commit")
	}
	if b := git(t, dir, "show", "HEAD:b.txt"); b != "b\n" {
		t.Fatalf("b.txt %q", b)
	}
}

func TestRebaseOntoBranch(t *testing.T) {
	dir := cleanRepo(t)
	git(t, dir, "checkout", "-q", "feature")
	commitFile(t, dir, "b.txt", "b\n", "feature work")
	git(t, dir, "checkout", "-q", "main")
	commitFile(t, dir, "c.txt", "c\n", "main work")
	a, tt := openApp(t, dir)
	selectBranchByKeys(t, a, tt, "feature")
	press(tt, "r")
	if _, ok := a.topOverlay().(*menuOverlay); !ok {
		t.Fatalf("rebase menu: %T", a.topOverlay())
	}
	press(tt, "s")
	if rev(t, dir, "main^") != rev(t, dir, "feature") {
		t.Fatalf("main not rebased onto feature:\n%s", git(t, dir, "log", "--oneline", "--graph", "--all"))
	}
}

func TestRebaseConflictOffersAbort(t *testing.T) {
	dir := cleanRepo(t)
	git(t, dir, "checkout", "-q", "feature")
	commitFile(t, dir, "a.go", "feature\n", "feature edit")
	git(t, dir, "checkout", "-q", "main")
	commitFile(t, dir, "a.go", "main\n", "main edit")
	a, tt := openApp(t, dir)
	selectBranchByKeys(t, a, tt, "feature")
	press(tt, "r")
	press(tt, "s")
	m, ok := a.topOverlay().(*menuOverlay)
	if !ok || m.title != "Conflicts!" {
		t.Fatalf("conflicts menu: %T", a.topOverlay())
	}
	if a.snap.Operation != "Rebasing" {
		t.Fatalf("operation %q", a.snap.Operation)
	}
	press(tt, "j")
	press(tt, "enter")
	if a.snap.Operation != "" {
		t.Fatalf("rebase not aborted: %q", a.snap.Operation)
	}
	if b := git(t, dir, "show", "HEAD:a.go"); b != "main\n" {
		t.Fatalf("a.go %q", b)
	}
}

func TestInteractiveRebaseEditsTodos(t *testing.T) {
	dir := cleanRepo(t)
	commitFile(t, dir, "c.txt", "c\n", "keep me")
	commitFile(t, dir, "d.txt", "d\n", "drop me")
	a, tt := openApp(t, dir)
	selectBranchByKeys(t, a, tt, "feature")
	press(tt, "r")
	press(tt, "i")
	if a.focus != panelCommits || a.snap.Operation != "Rebasing" {
		t.Fatalf("focus %v op %q overlay %T", a.focus, a.snap.Operation, a.topOverlay())
	}
	pressUntil(t, tt, "j", func() bool { return a.selectedCommit().Name == "drop me" })
	if !a.selectedCommit().IsTODO() {
		t.Fatalf("not a todo: %+v", a.selectedCommit())
	}
	press(tt, "d")
	if c := a.snap.Commits[a.cursor[panelCommits]]; c.Action.String() != "drop" {
		t.Fatalf("action %q", c.Action.String())
	}
	if out := shotDir(); out != "" {
		shot(t, tt, out+"/rebase-todo.png")
	}
	a.run("Continuing…", func() error { return a.repo.RebaseControl("continue") }, nil)
	tt.Frame()
	if log := git(t, dir, "log", "--format=%s"); strings.Contains(log, "drop me") || !strings.Contains(log, "keep me") {
		t.Fatalf("log:\n%s", log)
	}
}

// withRemote gives dir an origin (a bare clone) that feature tracks.
func withRemote(t *testing.T, dir string) string {
	t.Helper()
	remote := filepath.Join(t.TempDir(), "remote.git")
	git(t, dir, "clone", "-q", "--bare", dir, remote)
	git(t, dir, "remote", "add", "origin", remote)
	git(t, dir, "fetch", "-q", "origin")
	git(t, dir, "branch", "-q", "-u", "origin/feature", "feature")
	git(t, dir, "branch", "-q", "-u", "origin/main", "main")
	return remote
}

// pushFromClone adds a commit to branch on the remote from a second clone.
func pushFromClone(t *testing.T, remote, branch, file string) string {
	t.Helper()
	other := filepath.Join(t.TempDir(), "other")
	git(t, filepath.Dir(other), "clone", "-q", "-b", branch, remote, other)
	commitFile(t, other, file, "upstream\n", "upstream work")
	git(t, other, "push", "-q", "origin", branch)
	return rev(t, other, "HEAD")
}

func TestFastForwardOtherBranch(t *testing.T) {
	dir := cleanRepo(t)
	remote := withRemote(t, dir)
	want := pushFromClone(t, remote, "feature", "b.txt")
	a, tt := openApp(t, dir)
	selectBranchByKeys(t, a, tt, "feature")
	press(tt, "f")
	if got := rev(t, dir, "feature"); got != want {
		t.Fatalf("feature at %s, want %s (status %q)", got, want, a.status)
	}
	if b := a.selectedBranch(); b.Name != "feature" || b.Head {
		t.Fatalf("selection moved or branch checked out: %+v", b)
	}
}

func TestFastForwardCheckedOutBranch(t *testing.T) {
	dir := cleanRepo(t)
	remote := withRemote(t, dir)
	want := pushFromClone(t, remote, "main", "b.txt")
	a, tt := openApp(t, dir)
	press(tt, "3")
	press(tt, "f")
	if got := rev(t, dir, "HEAD"); got != want {
		t.Fatalf("HEAD at %s, want %s (status %q)", got, want, a.status)
	}
	if b := git(t, dir, "show", "HEAD:b.txt"); b != "upstream\n" {
		t.Fatalf("worktree not updated: %q", b)
	}
}

func TestFastForwardRefusesLocalCommits(t *testing.T) {
	dir := cleanRepo(t)
	remote := withRemote(t, dir)
	pushFromClone(t, remote, "feature", "b.txt")
	git(t, dir, "checkout", "-q", "feature")
	commitFile(t, dir, "mine.txt", "mine\n", "local only")
	git(t, dir, "checkout", "-q", "main")
	git(t, dir, "fetch", "-q", "origin") // diverged, not just ahead
	before := rev(t, dir, "feature")
	a, tt := openApp(t, dir)
	selectBranchByKeys(t, a, tt, "feature")
	press(tt, "f")
	if _, ok := a.topOverlay().(*errorOverlay); !ok {
		t.Fatalf("expected error, got %T (status %q)", a.topOverlay(), a.status)
	}
	if rev(t, dir, "feature") != before {
		t.Fatal("feature moved and lost a local commit")
	}
}

func TestPullRequestActions(t *testing.T) {
	dir := cleanRepo(t)
	git(t, dir, "remote", "add", "origin", "git@github.com:owner/repo.git")
	for _, b := range []string{"main", "feature"} {
		git(t, dir, "config", "branch."+b+".remote", "origin")
		git(t, dir, "config", "branch."+b+".merge", "refs/heads/"+b)
	}
	git(t, dir, "update-ref", "refs/remotes/origin/feature", "HEAD")
	a, tt := openApp(t, dir)
	var opened, copied []string
	a.openLink = func(u string) error { opened = append(opened, u); return nil }
	a.copyText = func(s string) error { copied = append(copied, s); return nil }
	selectBranchByKeys(t, a, tt, "feature")

	press(tt, "o")
	if len(opened) != 1 || opened[0] != "https://github.com/owner/repo/compare/feature?expand=1" {
		t.Fatalf("opened %q", opened)
	}

	press(tt, "O")
	m, ok := a.topOverlay().(*menuOverlay)
	if !ok {
		t.Fatalf("PR menu: %T", a.topOverlay())
	}
	want := []string{"main → feature", "main → default branch", "main → select branch", "feature → default branch", "feature → select branch"}
	if strings.Join(menuLabels(m), "|") != strings.Join(want, "|") {
		t.Fatalf("labels %q", menuLabels(m))
	}
	press(tt, "enter")
	if len(opened) != 2 || opened[1] != "https://github.com/owner/repo/compare/feature...main?expand=1" {
		t.Fatalf("opened %q", opened)
	}

	press(tt, "O")
	pressUntil(t, tt, "j", func() bool { return a.topOverlay().(*menuOverlay).cursor == 4 })
	press(tt, "enter")
	if _, ok := a.topOverlay().(*promptOverlay); !ok {
		t.Fatalf("target prompt: %T", a.topOverlay())
	}
	tt.Type("develop")
	press(tt, "enter")
	if len(opened) != 3 || opened[2] != "https://github.com/owner/repo/compare/develop...feature?expand=1" {
		t.Fatalf("opened %q", opened)
	}

	tt.Key(ui.Ctrl, ui.KeyY)
	tt.Frame()
	if len(copied) != 1 || copied[0] != opened[0] {
		t.Fatalf("copied %q", copied)
	}

	press(tt, "G") // no PR known for this branch
	if !a.statusErr || len(opened) != 3 {
		t.Fatalf("G without a PR: %q %q", a.status, opened)
	}

	// What GitHub would report (fetching needs the network and a gh login).
	a.prs = map[string]*models.GithubPullRequest{
		"feature": {Number: 42, State: "OPEN", Title: "Add the feature", Url: "https://github.com/owner/repo/pull/42"},
		"main":    {Number: 7, State: "MERGED", Title: "Old main PR", Url: "https://github.com/owner/repo/pull/7"},
	}
	a.diffKey = ""
	tt.Frame()
	if !tt.HasText("#42") || !tt.HasText("Add the feature") || tt.HasText("#7") {
		t.Fatalf("PR display: %q", tt.Texts())
	}
	if out := shotDir(); out != "" {
		shot(t, tt, out+"/pull-request.png")
	}
	press(tt, "G")
	if opened[len(opened)-1] != "https://github.com/owner/repo/pull/42" {
		t.Fatalf("opened %q", opened)
	}
	tt.Key(ui.Ctrl, ui.KeyY)
	tt.Frame()
	if copied[len(copied)-1] != "https://github.com/owner/repo/pull/42" {
		t.Fatalf("copied %q", copied)
	}
}
