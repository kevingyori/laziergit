package app

import (
	"github.com/jesseduffield/lazygit/pkg/config"
	"laziergit/internal/keymap"
	"testing"
)

func TestPromptSuggestionsKeyboardCheckout(t *testing.T) {
	dir := cleanRepo(t)
	a, tt := openApp(t, dir)
	var selected string
	a.promptSuggestions("Branch", "", "", "refs", func(v string) { selected = v })
	tt.Frame()
	o := a.topOverlay().(*promptOverlay)
	o.value = "HEAD"
	tt.Frame()
	press(tt, "tab")
	if !o.suggestionsFocused {
		t.Fatal("tab did not focus suggestions")
	}
	o.key(a, keymap.Parse(a.repo.Config.Keybinding.Universal.ConfirmSuggestion)[0])
	if selected != "HEAD" {
		t.Fatalf("selected %q", selected)
	}
}

func TestShellHistoryFilteringDeletionAndIgnoreSpace(t *testing.T) {
	dir := cleanRepo(t)
	a, tt := openApp(t, dir)
	a.repo.Common.AppState.ShellCommandsHistory = []string{"git status", "git diff", "git log"}
	a.shellCommand()
	o := a.topOverlay().(*promptOverlay)
	o.value = "diff"
	if got := o.matches(); len(got) != 1 || got[0] != "git diff" {
		t.Fatalf("matches: %v", got)
	}
	o.deleteSuggestion("git diff")
	if got := o.matches(); len(got) != 0 {
		t.Fatalf("deleted match remains: %v", got)
	}
	o.submit(a, " git status")
	tt.Frame()
	if got := a.repo.Common.AppState.ShellCommandsHistory; len(got) != 2 {
		t.Fatalf("space command saved: %v", got)
	}
}

func TestCustomSuggestionsSources(t *testing.T) {
	for _, preset := range []bool{false, true} {
		t.Run(map[bool]string{false: "command", true: "preset"}[preset], func(t *testing.T) {
			a, tt, _ := newTestApp(t)
			source := config.CustomCommandSuggestions{Command: "printf 'one\\ntwo\\n'"}
			if preset {
				source = config.CustomCommandSuggestions{Preset: "refs"}
			}
			a.customFlow(config.CustomCommand{Command: "true", Prompts: []config.CustomCommandPrompt{{Type: "input", Title: "Pick", Suggestions: source}}})
			tt.Frame()
			o, ok := a.topOverlay().(*promptOverlay)
			if !ok {
				t.Fatalf("overlay %T", a.topOverlay())
			}
			if preset {
				o.value = "HEAD"
			} else {
				o.value = "two"
			}
			got := o.matches()
			if len(got) == 0 || got[0] != o.value {
				t.Fatalf("matches %v", got)
			}
		})
	}
}

func TestShotsSuggestions(t *testing.T) {
	out := shotDir()
	if out == "" {
		t.Skip("SHOT_DIR not set")
	}
	a, tt, _ := newTestApp(t)
	a.promptSuggestions(a.repo.Common.Tr.BranchName, "", "", "refs", func(string) {})
	tt.Frame()
	press(tt, "tab")
	shot(t, tt, out+"/suggestions.png")
}
