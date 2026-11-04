package keymap

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/jesseduffield/lazygit/pkg/config"
)

func TestParseLabel(t *testing.T) {
	cases := map[string]Chord{
		"q":         {Rune: 'q'},
		"?":         {Rune: '?'},
		"<":         {Rune: '<'},
		"P":         {Rune: 'P'},
		"<space>":   {Key: ui.KeySpace},
		"<enter>":   {Key: ui.KeyEnter},
		"<esc>":     {Key: ui.KeyEscape},
		"<c-r>":     {Mods: ui.Ctrl, Rune: 'r'},
		"<a-up>":    {Mods: ui.Alt, Key: ui.KeyUp},
		"<pgdown>":  {Key: ui.KeyPageDown},
		"<backtab>": {Mods: ui.Shift, Key: ui.KeyTab},
	}
	for label, want := range cases {
		got, ok := ParseLabel(label)
		if !ok || got != want {
			t.Errorf("%q: got %+v ok=%v, want %+v", label, got, ok, want)
		}
	}
}

func TestFromEvent(t *testing.T) {
	// Plain letters wait for their text.
	if _, ok := FromEvent(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyC}); ok {
		t.Error("plain keydown should defer to InputText")
	}
	if c, ok := FromEvent(ui.InputEvent{Kind: ui.InputText, Text: "C"}); !ok || c != (Chord{Rune: 'C'}) {
		t.Errorf("text C: %+v %v", c, ok)
	}
	if c, ok := FromEvent(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyR, Mods: ui.Ctrl}); !ok || c != (Chord{Mods: ui.Ctrl, Rune: 'r'}) {
		t.Errorf("ctrl-r: %+v %v", c, ok)
	}
	if c, ok := FromEvent(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeySpace}); !ok || c != (Chord{Key: ui.KeySpace}) {
		t.Errorf("space: %+v %v", c, ok)
	}
	if _, ok := FromEvent(ui.InputEvent{Kind: ui.InputText, Text: " "}); ok {
		t.Error("space text must not double-fire")
	}
	if _, ok := FromEvent(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyC, Mods: ui.Super}); ok {
		t.Error("cmd chords belong to the system")
	}
}

func TestDefaultConfigParses(t *testing.T) {
	cfg := config.GetDefaultConfig()
	cfg.Keybinding.MergeLegacyAltKeybindings()
	u := cfg.Keybinding.Universal
	for name, kb := range map[string]config.Keybinding{
		"quit": u.Quit, "prev": u.PrevItem, "next": u.NextItem, "select": u.Select,
		"prevBlock": u.PrevBlock, "nextBlock": u.NextBlock, "refresh": u.Refresh,
	} {
		if len(Parse(kb)) != len(kb) || len(kb) == 0 {
			t.Errorf("%s: %v parsed to %v", name, kb, Parse(kb))
		}
	}
	if !Parse(u.NextItem).Matches(Chord{Rune: 'j'}) {
		t.Errorf("j should be next item after legacy merge: %v", u.NextItem)
	}
}
