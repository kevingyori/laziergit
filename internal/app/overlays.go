package app

import (
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"github.com/jesseduffield/lazygit/pkg/commands/oscommands"

	"laziergit/internal/keymap"
)

// overlay is a modal layer: menus, confirmations, prompts, errors and the
// commit editor. Keys that reach the root while one is open go to it.
type overlay interface {
	key(a *App, ch keymap.Chord)
	view(a *App, c *ui.Context, p *Palette)
}

func (a *App) pushOverlay(o overlay) { a.overlays = append(a.overlays, o) }

// wantsTextFocus reports whether the top overlay edits text, so the root
// must leave the keyboard to it.
func (a *App) wantsTextFocus() bool {
	switch a.topOverlay().(type) {
	case *promptOverlay, *commitOverlay:
		return true
	}
	return false
}

func (a *App) topOverlay() overlay {
	if len(a.overlays) == 0 {
		return nil
	}
	return a.overlays[len(a.overlays)-1]
}

func (a *App) closeOverlay(o overlay) {
	for i := len(a.overlays) - 1; i >= 0; i-- {
		if a.overlays[i] == o {
			a.overlays = append(a.overlays[:i], a.overlays[i+1:]...)
			return
		}
	}
}

// --- menu -------------------------------------------------------------------

type menuItem struct {
	label   string
	key     string // shown as a keycap
	chord   keymap.Binding
	section string
	danger  bool
	run     func()
}

type menuOverlay struct {
	title      string
	items      []menuItem
	cursor     int
	filterable bool
	filtering  bool
	filter     string
	noItemKeys bool
	list       ui.ListState
}

func (m *menuOverlay) visible() []menuItem {
	if m.filter == "" {
		return m.items
	}
	f := strings.ToLower(m.filter)
	var out []menuItem
	for _, it := range m.items {
		if strings.Contains(strings.ToLower(it.label), f) || strings.Contains(strings.ToLower(it.section), f) {
			out = append(out, it)
		}
	}
	return out
}

func (m *menuOverlay) choose(a *App, it menuItem) {
	a.closeOverlay(m)
	if it.run != nil {
		it.run()
	}
}

func (m *menuOverlay) moveBy(d int) {
	m.cursor = clamp(m.cursor+d, 0, len(m.visible())-1)
	m.list.ScrollIntoView(m.cursor)
}

func (m *menuOverlay) key(a *App, ch keymap.Chord) {
	k := a.keys
	items := m.visible()
	if m.filtering {
		switch {
		case ch.Key == ui.KeyEscape:
			m.filtering, m.filter, m.cursor = false, "", 0
		case ch.Key == ui.KeyEnter:
			if len(items) > 0 {
				m.choose(a, items[m.cursor])
			}
		case ch.Key == ui.KeyBackspace:
			if m.filter != "" {
				_, size := utf8.DecodeLastRuneInString(m.filter)
				m.filter = m.filter[:len(m.filter)-size]
				m.cursor = 0
			}
		case ch.Key == ui.KeyDown:
			m.moveBy(1)
		case ch.Key == ui.KeyUp:
			m.moveBy(-1)
		case ch.Key == ui.KeySpace:
			m.filter += " "
		case ch.Rune != 0 && ch.Mods == 0:
			m.filter += string(ch.Rune)
			m.cursor = 0
		}
		return
	}
	switch {
	case k.ret.Matches(ch):
		a.closeOverlay(m)
	case k.next.Matches(ch):
		m.moveBy(1)
	case k.prev.Matches(ch):
		m.moveBy(-1)
	case k.confirmMenu.Matches(ch) || k.selectKey.Matches(ch):
		if len(items) > 0 {
			m.choose(a, items[m.cursor])
		}
	case m.filterable && ch == keymap.Chord{Rune: '/'}:
		m.filtering = true
	default:
		if !m.noItemKeys {
			for _, it := range items {
				if it.chord.Matches(ch) {
					m.choose(a, it)
					return
				}
			}
		}
	}
}

func (m *menuOverlay) view(a *App, c *ui.Context, p *Palette) {
	items := m.visible()
	card(c, p, 560, func() {
		ui.Row(c).PaddingX(16).PaddingY(12).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, m.title).FontWeight(600)
			ui.Box(c).Grow(1)
			if m.filterable {
				if m.filtering || m.filter != "" {
					ui.RichText(c,
						ui.Span{Text: "/ ", Color: p.Faint, Font: monoFont},
						ui.Span{Text: m.filter, Color: p.Text, Font: monoFont},
						ui.Span{Text: "▏", Color: p.Accent},
					).FontSize(12)
				} else {
					keycap(c, p, "/")
					ui.Text(c, "Filter").FontSize(12).TextColor(p.Muted)
				}
			}
		})
		ui.Box(c).Height(1).FillWidth().Background(p.Hairline)
		h := float32(min(max(len(items), 1), 16)*26 + 12)
		ui.List(c, &m.list, len(items), func(i int) {
			it := items[i]
			showSection := it.section != "" && (i == 0 || items[i-1].section != it.section)
			ui.Column(c).Children(func() {
				if showSection {
					ui.Text(c, strings.ToUpper(it.section)).FontSize(sizeCaption).FontWeight(650).
						LetterSpacing(0.7).TextColor(p.Faint).Padding(10, 16, 4, 16)
				}
				row := ui.Row(c).Height(26).PaddingX(10).MarginX(6).Gap(10).AlignItems(ui.Center).Radius(5)
				switch {
				case i == m.cursor:
					row.Background(p.AccentSoft)
				case row.Hovered():
					row.Background(p.Hover)
				}
				if row.Clicked() {
					m.choose(a, it)
				}
				row.Children(func() {
					color := p.Text
					if it.danger {
						color = p.Conflict
					}
					ui.Text(c, it.label).TextColor(color).SingleLine().Grow(1).Shrink(1)
					if it.key != "" {
						keycap(c, p, it.key)
					}
				})
			})
		}).Height(h).PaddingY(6)
		if len(items) == 0 {
			ui.Text(c, "No matches").TextColor(p.Muted).Padding(0, 16, 12, 16)
		}
	})
}

// --- confirm ----------------------------------------------------------------

type confirmOverlay struct {
	title, body string
	danger      bool
	yesLabel    string
	onYes       func()
}

func (o *confirmOverlay) key(a *App, ch keymap.Chord) {
	switch {
	case a.keys.ret.Matches(ch):
		a.closeOverlay(o)
	case a.keys.confirmMenu.Matches(ch) || ch == keymap.Chord{Rune: 'y'}:
		a.closeOverlay(o)
		o.onYes()
	}
}

func (o *confirmOverlay) view(a *App, c *ui.Context, p *Palette) {
	card(c, p, 480, func() {
		ui.Column(c).Padding(18, 18, 6, 18).Gap(8).Children(func() {
			ui.Text(c, o.title).FontWeight(600).FontSize(15)
			ui.Text(c, o.body).TextColor(p.Muted).LineHeight(1.45)
		})
		yes := o.yesLabel
		if yes == "" {
			yes = "Confirm"
		}
		footer(c, p, func() {
			if button(c, p, "Cancel", a.keys.ret.Label(), false, false) {
				a.closeOverlay(o)
			}
			if button(c, p, yes, a.keys.confirmMenu.Label(), true, o.danger) {
				a.closeOverlay(o)
				o.onYes()
			}
		})
	})
}

// --- error ------------------------------------------------------------------

type errorOverlay struct {
	title, body string
	neutral     bool
}

func (o *errorOverlay) key(a *App, ch keymap.Chord) {
	if a.keys.ret.Matches(ch) || a.keys.confirmMenu.Matches(ch) {
		a.closeOverlay(o)
	}
}

func (o *errorOverlay) view(a *App, c *ui.Context, p *Palette) {
	card(c, p, 620, func() {
		ui.Row(c).Padding(16, 18, 8, 18).Gap(8).AlignItems(ui.Center).Children(func() {
			if !o.neutral {
				ui.Box(c).Size(8, 8).Radius(4).Background(p.Conflict)
			}
			ui.Text(c, o.title).FontWeight(600).FontSize(15)
		})
		ui.Scroll(c).MaxHeight(320).Padding(0, 18, 8, 18).Children(func() {
			ui.Text(c, strings.TrimSpace(o.body)).Font(monoFont).FontSize(12).TextColor(p.Muted).
				LineHeight(1.5).Selectable()
		})
		footer(c, p, func() {
			if button(c, p, "Close", a.keys.ret.Label(), true, false) {
				a.closeOverlay(o)
			}
		})
	})
}

func (a *App) showError(title string, err error) {
	a.setStatus(title+" failed", true)
	a.pushOverlay(&errorOverlay{title: title + " failed", body: err.Error()})
}

// --- prompt -----------------------------------------------------------------

type promptOverlay struct {
	title, hint string
	value       string
	mask        bool
	onSubmit    func(string)
	onCancel    func()
}

func (o *promptOverlay) cancel(a *App) {
	a.closeOverlay(o)
	if o.onCancel != nil {
		o.onCancel()
	}
}

func (o *promptOverlay) submit(a *App, v string) {
	a.closeOverlay(o)
	o.onSubmit(v)
}

func (o *promptOverlay) key(a *App, ch keymap.Chord) {
	if a.keys.ret.Matches(ch) {
		o.cancel(a)
	}
}

func (o *promptOverlay) view(a *App, c *ui.Context, p *Palette) {
	card(c, p, 520, func() {
		ui.Column(c).Padding(16, 18, 16, 18).Gap(10).Children(func() {
			ui.Text(c, o.title).FontWeight(600)
			in := ui.TextInput(c.Key("prompt"), &o.value).AutoFocus().Label(o.title).
				OnTextKey(func(k ui.TextKey) bool {
					switch {
					case k.Key == ui.KeyEnter && k.Mods == 0:
						o.submit(a, k.Text())
						return true
					case k.Key == ui.KeyEscape:
						o.cancel(a)
						return true
					}
					return false
				})
			if o.mask {
				in.Password()
			}
			if o.hint != "" {
				ui.Text(c, o.hint).FontSize(12).TextColor(p.Muted)
			}
		})
	})
}

func (a *App) prompt(title, initial, hint string, onSubmit func(string)) {
	a.pushOverlay(&promptOverlay{title: title, value: initial, hint: hint, onSubmit: onSubmit})
}

func (a *App) confirm(title, body string, danger bool, onYes func()) {
	a.pushOverlay(&confirmOverlay{title: title, body: body, danger: danger, onYes: onYes})
}

// promptCredential answers a git credential request (lazygit's pty
// detection of "Username for", "Password for", passphrases, ...).
func (a *App) promptCredential(kind oscommands.CredentialType, ch chan<- string) {
	title, mask := "Password", true
	switch kind {
	case oscommands.Username:
		title, mask = "Username", false
	case oscommands.Passphrase:
		title = "SSH key passphrase"
	case oscommands.PIN:
		title = "PIN"
	case oscommands.Token:
		title = "Token"
	}
	a.pushOverlay(&promptOverlay{
		title: title, mask: mask, hint: "Requested by git" + busySuffix(a.busy),
		// The answer is typed into git's pty, so it needs the Enter, as in
		// lazygit's credentials helper.
		onSubmit: func(v string) { ch <- v + "\n" },
		onCancel: func() { ch <- "\n" },
	})
}

func busySuffix(s string) string {
	if s == "" {
		return ""
	}
	return " while " + strings.ToLower(strings.TrimSuffix(s, "…"))
}

// --- commit editor ----------------------------------------------------------

type commitDraft struct{ summary, description string }

type commitOverlay struct {
	title    string
	draft    *commitDraft
	onSubmit func(summary, description string)
	summaryH ui.Handle
	descH    ui.Handle
}

func (o *commitOverlay) key(a *App, ch keymap.Chord) {
	if a.keys.ret.Matches(ch) {
		a.closeOverlay(o)
	}
}

func (o *commitOverlay) submit(a *App, summary, description string) {
	if strings.TrimSpace(summary) == "" {
		return
	}
	a.closeOverlay(o)
	o.onSubmit(strings.TrimSpace(summary), strings.TrimRight(description, "\n "))
}

func (o *commitOverlay) view(a *App, c *ui.Context, p *Palette) {
	k := a.keys
	asChord := func(t ui.TextKey) keymap.Chord {
		ch, _ := keymap.FromEvent(ui.InputEvent{Kind: ui.InputKeyDown, Key: t.Key, Mods: t.Mods})
		return ch
	}
	card(c, p, 640, func() {
		ui.Column(c).Padding(16, 18, 14, 18).Gap(10).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Text(c, o.title).FontWeight(600)
				ui.Box(c).Grow(1)
				n := utf8.RuneCountInString(o.draft.summary)
				col := p.Faint
				switch {
				case n > 72:
					col = p.Conflict
				case n > 50:
					col = p.Unstaged
				}
				ui.Textf(c, "%d", n).FontSize(12).TextColor(col).FontFeatures("tnum")
			})
			ui.TextInput(c.Key("summary"), &o.draft.summary).AutoFocus().Placeholder("Summary").
				Label("Summary").Bind(&o.summaryH).
				OnTextKey(func(t ui.TextKey) bool {
					switch {
					case t.Key == ui.KeyEscape:
						a.closeOverlay(o)
						return true
					case t.Key == ui.KeyTab:
						o.descH.Focus()
						return true
					case t.Key == ui.KeyEnter && (t.Mods&ui.Cmd != 0 || k.confirmInEditor.Matches(asChord(t))):
						o.submit(a, t.Text(), o.draft.description)
						return true
					case k.submitEditor.Matches(asChord(t)):
						o.submit(a, t.Text(), o.draft.description)
						return true
					}
					return false
				})
			ui.TextArea(c.Key("description"), &o.draft.description).Lines(5, 14).
				Placeholder("Description").Label("Description").Bind(&o.descH).
				OnTextKey(func(t ui.TextKey) bool {
					switch {
					case t.Key == ui.KeyEscape:
						a.closeOverlay(o)
						return true
					case t.Key == ui.KeyTab:
						o.summaryH.Focus()
						return true
					case t.Mods&ui.Cmd != 0 && t.Key == ui.KeyEnter, k.confirmInEditor.Matches(asChord(t)):
						o.submit(a, o.draft.summary, t.Text())
						return true
					}
					return false
				})
		})
		footer(c, p, func() {
			ui.Text(c, "tab switches fields").FontSize(12).TextColor(p.Faint)
			ui.Box(c).Grow(1)
			if button(c, p, "Cancel", k.ret.Label(), false, false) {
				a.closeOverlay(o)
			}
			if button(c, p, "Commit", k.submitEditor.Label(), true, false) {
				o.submit(a, o.draft.summary, o.draft.description)
			}
		})
	})
}

// --- shared chrome ----------------------------------------------------------

// card is the overlay surface, centred near the top of the window over a
// dimmed backdrop; clicking the backdrop dismisses it.
func card(c *ui.Context, p *Palette, width float32, body func()) {
	e := ui.Column(c).Width(width).MaxWidthPercent(92).Background(p.Panel).Radius(12).
		Border(1, p.Hairline).Shadow(0, 18, 48, 0, ui.Hex("#00000055")).Clip()
	_ = e.Clicked() // a click inside the card is not a click on the backdrop
	e.Children(body)
}

func footer(c *ui.Context, p *Palette, body func()) {
	ui.Row(c).Padding(10, 14, 12, 14).Gap(8).AlignItems(ui.Center).Justify(ui.End).Children(body)
}

func button(c *ui.Context, p *Palette, label, key string, primary, danger bool) bool {
	b := ui.Row(c).Gap(8).PaddingX(12).Height(30).AlignItems(ui.Center).Radius(7).Cursor(ui.CursorPointer)
	bg, fg := p.Idle, p.Text
	switch {
	case danger && primary:
		bg, fg = p.Conflict, ui.Hex("#ffffff")
	case primary:
		bg, fg = p.Accent, ui.Hex("#ffffff")
	}
	if b.Hovered() {
		bg = bg.Mix(fg, 0.08)
	}
	b.Background(bg).Children(func() {
		ui.Text(c, label).TextColor(fg).FontWeight(550)
		if key != "" {
			ui.Text(c, key).Font(monoFont).FontSize(11).TextColor(fg.Alpha(0.7))
		}
	})
	return b.Clicked()
}

func (a *App) overlayLayer(c *ui.Context, p *Palette) {
	ov := a.topOverlay()
	if ov == nil {
		return
	}
	ui.Overlay(c, func() {
		back := ui.Column(c).Absolute().Left(0).Top(0).Fill().Background(p.Dim).
			AlignItems(ui.Center).Padding(88, 0, 0, 0)
		if back.Clicked() {
			if po, ok := ov.(*promptOverlay); ok {
				po.cancel(a)
			} else {
				a.closeOverlay(ov)
			}
		}
		back.Children(func() { ov.view(a, c.Key(ov), p) })
	})
}
