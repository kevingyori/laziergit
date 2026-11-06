package app

import (
	"fmt"
	"slices"
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
	if o, ok := a.topOverlay().(*promptOverlay); ok {
		return !o.suggestionsFocused
	}
	switch a.topOverlay().(type) {
	case *promptOverlay, *commitOverlay, *searchOverlay:
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
	detail  string // second column (lazygit's LabelColumns[1]), e.g. the git command
	key     string // shown as a keycap
	widget  string // lazygit's MenuWidget: "( )", "(•)", "[ ]", "[✓]" before the label
	chord   keymap.Binding
	section string
	danger  bool
	// disabled says why the item can't run now; it stays listed (lazygit's
	// DisabledReason) so nothing is hidden.
	disabled string
	tooltip  string // lazygit's MenuItem.Tooltip
	// columns follow the label (lazygit's LabelColumns[1:]), lined up across
	// the items.
	columns []string
	// filterText is matched by the filter instead of the label (lazygit's
	// FilterColumns).
	filterText string
	// keyLabels are all the item's keys, for a "@" filter (lazygit's
	// AllowFilteringKeybindings).
	keyLabels []string
	run       func()
}

func (it menuItem) shownLabel() string {
	if it.widget != "" {
		return it.widget + " " + it.label
	}
	return it.label
}

type menuOverlay struct {
	itemKeysFirst bool
	body          string
	title         string
	items         []menuItem
	cursor        int
	filterable    bool
	// filterAsYouType: typing filters straight away (lazygit's
	// FilterAsYouType); navigation stays on the arrow keys.
	filterAsYouType bool
	filtering       bool
	filter          string
	noItemKeys      bool
	allowKeyFilter  bool // a filter starting with "@" matches the items' keys
	list            ui.ListState
}

func (m *menuOverlay) visible() []menuItem {
	if m.filter == "" {
		return m.items
	}
	f := strings.ToLower(m.filter)
	var out []menuItem
	if m.allowKeyFilter && strings.HasPrefix(f, "@") {
		for _, it := range m.items {
			if slices.ContainsFunc(it.keyLabels, func(l string) bool { return strings.Contains(strings.ToLower(l), f[1:]) }) {
				out = append(out, it)
			}
		}
		return out
	}
	for _, it := range m.items {
		text := it.label
		if it.filterText != "" {
			text = it.filterText
		}
		if strings.Contains(strings.ToLower(text), f) || strings.Contains(strings.ToLower(it.section), f) {
			out = append(out, it)
		}
	}
	return out
}

func (m *menuOverlay) choose(a *App, it menuItem) {
	if it.disabled != "" {
		a.setStatus(a.repo.Common.Tr.DisabledMenuItemPrefix+it.disabled, true)
		return
	}
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
	if m.filterAsYouType {
		m.keyAsYouType(a, ch)
		return
	}
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
	if m.filterAsYouType && ch.Rune != 0 && ch.Mods == 0 && ch.Key != ui.KeySpace {
		m.filtering, m.filter, m.cursor = true, string(ch.Rune), 0
		return
	}
	if m.itemKeysFirst {
		for _, it := range items {
			if it.chord.Matches(ch) {
				m.choose(a, it)
				return
			}
		}
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

// subtitle is lazygit's menu subtitle: until the filter row of a
// filter-as-you-type menu appears, the menu says typing filters it.
func (m *menuOverlay) subtitle(a *App) string {
	if m.filterAsYouType && !m.filtering && m.filter == "" {
		return a.repo.Common.Tr.MenuFilterHint
	}
	return ""
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
				} else if sub := m.subtitle(a); sub != "" {
					ui.Text(c, sub).FontSize(12).TextColor(p.Muted)
				} else {
					keycap(c, p, "/")
					ui.Text(c, "Filter").FontSize(12).TextColor(p.Muted)
				}
			}
		})
		if m.body != "" {
			ui.Text(c, m.body).Padding(10, 16, 10, 16).TextColor(p.Muted)
		}
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
					switch {
					case it.disabled != "":
						color = p.Faint
					case it.danger:
						color = p.Conflict
					}
					if len(it.columns) > 0 {
						// Columns line up across the items: fixed widths, the
						// last one takes the rest.
						ui.Text(c, it.shownLabel()).TextColor(color).SingleLine().Width(170)
						for j, col := range it.columns {
							t := ui.Text(c, col).Font(monoFont).FontSize(12).TextColor(p.Muted).SingleLine()
							if j < len(it.columns)-1 {
								t.Width(170)
							} else {
								t.Shrink(1)
							}
						}
					} else {
						ui.Text(c, it.shownLabel()).TextColor(color).SingleLine().Shrink(1)
					}
					if it.detail != "" {
						ui.Text(c, it.detail).Font(monoFont).FontSize(12).TextColor(p.Conflict).SingleLine().Shrink(2)
					}
					ui.Box(c).Grow(1)
					if it.disabled != "" {
						ui.Text(c, it.disabled).FontSize(12).TextColor(p.Faint).SingleLine().Shrink(2)
					}
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
	onClose     func() // lazygit's HandleClose
	alert       bool   // lazygit's Alert: output to read, no choice to make
}

func (o *confirmOverlay) key(a *App, ch keymap.Chord) {
	switch {
	case a.keys.ret.Matches(ch):
		a.closeOverlay(o)
		if o.onClose != nil {
			o.onClose()
		}
	case keymap.Parse(a.repo.Config.Keybinding.Universal.Confirm).Matches(ch): // ConfirmationController
		a.closeOverlay(o)
		o.onYes()
	case keymap.Parse(a.repo.Config.Keybinding.Universal.CopyToClipboard).Matches(ch):
		// lazygit's confirmation controller: copy the message and toast.
		if err := a.copyText(o.body); err != nil {
			a.showError(a.repo.Common.Tr.Actions.CopyToClipboard, err)
			return
		}
		a.setStatus(a.repo.Common.Tr.MessageCopiedToClipboard, false)
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
			if !o.alert && button(c, p, "Cancel", a.keys.ret.Label(), false, false) {
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
	suggestionList     ui.ListState
	title, hint        string
	value              string
	mask               bool
	suggestions        func(string) []string
	suggestionsFocused bool
	suggestionCursor   int
	editSuggestion     bool
	deleteSuggestion   func(string)
	onSubmit           func(string)
	onCancel           func()
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
	if !o.suggestionsFocused {
		if a.keys.ret.Matches(ch) {
			o.cancel(a)
		}
		return
	}
	items := o.matches()
	k := a.repo.Config.Keybinding.Universal
	switch {
	case a.keys.ret.Matches(ch):
		o.cancel(a)
	case keymap.Parse(k.TogglePanel).Matches(ch):
		o.suggestionsFocused = false
	case a.keys.next.Matches(ch):
		o.suggestionCursor = clamp(o.suggestionCursor+1, 0, len(items)-1)
		o.suggestionList.ScrollIntoView(o.suggestionCursor)
	case a.keys.prev.Matches(ch):
		o.suggestionCursor = clamp(o.suggestionCursor-1, 0, len(items)-1)
		o.suggestionList.ScrollIntoView(o.suggestionCursor)
	case len(items) > 0 && keymap.Parse(k.ConfirmSuggestion).Matches(ch):
		o.submit(a, items[o.suggestionCursor])
	case len(items) > 0 && o.editSuggestion && keymap.Parse(k.Edit).Matches(ch):
		o.value = items[o.suggestionCursor]
		o.suggestionsFocused = false
	case len(items) > 0 && o.deleteSuggestion != nil && keymap.Parse(k.Remove).Matches(ch):
		o.deleteSuggestion(items[o.suggestionCursor])
		o.suggestionCursor = 0
	}
}

func (o *promptOverlay) view(a *App, c *ui.Context, p *Palette) {
	card(c, p, 520, func() {
		ui.Column(c).Padding(16, 18, 16, 18).Gap(10).Children(func() {
			ui.Text(c, o.title).FontWeight(600)
			in := ui.TextInput(c.Key("prompt"), &o.value).Label(o.title).
				OnTextKey(func(k ui.TextKey) bool {
					switch {
					case k.Key == ui.KeyEnter && k.Mods == 0:
						o.submit(a, k.Text())
						return true
					case k.Key == ui.KeyEscape:
						o.cancel(a)
						return true
					case keymap.Parse(a.repo.Config.Keybinding.Universal.TogglePanel).Matches(keymap.Chord{Key: k.Key, Mods: k.Mods}):
						if len(o.matches()) > 0 {
							o.suggestionsFocused = true
							o.suggestionCursor = 0
						}
						return true
					case isClearKey(k):
						o.value = ""
						return true
					}
					return false
				})
			if !o.suggestionsFocused {
				in.AutoFocus()
			}
			if o.mask {
				in.Password()
			}
			if o.suggestions != nil {
				items := o.matches()
				ui.List(c, &o.suggestionList, len(items), func(i int) {
					item := items[i]
					row := ui.Row(c).Height(26).PaddingX(10).Radius(5).AlignItems(ui.Center)
					if o.suggestionsFocused && i == o.suggestionCursor {
						row.Background(p.AccentSoft)
					} else if row.Hovered() {
						row.Background(p.Hover)
					}
					if row.Clicked() {
						o.suggestionCursor = i
						o.suggestionsFocused = true
					}
					row.Children(func() { ui.Text(c, item).FontSize(sizeBody) })
				}).Height(float32(min(len(items), 8) * 26))

				ui.Text(c, fmt.Sprintf(a.repo.Common.Tr.SuggestionsTitle, a.repo.Config.Keybinding.Universal.TogglePanel)).FontSize(12).TextColor(p.Muted)
				if o.suggestionsFocused && o.deleteSuggestion != nil {
					k := a.repo.Config.Keybinding.Universal
					ui.Text(c, fmt.Sprintf(a.repo.Common.Tr.SuggestionsSubtitle, k.Remove, k.Edit)).FontSize(12).TextColor(p.Muted)
				}
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

// alert is lazygit's Alert: a titled popup with text and nothing to decide.
func (a *App) alert(title, body string) {
	a.pushOverlay(&confirmOverlay{title: title, body: body, yesLabel: "OK", alert: true, onYes: func() {}})
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
	title            string
	descriptionTitle string
	summaryTitle     string
	draft            *commitDraft
	onSubmit         func(summary, description string)
	onSwitchToEditor func(path string)
	refocus          *ui.Handle // the field to focus again after a popup on top closes
	hist             int        // lazygit's commit history index + 1; 0 is NoCommitIndex
	historyMessage   string     // the message typed before walking the history
	gen              int        // bumped when the text is replaced, remounting the fields
	preserve         bool       // lazygit's PreserveMessage: closing writes LAZYGIT_PENDING_COMMIT
	initial          string     // the message the panel opened with (GetInitialMessage)
	summaryH         ui.Handle
	descH            ui.Handle
}

func (o *commitOverlay) key(a *App, ch keymap.Chord) {
	if a.keys.ret.Matches(ch) {
		a.closeOverlay(o)
		a.preserveCommitMessage(o)
	}
}

func (o *commitOverlay) submit(a *App, summary, description string) {
	if strings.TrimSpace(summary) == "" {
		return
	}
	a.closeOverlay(o)
	a.preserveCommitMessage(o) // CloseCommitMessagePanel
	if description == o.draft.description {
		description = o.wrappedDescription(a)
	}
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
			focusDesc := o.refocus == &o.descH && a.topOverlay() == overlay(o)
			if a.topOverlay() == overlay(o) {
				o.refocus = nil
			}
			sum := ui.TextInput(c.Key(fmt.Sprintf("summary%d", o.gen)), &o.draft.summary).Placeholder("Summary").
				Label("Summary").Bind(&o.summaryH).
				OnTextKey(func(t ui.TextKey) bool {
					switch {
					case isClearKey(t):
						o.draft.summary = ""
						return true
					case t.Key == ui.KeyEscape:
						a.closeOverlay(o)
						a.preserveCommitMessage(o)
						return true
					case t.Key == ui.KeyTab:
						if o.descriptionTitle != "" {
							o.summaryTitle, o.title = o.title, o.descriptionTitle
						}
						o.descH.Focus()
						return true
					case t.Key == ui.KeyEnter && (t.Mods&ui.Cmd != 0 || k.confirmInEditor.Matches(asChord(t))):
						o.submit(a, t.Text(), o.draft.description)
						return true
					case k.submitEditor.Matches(asChord(t)):
						o.submit(a, t.Text(), o.draft.description)
						return true
					case k.commitMenu.Matches(asChord(t)):
						a.commitMenu(o, &o.summaryH)
						return true
					case k.prev.Matches(asChord(t)):
						a.commitHistory(o, 1)
						return true
					case k.next.Matches(asChord(t)):
						a.commitHistory(o, -1)
						return true
					}
					return false
				})
			if !focusDesc {
				sum.AutoFocus()
			}
			desc := ui.TextArea(c.Key(fmt.Sprintf("description%d", o.gen)), &o.draft.description).Lines(5, 14).
				Placeholder("Description").Label("Description").Bind(&o.descH).
				OnTextKey(func(t ui.TextKey) bool {
					switch {
					case isClearKey(t):
						// ctrl+u deletes one line, like lazygit's text areas.
						d := strings.TrimRight(o.draft.description, "\n")
						if i := strings.LastIndex(d, "\n"); i >= 0 {
							o.draft.description = d[:i+1]
						} else {
							o.draft.description = ""
						}
						return true
					case t.Key == ui.KeyEscape:
						a.closeOverlay(o)
						a.preserveCommitMessage(o)
						return true
					case t.Key == ui.KeyTab:
						if o.summaryTitle != "" {
							o.title = o.summaryTitle
						}
						o.summaryH.Focus()
						return true
					case t.Mods&ui.Cmd != 0 && t.Key == ui.KeyEnter, k.confirmInEditor.Matches(asChord(t)):
						o.submit(a, o.draft.summary, t.Text())
						return true
					case k.commitMenu.Matches(asChord(t)):
						a.commitMenu(o, &o.descH)
						return true
					}
					return false
				})
			if focusDesc {
				desc.AutoFocus()
			}
		})
		footer(c, p, func() {
			ui.Text(c, "tab switches fields").FontSize(12).TextColor(p.Faint)
			ui.Box(c).Grow(1)
			if button(c, p, "Cancel", k.ret.Label(), false, false) {
				a.closeOverlay(o)
				a.preserveCommitMessage(o)
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

// isClearKey is lazygit's ctrl+u in a text field: delete to the start of the line.
func isClearKey(t ui.TextKey) bool {
	return t.Mods&ui.Ctrl != 0 && t.Key == ui.KeyU
}

// keyAsYouType is lazygit's menu with FilterAsYouType: until there is a
// filter the configured navigation keys drive the menu; after that every
// printable key is filter text, and only the keys that can't be typed
// (arrows, pages, home/end) move. An item's own key never runs it.
func (m *menuOverlay) keyAsYouType(a *App, ch keymap.Chord) {
	k := a.keys
	items := m.visible()
	page := 15
	physical := map[ui.Key]int{ui.KeyDown: 1, ui.KeyUp: -1, ui.KeyPageDown: page, ui.KeyPageUp: -page, ui.KeyEnd: 1 << 30, ui.KeyHome: -(1 << 30)}
	if ch.Mods == 0 && ch.Rune == 0 {
		if d, ok := physical[ch.Key]; ok {
			m.moveBy(d)
			return
		}
	}
	switch {
	case ch.Key == ui.KeyEscape && m.filtering:
		// Escape gives up the filter first, keeping the selected item.
		var sel *menuItem
		if m.cursor >= 0 && m.cursor < len(items) {
			sel = &items[m.cursor]
		}
		m.filtering, m.filter, m.cursor = false, "", 0
		if sel != nil {
			for i := range m.items {
				if m.items[i].label == sel.label && m.items[i].section == sel.section {
					m.cursor = i
				}
			}
		}
		m.list.ScrollIntoView(m.cursor)
		return
	case ch.Key == ui.KeyEnter || (!m.filtering && k.confirmMenu.Matches(ch)):
		if len(items) > 0 {
			m.choose(a, items[m.cursor])
		}
		return
	case ch.Key == ui.KeyBackspace:
		if m.filter != "" {
			_, size := utf8.DecodeLastRuneInString(m.filter)
			m.filter = m.filter[:len(m.filter)-size]
			m.cursor = 0
		}
		return
	case ch.Mods == ui.Ctrl && ch.Rune == 'u' && m.filtering:
		m.filter, m.cursor = "", 0
		return
	}
	if !m.filtering {
		moves := []struct {
			b keymap.Binding
			d int
		}{{k.next, 1}, {k.prev, -1}, {k.nextPage, page}, {k.prevPage, -page}, {k.bottom, 1 << 30}, {k.top, -(1 << 30)}}
		for _, mv := range moves {
			if mv.b.Matches(ch) {
				m.moveBy(mv.d)
				return
			}
		}
		if k.ret.Matches(ch) {
			a.closeOverlay(m)
			return
		}
	}
	if ch.Rune != 0 && ch.Mods == 0 || ch.Key == ui.KeySpace {
		r := ch.Rune
		if ch.Key == ui.KeySpace {
			r = ' '
		}
		m.filtering = true
		m.filter += string(r)
		m.cursor = 0
		m.list.ScrollTo(0, ui.Start)
		return
	}
	if ch.Key == ui.KeyEscape || k.ret.Matches(ch) {
		a.closeOverlay(m)
		return
	}
	// Anything else reaches the global keybindings.
	a.runGlobalKey(ch)
}

// runGlobalKey runs the global action bound to ch, for a key a popup doesn't
// handle (lazygit's global keybindings stay live under a menu).
func (a *App) runGlobalKey(ch keymap.Chord) {
	for _, act := range a.actions {
		if act.scope == scopeGlobal && act.keys.Matches(ch) {
			act.run()
			return
		}
	}
}
