package app

import (
	"encoding/json"
	"fmt"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime/debug"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
	"github.com/jesseduffield/lazygit/pkg/config"
	"github.com/jesseduffield/lazygit/pkg/gui/controllers/helpers"
	guiTypes "github.com/jesseduffield/lazygit/pkg/gui/types"
	"github.com/jesseduffield/lazygit/pkg/integration/components"
	"github.com/jesseduffield/lazygit/pkg/integration/tests"
	integrationTypes "github.com/jesseduffield/lazygit/pkg/integration/types"

	"laziergit/internal/repo"
)

var _ integrationTypes.GuiDriver = (*parityDriver)(nil)

// parityResult is one lazygit integration test run against laziergit.
type parityResult struct {
	Name    string  `json:"name"`
	Status  string  `json:"status"` // pass, fail, skip
	Reason  string  `json:"reason,omitempty"`
	After   string  `json:"after,omitempty"`  // last key pressed before the failure
	Screen  string  `json:"screen,omitempty"` // the focused view at the failure
	Seconds float64 `json:"seconds"`
	// Progress is the share of the test's key presses that happened before
	// its first failed check; -1 when the total is unknown because the run
	// stopped early. Keys is how many keys were pressed in all.
	Progress float64 `json:"progress"`
	Keys     int     `json:"keys"`
	// Stopped says why a run ended before the test's script did.
	Stopped string `json:"stopped,omitempty"`
}

// repoRoot is resolved when the package loads, while the working directory is
// still this package's: repo.Open changes it later.
var repoRoot = func() string {
	wd, _ := os.Getwd()
	return filepath.Join(wd, "..", "..")
}()

// parityDir holds the committed parity state: the ratchet (passing.txt),
// the task backlog and the review list. See parity/AGENT.md.
var parityDir = filepath.Join(repoRoot, "parity")

// TestLazygitParity runs lazygit's own integration tests (pkg/integration/
// tests) against the headless app and reports how many pass. It is a report,
// not a gate: failures are expected while parity is incomplete, but a test
// listed in parity/passing.txt that fails is a regression and fails the run.
//
//	LAZYGIT_PARITY=1 go test ./internal/app -run TestLazygitParity -v
//
// LAZYGIT_PARITY_RUN filters tests by name (regexp). Every run writes
// parity/last-run.json (one result per test, with a screen dump per failure)
// and, with LAZYGIT_PARITY_SHOTS=dir, the last frame of each test as a PNG,
// rendered offscreen. LAZYGIT_PARITY_UPDATE=1 adds newly passing tests to
// parity/passing.txt (it never removes any).
func TestLazygitParity(t *testing.T) {
	if os.Getenv("LAZYGIT_PARITY") == "" {
		t.Skip("set LAZYGIT_PARITY=1 to run lazygit's integration tests against laziergit")
	}
	var filter *regexp.Regexp
	if f := os.Getenv("LAZYGIT_PARITY_RUN"); f != "" {
		filter = regexp.MustCompile(f)
	}
	results := runAllParity(t, func(name string) bool { return filter == nil || filter.MatchString(name) },
		fromRepoRoot(os.Getenv("LAZYGIT_PARITY_SHOTS")))
	passing := readPassing(t)
	t.Log("\n" + parityReport(results))
	b, _ := json.MarshalIndent(results, "", "  ")
	out := filepath.Join(parityDir, "last-run.json")
	if p := os.Getenv("LAZYGIT_PARITY_OUT"); p != "" {
		out = fromRepoRoot(p)
	}
	if err := os.WriteFile(out, b, 0o644); err != nil {
		t.Fatal(err)
	}
	failRegressions(t, results, passing)
	if os.Getenv("LAZYGIT_PARITY_UPDATE") != "" {
		added := 0
		for _, r := range results {
			if r.Status == "pass" && !passing[r.Name] {
				passing[r.Name] = true
				added++
			}
		}
		writePassing(t, passing)
		t.Logf("parity/passing.txt: %d tests (%d new)", len(passing), added)
	}
}

// TestParityRatchet runs the lazygit tests listed in parity/passing.txt. It is
// part of the normal go test run: a feature may not break a test that
// already passes. Skipped with -short.
func TestParityRatchet(t *testing.T) {
	if testing.Short() {
		t.Skip("lazygit parity ratchet")
	}
	passing := readPassing(t)
	results := runAllParity(t, func(name string) bool { return passing[name] }, "")
	failRegressions(t, results, passing)
	if len(results) != len(passing) {
		t.Errorf("parity/passing.txt lists %d tests, but only %d exist in this lazygit version", len(passing), len(results))
	}
}

// fromRepoRoot resolves a relative path from the repo root rather than this
// package's directory, where go test runs.
func fromRepoRoot(p string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(repoRoot, p)
}

func failRegressions(t *testing.T, results []parityResult, passing map[string]bool) {
	t.Helper()
	for _, r := range results {
		if passing[r.Name] && r.Status != "pass" {
			t.Errorf("REGRESSION %s (listed in parity/passing.txt): %s\n%s", r.Name, firstLine(r.Reason), r.Screen)
		}
	}
}

func readPassing(t *testing.T) map[string]bool {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(parityDir, "passing.txt"))
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]bool{}
	for _, l := range strings.Split(string(b), "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
			m[l] = true
		}
	}
	return m
}

func writePassing(t *testing.T, m map[string]bool) {
	t.Helper()
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)
	head := "# lazygit integration tests laziergit passes. TestParityRatchet keeps them passing.\n" +
		"# Add with LAZYGIT_PARITY=1 LAZYGIT_PARITY_UPDATE=1 go test ./internal/app -run TestLazygitParity; never remove by hand.\n"
	if err := os.WriteFile(filepath.Join(parityDir, "passing.txt"), []byte(head+strings.Join(names, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// runAllParity runs every lazygit test whose name passes want.
func runAllParity(t *testing.T, want func(string) bool, shots string) []parityResult {
	t.Helper()
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/jesseduffield/lazygit")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	root := strings.TrimSpace(string(out))
	verOut, err := exec.Command("git", "--version").Output()
	if err != nil {
		t.Fatal(err)
	}
	gitVersion, err := git_commands.ParseGitVersion(string(verOut))
	if err != nil {
		t.Fatal(err)
	}
	if shots != "" {
		// Absolute: each test runs with its own repo as the working directory.
		shots, _ = filepath.Abs(shots)
		if err := os.MkdirAll(shots, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)

	var results []parityResult
	for _, it := range tests.GetTests(root) {
		if !want(it.Name()) {
			continue
		}
		start := time.Now()
		r := runParity(t, root, gitVersion, it, shots)
		r.Name, r.Seconds = it.Name(), time.Since(start).Seconds()
		if strings.HasPrefix(r.Reason, "harness: ") || strings.HasPrefix(r.Stopped, "harness: ") {
			r.Status = "unreachable" // can't pass until the harness supports it
		}
		results = append(results, r)
		if r.Status == "fail" && testing.Verbose() {
			t.Logf("FAIL %s: %s", r.Name, firstLine(r.Reason))
		}
	}
	return results
}

func runParity(t *testing.T, root string, gitVersion *git_commands.GitVersion, it *components.IntegrationTest, shots string) parityResult {
	switch {
	case it.Skip():
		return parityResult{Status: "skip", Reason: "skipped by lazygit"}
	case it.IsDemo():
		return parityResult{Status: "skip", Reason: "demo recording"}
	case !it.ShouldRunForGitVersion(gitVersion):
		return parityResult{Status: "skip", Reason: fmt.Sprintf("not for git %d.%d.%d", gitVersion.Major, gitVersion.Minor, gitVersion.Patch)}
	case len(it.ExtraCmdArgs()) > 0:
		return parityResult{Status: "fail", Reason: "harness: command-line args not supported: " + strings.Join(it.ExtraCmdArgs(), " ")}
	}

	// lazygit's layout: helper files are found relative to the repo
	// (test/_results/<name>/actual/repo → test/files).
	tmp, err := os.MkdirTemp("", "laziergit-parity-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	testDir := filepath.Join(tmp, "test")
	actual := filepath.Join(testDir, "_results", filepath.FromSlash(it.Name()), "actual")
	repoDir := filepath.Join(actual, "repo")
	configDir := filepath.Join(tmp, "config")
	for _, d := range []string{repoDir, configDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(root, "test", "files"), filepath.Join(testDir, "files")); err != nil {
		t.Fatal(err)
	}
	copyFile(t, filepath.Join(root, "test", "global_git_config"), filepath.Join(testDir, "global_git_config"))
	copyFile(t, filepath.Join(root, "test", "default_test_config", "config.yml"), filepath.Join(configDir, "config.yml"))
	// Never sit in a directory that is about to be removed.
	cwd, _ := os.Getwd()
	os.Chdir(repoDir)
	defer os.Chdir(cwd)

	// lazygit's test environment: its own HOME and global git config, no
	// system config, nothing from the host that changes git's behaviour.
	env := map[string]string{
		"HOME": testDir, "GIT_CONFIG_GLOBAL": filepath.Join(testDir, "global_git_config"),
		"GIT_CONFIG_NOSYSTEM": "1", "CONFIG_DIR": configDir, "PWD": repoDir,
		"GH_PATH": filepath.Join(tmp, ".no-gh"), "GH_TOKEN": "", "GITHUB_TOKEN": "", "GH_TELEMETRY": "disabled",
		"GIT_AUTHOR_NAME": "", "GIT_AUTHOR_EMAIL": "", "GIT_COMMITTER_NAME": "", "GIT_COMMITTER_EMAIL": "",
		"GIT_EDITOR": "", "EDITOR": "", "VISUAL": "", "LG_CONFIG_FILE": "",
	}
	for k, v := range it.ExtraEnvVars() {
		env[k] = strings.NewReplacer("{{actualPath}}", actual, "{{actualRepoPath}}", repoDir).Replace(v)
	}
	// lazygit's git layer sets GIT_DIR/GIT_WORK_TREE for some repo layouts;
	// restore the whole environment, not just what we set.
	defer restoreEnviron(os.Environ())
	setEnv(env)

	var d *parityDriver
	r := capture(func() parityResult {
		shell := components.NewShell(repoDir, os.Environ(), func(msg string) { panic(parityFailure{msg: "setup: " + msg}) })
		shell.Init()
		it.SetupRepo(shell)
		// Setup may Chdir; lazygit opens wherever the shell ended up.
		dir := reflect.ValueOf(shell).Elem().FieldByName("dir").String()

		r, err := repo.OpenWithConfig(dir, it.SetupConfig)
		if err != nil {
			return parityResult{Status: "fail", Reason: "harness: open: " + err.Error()}
		}
		defer r.Close()
		sync := func(fn func()) { fn() }
		a := New(r, sync, sync, func() {})
		a.openLink = func(string) error { return nil }
		if r.Config.OS.OpenLink != "" {
			// A test's own os.openLink command (a shell stand-in, as in
			// lazygit's tests) runs headless; the default would open a browser.
			a.openLink = func(url string) error { return r.OS.OpenLink(url) }
		}
		a.copyText = func(string) error { return nil }
		// Subprocesses (editors, terminal custom commands) run headless, as
		// lazygit's integration tests run them, never in Terminal.app.
		a.terminalCommand = func(command string) error {
			tmp, err := os.MkdirTemp("", "parity-terminal-")
			if err != nil {
				return err
			}
			defer os.RemoveAll(tmp)
			result := filepath.Join(tmp, "result")
			cmd := exec.Command("/bin/sh", "-c", a.terminalScript(command, result))
			if out, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("%s%w", out, err)
			}
			code, err := os.ReadFile(result)
			if err != nil {
				return err
			}
			if strings.TrimSpace(string(code)) != "0" {
				return fmt.Errorf("command exited with status %s", code)
			}
			return nil
		}
		if r.Config.OS.CopyToClipboardCmd != "" {
			// The test configured its own clipboard command (it writes a
			// file in the repo), so lazygit's OS.CopyToClipboard runs it.
			a.copyText = r.OS.CopyToClipboard
		}
		a.Refresh()
		w, h := 1280, 800
		tw, th := it.HeadlessDimensions()
		sized := tw != 150 || th != 100 // lazygit's unexported defaultWidth/defaultHeight
		if sized {
			// The test sets its terminal size: one cell is one mono column
			// by one row.
			w, h = int(float32(tw)*monoAdvance()), th*rowHeight
		}
		tt := ui.NewTester(a.View, w, h)
		tt.Frame()
		if sized {
			// Then the window is sized so that the main pane holds as many
			// cells as lazygit's main view does in that terminal.
			// lazygit splits the main window for a selected file with both
			// staged and unstaged changes (or with gui.splitDiff always).
			panes := guiTypes.MainPaneOnly
			n := a.selectedFileNode()
			if a.focus == panelFiles && n != nil &&
				((n.GetHasStagedChanges() && n.GetHasUnstagedChanges()) || r.Config.Gui.SplitDiff == "always") {
				panes = guiTypes.BothMainPanes
			} else if a.focus == panelFiles && n != nil && n.GetHasStagedChanges() && !n.GetHasUnstagedChanges() {
				// lazygit's SecondaryPaneOnly: a file with nothing but staged
				// changes has them in the secondary pane alone.
				panes = guiTypes.SecondaryPaneOnly
			}
			if split := a.mainVisible() && a.secondaryVisible(); split != (panes == guiTypes.BothMainPanes) {
				return parityResult{Status: "fail", Reason: fmt.Sprintf("app: main window split is %v, lazygit's %v", split, !split)}
			}
			paneCells := a.MainPaneCells
			if panes == guiTypes.SecondaryPaneOnly {
				if a.mainVisible() || !a.secondaryVisible() {
					return parityResult{Status: "fail", Reason: "app: lazygit shows the secondary pane alone"}
				}
				paneCells = a.SecondaryPaneCells
			}
			innerW, innerH := lazygitMainInnerSize(r.Config, tw, th, panes, a.showLog)
			// A pixel width needn't be a whole number of columns, and with
			// the main panes side by side a pane gets only part of what the
			// window gains: step until the sizes agree.
			for range 12 {
				cols, rows := paneCells()
				if cols == innerW && rows == innerH {
					break
				}
				w += int(math.Ceil(float64(float32(innerW-cols) * monoAdvance())))
				h += (innerH - rows) * diffRowHeight
				tt.SetSize(w, h)
				tt.Frame()
			}
			if cols, rows := paneCells(); cols != innerW || rows != innerH {
				return parityResult{Status: "fail", Reason: fmt.Sprintf("harness: main pane is %dx%d cells, lazygit's main view %dx%d", cols, rows, innerW, innerH)}
			}
		}
		d = &parityDriver{a: a, tt: tt}
		if shots != "" {
			// Runs on failure too: the last frame shows where it went wrong.
			defer saveShot(tt, filepath.Join(shots, strings.ReplaceAll(it.Name(), "/", "__")+".png"))
		}
		it.Run(d)
		return parityResult{Status: "pass"}
	})
	return withProgress(r, d)
}

// withProgress folds the driver's record into the result: the first failed
// check is the reason, and progress is how far the script got before it.
func withProgress(r parityResult, d *parityDriver) parityResult {
	if d == nil { // failed during setup
		r.Progress = 0
		return r
	}
	r.Keys = d.keys
	stopped := r.Status == "fail" // panicked, unsupported or timed out
	if stopped {
		r.Stopped = firstLine(r.Reason)
		if os.Getenv("LAZYGIT_PARITY_STACK") != "" {
			r.Stopped = r.Reason
		}
	}
	switch {
	case d.first != nil:
		r.Status, r.Reason, r.After, r.Screen = "fail", d.first.msg, d.first.after, d.first.screen
		r.Progress = -1
		if !stopped && d.keys > 0 {
			r.Progress = float64(d.firstAt) / float64(d.keys)
		} else if !stopped {
			r.Progress = 0
		}
	case stopped:
		r.Progress = -1
	default:
		r.Progress = 1
	}
	return r
}

// capture runs fn, turning a lazygit assertion failure (or any panic) into a
// failed result. A test that hangs, e.g. waiting for credentials nobody
// types, is abandoned after a timeout.
func capture(fn func() parityResult) parityResult {
	done := make(chan parityResult, 1)
	go func() {
		defer func() {
			if v := recover(); v != nil {
				if f, ok := v.(parityFailure); ok {
					done <- parityResult{Status: "fail", Reason: f.msg, After: f.after, Screen: f.screen}
					return
				}
				done <- parityResult{Status: "fail", Reason: fmt.Sprintf("panic: %v\n%s", v, debug.Stack())}
			}
		}()
		done <- fn()
	}()
	select {
	case r := <-done:
		return r
	case <-time.After(30 * time.Second):
		return parityResult{Status: "fail", Reason: "harness: timed out"}
	}
}

// saveShot writes the tester's frame as a PNG. It is rendered offscreen: no
// window opens or takes focus.
func saveShot(tt *ui.Tester, path string) {
	tt.Frame()
	f, err := os.Create(path)
	if err != nil {
		return
	}
	defer f.Close()
	png.Encode(f, tt.Image())
}

func setEnv(env map[string]string) {
	for k, v := range env {
		if v == "" {
			os.Unsetenv(k)
		} else {
			os.Setenv(k, v)
		}
	}
}

func restoreEnviron(environ []string) {
	os.Clearenv()
	for _, kv := range environ {
		k, v, _ := strings.Cut(kv, "=")
		os.Setenv(k, v)
	}
}

func copyFile(t *testing.T, from, to string) {
	b, err := os.ReadFile(from)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(to, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func firstLine(s string) string {
	s, _, _ = strings.Cut(strings.TrimSpace(s), "\n")
	return s
}

// failureKind buckets a failure message so the report shows what blocks
// the most tests: values are blanked so similar assertions group together.
func failureKind(reason string) string {
	r := firstLine(reason)
	switch {
	case strings.HasPrefix(r, "harness: command-line args"):
		return "harness: command-line args not supported"
	case strings.HasPrefix(r, "harness: "):
		return r
	case strings.HasPrefix(r, "setup: "):
		return "setup failed"
	case r == "panic: failed to clear view buffer":
		return "text field: <ctrl+u> doesn't clear it"
	case strings.HasPrefix(r, "panic: "):
		return "panic"
	}
	r = viewName.ReplaceAllString(r, "view «$1»") // keep which view
	r = quoted.ReplaceAllString(r, "'…'")
	r = number.ReplaceAllString(r, "N")
	if len(r) > 90 {
		r = r[:90] + "…"
	}
	return r
}

var (
	quoted   = regexp.MustCompile(`'[^']*'`)
	viewName = regexp.MustCompile(`view '(\w+)'`)
	number   = regexp.MustCompile(`\b\d+\b`)
	// keyResponse matches failures where a key didn't open what lazygit
	// opens, so the report can list them by key.
	keyResponse = regexp.MustCompile(`^Expected .* to be focused|Unexpected view focused|Expected to be in window`)
)

func parityReport(results []parityResult) string {
	type tally struct {
		pass, fail, skip int
		harness          int     // unreachable: need harness support
		reach            float64 // sum of known progress
		known, unknown   int
	}
	areas := map[string]*tally{}
	var total tally
	kinds := map[string]int{}
	type kc struct {
		k string
		n int
	}
	for _, r := range results {
		area, _, _ := strings.Cut(r.Name, "/")
		if areas[area] == nil {
			areas[area] = &tally{}
		}
		tl := areas[area]
		if r.Status == "pass" || r.Status == "fail" {
			for _, t := range []*tally{tl, &total} {
				if r.Progress >= 0 {
					t.reach += r.Progress
					t.known++
				} else {
					t.unknown++
				}
			}
		}
		switch r.Status {
		case "pass":
			tl.pass++
			total.pass++
		case "fail":
			tl.fail++
			total.fail++
			kinds[failureKind(r.Reason)]++
		case "unreachable":
			tl.harness++
			total.harness++
		default:
			tl.skip++
			total.skip++
		}
	}
	var b strings.Builder
	pct := func(t tally) string {
		if t.pass+t.fail == 0 {
			return "-"
		}
		return fmt.Sprintf("%.0f%%", 100*float64(t.pass)/float64(t.pass+t.fail))
	}
	names := make([]string, 0, len(areas))
	for n := range areas {
		names = append(names, n)
	}
	sort.Strings(names)
	reach := func(t tally) string {
		if t.known == 0 {
			return "-"
		}
		return fmt.Sprintf("%.0f%%", 100*t.reach/float64(t.known))
	}
	fmt.Fprintf(&b, "%-22s %5s %5s %5s %7s %6s %6s %5s\n", "area", "pass", "fail", "skip", "harness", "parity", "reach", "n/a")
	for _, n := range names {
		tl := *areas[n]
		fmt.Fprintf(&b, "%-22s %5d %5d %5d %7d %6s %6s %5d\n", n, tl.pass, tl.fail, tl.skip, tl.harness, pct(tl), reach(tl), tl.unknown)
	}
	fmt.Fprintf(&b, "%-22s %5d %5d %5d %7d %6s %6s %5d\n", "TOTAL", total.pass, total.fail, total.skip, total.harness, pct(total), reach(total), total.unknown)
	b.WriteString("parity: tests that match lazygit end to end (pass / (pass+fail)). reach: average share of\n" +
		"each test's key presses made before its first failed check. harness: tests that can't pass until the\n" +
		"harness supports something (mouse on terminal cells, command-line args); not counted.\n" +
		"n/a: runs that stopped early, so reach is unknown.\n")
	b.WriteString("\nfailures by first assertion:\n")
	var ks []kc
	for k, n := range kinds {
		ks = append(ks, kc{k, n})
	}
	sort.Slice(ks, func(i, j int) bool { return ks[i].n > ks[j].n || ks[i].n == ks[j].n && ks[i].k < ks[j].k })
	for _, k := range ks {
		fmt.Fprintf(&b, "%5d  %s\n", k.n, k.k)
	}
	b.WriteString("\nkeys that didn't open what lazygit opens (key in view → expected):\n")
	keys := map[string]int{}
	for _, r := range results {
		if r.Status == "fail" && r.After != "" && keyResponse.MatchString(r.Reason) {
			keys[r.After+" → "+strings.TrimSuffix(firstLine(r.Reason), " to be focused")]++
		}
	}
	ks = ks[:0]
	for k, n := range keys {
		ks = append(ks, kc{k, n})
	}
	sort.Slice(ks, func(i, j int) bool { return ks[i].n > ks[j].n || ks[i].n == ks[j].n && ks[i].k < ks[j].k })
	for _, k := range ks {
		fmt.Fprintf(&b, "%5d  %s\n", k.n, k.k)
	}
	return b.String()
}

// lazygitMainInnerSize is the inner size of lazygit's main view in a
// tw x th terminal at startup (files focused), from its own
// window arrangement.
func lazygitMainInnerSize(cfg *config.UserConfig, tw, th int, panes guiTypes.MainPanes, showExtras bool) (int, int) {
	dims := helpers.GetWindowDimensions(helpers.WindowArrangementArgs{
		Width: tw, Height: th, UserConfig: cfg,
		CurrentWindow: "files", CurrentSideWindow: "files",
		ActiveViewForWindow:    func(w string) string { return w },
		ContentHeightForWindow: func(string) int { return 0 },
		MainPanes:              panes,
		ScreenMode:             guiTypes.SCREEN_NORMAL,
		ShowExtrasWindow:       showExtras, // lazygit's, from gui.showCommandLog at startup
	})
	m := dims["main"]
	if panes == guiTypes.SecondaryPaneOnly {
		m = dims["secondary"]
	}
	return m.X1 - m.X0 + 1 - 2, m.Y1 - m.Y0 + 1 - 2
}
