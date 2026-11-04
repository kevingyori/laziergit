// Package keymap turns lazygit's keybinding strings ("q", "<c-r>", "<space>",
// "<pgdown>") into chords and matches MyGo input events against them.
//
// lazygit binds characters, not physical keys: "?" and "<" are whatever the
// layout types. So unmodified printable keys are matched on the InputText
// event (the character typed, shift folded in), while named keys and anything
// with Ctrl/Alt/Super are matched on InputKeyDown.
package keymap

import (
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"github.com/jesseduffield/lazygit/pkg/config"
	"github.com/jesseduffield/lazygit/pkg/gocui"
)

// Chord is one key with modifiers. Exactly one of Key and Rune is set.
type Chord struct {
	Mods ui.Modifiers
	Key  ui.Key // a named key (enter, esc, arrows, ...)
	Rune rune   // a character; lowercase when Ctrl/Alt are held
}

func (c Chord) IsZero() bool { return c.Key == 0 && c.Rune == 0 }

// Binding is the set of chords bound to one action (lazygit allows alternates).
type Binding []Chord

// Parse converts a lazygit Keybinding into chords. Unparseable entries are
// dropped; lazygit itself validates config at load time.
func Parse(kb config.Keybinding) Binding {
	var out Binding
	for _, label := range kb {
		if c, ok := ParseLabel(label); ok {
			out = append(out, c)
		}
	}
	return out
}

// ParseLabel parses a single lazygit key label.
func ParseLabel(label string) (Chord, bool) {
	k, ok := config.KeyFromLabel(label)
	if !ok || !k.IsSet() {
		return Chord{}, false
	}
	var c Chord
	m := k.Mod()
	if m&gocui.ModShift != 0 {
		c.Mods |= ui.Shift
	}
	if m&gocui.ModCtrl != 0 {
		c.Mods |= ui.Ctrl
	}
	if m&gocui.ModAlt != 0 || m&gocui.ModMeta != 0 {
		// Terminals report Option as Alt or Meta depending on the emulator;
		// a desktop app only ever sees Alt.
		c.Mods |= ui.Alt
	}
	if s := k.Str(); s != "" {
		r, _ := utf8.DecodeRuneInString(s)
		if r == ' ' {
			c.Key = ui.KeySpace
			return c, true
		}
		c.Rune = r
		return c, true
	}
	key, ok := namedKeys[k.KeyName()]
	if !ok {
		return Chord{}, false
	}
	if k.KeyName() == gocui.KeyBacktab {
		c.Mods |= ui.Shift
	}
	c.Key = key
	return c, true
}

var namedKeys = map[gocui.KeyName]ui.Key{
	gocui.KeyF1: ui.KeyF1, gocui.KeyF2: ui.KeyF2, gocui.KeyF3: ui.KeyF3, gocui.KeyF4: ui.KeyF4,
	gocui.KeyF5: ui.KeyF5, gocui.KeyF6: ui.KeyF6, gocui.KeyF7: ui.KeyF7, gocui.KeyF8: ui.KeyF8,
	gocui.KeyF9: ui.KeyF9, gocui.KeyF10: ui.KeyF10, gocui.KeyF11: ui.KeyF11, gocui.KeyF12: ui.KeyF12,
	gocui.KeyInsert: ui.KeyInsert, gocui.KeyDelete: ui.KeyDelete,
	gocui.KeyHome: ui.KeyHome, gocui.KeyEnd: ui.KeyEnd,
	gocui.KeyPgup: ui.KeyPageUp, gocui.KeyPgdn: ui.KeyPageDown,
	gocui.KeyArrowUp: ui.KeyUp, gocui.KeyArrowDown: ui.KeyDown,
	gocui.KeyArrowLeft: ui.KeyLeft, gocui.KeyArrowRight: ui.KeyRight,
	gocui.KeyTab: ui.KeyTab, gocui.KeyBacktab: ui.KeyTab,
	gocui.KeyEnter: ui.KeyEnter, gocui.KeyEsc: ui.KeyEscape,
	gocui.KeyBackspace: ui.KeyBackspace,
}

// FromEvent converts an input event into the chord lazygit would see, and
// reports whether the event is one to dispatch at all. Plain printable
// KeyDowns return false: their InputText follows and carries the character.
func FromEvent(ev ui.InputEvent) (Chord, bool) {
	mods := ev.Mods &^ ui.Super // Cmd is left to the app menu / system
	switch ev.Kind {
	case ui.InputKeyDown:
		if ev.Mods&ui.Super != 0 {
			return Chord{}, false
		}
		if r, ok := keyRune(ev.Key); ok {
			if mods&(ui.Ctrl|ui.Alt) == 0 {
				return Chord{}, false // wait for InputText
			}
			return Chord{Mods: mods, Rune: r}, true
		}
		if ev.Key == ui.KeySpace || isNamed(ev.Key) {
			return Chord{Mods: mods, Key: ev.Key}, true
		}
		return Chord{}, false
	case ui.InputText:
		r, size := utf8.DecodeRuneInString(ev.Text)
		if size == 0 || size != len(ev.Text) || r == ' ' || r < 0x20 {
			return Chord{}, false
		}
		return Chord{Rune: r}, true
	}
	return Chord{}, false
}

// Matches reports whether the binding contains the chord.
func (b Binding) Matches(c Chord) bool {
	for _, x := range b {
		if x == c {
			return true
		}
	}
	return false
}

// Label renders the binding's first chord the way lazygit's UI does.
func (b Binding) Label() string {
	if len(b) == 0 {
		return ""
	}
	return b[0].Label()
}

// LabelAll renders every chord of the binding, space separated.
func (b Binding) LabelAll() string {
	labels := make([]string, len(b))
	for i, c := range b {
		labels[i] = c.Label()
	}
	return strings.Join(labels, " ")
}

func (c Chord) Label() string {
	var sb strings.Builder
	if c.Mods&ui.Ctrl != 0 {
		sb.WriteString("ctrl+")
	}
	if c.Mods&ui.Alt != 0 {
		sb.WriteString("alt+")
	}
	if c.Mods&ui.Shift != 0 && c.Rune == 0 {
		sb.WriteString("shift+")
	}
	if c.Rune != 0 {
		sb.WriteRune(c.Rune)
		return sb.String()
	}
	for name, k := range labelNames {
		if k == c.Key {
			sb.WriteString(name)
			return sb.String()
		}
	}
	return sb.String() + "?"
}

var labelNames = map[string]ui.Key{
	"space": ui.KeySpace, "enter": ui.KeyEnter, "esc": ui.KeyEscape, "tab": ui.KeyTab,
	"↑": ui.KeyUp, "↓": ui.KeyDown, "←": ui.KeyLeft, "→": ui.KeyRight,
	"pgup": ui.KeyPageUp, "pgdn": ui.KeyPageDown, "home": ui.KeyHome, "end": ui.KeyEnd,
	"del": ui.KeyDelete, "bksp": ui.KeyBackspace, "ins": ui.KeyInsert,
	"f1": ui.KeyF1, "f2": ui.KeyF2, "f3": ui.KeyF3, "f4": ui.KeyF4, "f5": ui.KeyF5, "f6": ui.KeyF6,
	"f7": ui.KeyF7, "f8": ui.KeyF8, "f9": ui.KeyF9, "f10": ui.KeyF10, "f11": ui.KeyF11, "f12": ui.KeyF12,
}

func isNamed(k ui.Key) bool {
	for _, v := range namedKeys {
		if v == k {
			return true
		}
	}
	return false
}

// keyRune maps a layout key to the unshifted character it stands for, used
// for Ctrl/Alt chords where the OS gives no (or a garbled) InputText.
func keyRune(k ui.Key) (rune, bool) {
	switch {
	case k >= ui.KeyA && k <= ui.KeyZ:
		return 'a' + rune(k-ui.KeyA), true
	case k >= ui.Key0 && k <= ui.Key9:
		return '0' + rune(k-ui.Key0), true
	}
	switch k {
	case ui.KeyMinus:
		return '-', true
	case ui.KeyEqual:
		return '=', true
	case ui.KeyComma:
		return ',', true
	case ui.KeyPeriod:
		return '.', true
	case ui.KeySlash:
		return '/', true
	case ui.KeySemicolon:
		return ';', true
	case ui.KeyQuote:
		return '\'', true
	case ui.KeyBracketLeft:
		return '[', true
	case ui.KeyBracketRight:
		return ']', true
	case ui.KeyBackslash:
		return '\\', true
	case ui.KeyBackquote:
		return '`', true
	}
	return 0, false
}
