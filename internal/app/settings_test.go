package app

import (
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func typeText(tt *ui.Tester, s string) {
	for _, r := range s {
		press(tt, string(r))
	}
}

func TestSettingsEditsTheConfigFile(t *testing.T) {
	a, tt, _ := newTestApp(t)
	file := a.settingsFile()
	if err := os.WriteFile(file, []byte("# mine\ndisableStartupPopups: true # keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.OpenSettings()
	tt.Frame()

	typeText(tt, "confirmonquit")
	press(tt, "enter")
	if !a.repo.Config.ConfirmOnQuit {
		t.Fatal("switch did not turn confirmOnQuit on")
	}

	press(tt, "esc")
	typeText(tt, "notarepository")
	press(tt, "enter")
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	press(tt, "enter")
	if got := a.repo.Config.NotARepository; got != "create" {
		t.Fatalf("notARepository = %q", got)
	}

	press(tt, "esc")
	typeText(tt, "gui.scrollheight")
	press(tt, "enter")
	tt.Key(0, ui.KeyBackspace)
	typeText(tt, "7")
	press(tt, "enter")
	if got := a.repo.Config.Gui.ScrollHeight; got != 7 {
		t.Fatalf("scrollHeight = %d", got)
	}

	press(tt, "esc")
	typeText(tt, "universal.quit")
	press(tt, "enter")
	for range len(strings.Join(a.repo.Config.Keybinding.Universal.Quit, ", ")) {
		tt.Key(0, ui.KeyBackspace)
	}
	typeText(tt, "Q, <c-q>")
	press(tt, "enter")
	if got := strings.Join(a.repo.Config.Keybinding.Universal.Quit, " "); !strings.HasPrefix(got, "Q <c-q>") {
		t.Fatalf("quit keys = %q", got)
	}

	data, _ := os.ReadFile(file)
	for _, want := range []string{"# mine", "disableStartupPopups: true # keep", "confirmOnQuit: true",
		"notARepository: create", "gui:\n  scrollHeight: 7", "quit: [Q, <c-q>]"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("config file lacks %q:\n%s", want, data)
		}
	}
	if _, ok := a.topOverlay().(*settingsOverlay); !ok {
		t.Fatal("settings closed after an edit")
	}
	press(tt, "esc")
	press(tt, "esc")
	if a.topOverlay() != nil {
		t.Fatal("esc did not close settings")
	}
}

func TestShotsSettings(t *testing.T) {
	out := shotDir()
	if out == "" {
		t.Skip("SHOT_DIR not set")
	}
	a, tt, _ := newTestApp(t)
	a.OpenSettings()
	tt.Frame()
	shot(t, tt, out+"/settings.png")
	typeText(tt, "theme")
	shot(t, tt, out+"/settings-filtered.png")
}
