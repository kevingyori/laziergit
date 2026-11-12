package app

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"github.com/jesseduffield/lazygit/pkg/config"
	"github.com/jesseduffield/lazygit/pkg/utils/yaml_utils"
	"gopkg.in/yaml.v3"

	"laziergit/internal/keymap"
)

type settingKind int

const (
	settingBool settingKind = iota
	settingChoice
	settingText
	settingNumber
	settingList
)

// setting is one editable leaf of lazygit's UserConfig.
type setting struct {
	path    []string // yaml keys from the root
	index   []int    // reflect field index into UserConfig
	kind    settingKind
	choices []string
	label   string
	section string
}

// settingsList walks UserConfig once. Maps and lists of structs (custom
// commands, side panels, icons) have no single-value control and are left to
// the file.
var settingsList = func() []setting {
	var out []setting
	var walk func(rt reflect.Type, path []string, index []int)
	walk = func(rt reflect.Type, path []string, index []int) {
		for i := range rt.NumField() {
			f := rt.Field(i)
			name := strings.Split(f.Tag.Get("yaml"), ",")[0]
			schema := f.Tag.Get("jsonschema")
			if name == "" || name == "-" || strings.Contains(schema, "deprecated") {
				continue
			}
			p := append(append([]string{}, path...), name)
			idx := append(append([]int{}, index...), i)
			t := f.Type
			if t.Kind() == reflect.Pointer {
				t = t.Elem()
			}
			s := setting{path: p, index: idx}
			switch {
			case t.Kind() == reflect.Struct:
				walk(t, p, idx)
				continue
			case t.Kind() == reflect.Bool:
				s.kind = settingBool
			case t.Kind() == reflect.String:
				s.kind = settingText
				for _, part := range strings.Split(schema, ",") {
					if v, ok := strings.CutPrefix(part, "enum="); ok {
						s.choices = append(s.choices, v)
					}
				}
				if len(s.choices) > 0 {
					s.kind = settingChoice
				}
			case t.Kind() >= reflect.Int && t.Kind() <= reflect.Float64:
				s.kind = settingNumber
			case t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.String:
				s.kind = settingList
			default:
				continue
			}
			s.label = humanize(name)
			s.section = "General"
			if len(path) > 0 {
				parts := make([]string, len(path))
				for j, k := range path {
					parts[j] = humanize(k)
				}
				s.section = strings.Join(parts, " › ")
			}
			out = append(out, s)
		}
	}
	walk(reflect.TypeOf(config.UserConfig{}), nil, nil)
	return out
}()

// humanize turns a yaml key into a label: "scrollHeight" → "Scroll height".
func humanize(key string) string {
	switch key {
	case "os":
		return "OS"
	case "gui":
		return "GUI"
	}
	var b strings.Builder
	for i, r := range key {
		switch {
		case i == 0:
			b.WriteRune(unicode.ToUpper(r))
		case unicode.IsUpper(r):
			b.WriteRune(' ')
			b.WriteRune(unicode.ToLower(r))
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (s setting) value(cfg *config.UserConfig) reflect.Value {
	v := reflect.ValueOf(cfg).Elem().FieldByIndex(s.index)
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return reflect.Zero(v.Type().Elem())
		}
		v = v.Elem()
	}
	return v
}

// text is the value as it's typed into a prompt.
func (s setting) text(cfg *config.UserConfig) string {
	v := s.value(cfg)
	switch s.kind {
	case settingBool:
		return strconv.FormatBool(v.Bool())
	case settingList:
		items := make([]string, v.Len())
		for i := range items {
			items[i] = v.Index(i).String()
		}
		return strings.Join(items, ", ")
	case settingNumber:
		switch {
		case v.CanInt():
			return strconv.FormatInt(v.Int(), 10)
		case v.CanUint():
			return strconv.FormatUint(v.Uint(), 10)
		}
		return strconv.FormatFloat(v.Float(), 'g', -1, 64)
	}
	return v.String()
}

// parse turns typed text into the value written to the file.
func (s setting) parse(cfg *config.UserConfig, text string) (any, error) {
	text = strings.TrimSpace(text)
	switch s.kind {
	case settingList:
		items := []string{}
		for _, item := range strings.Split(text, ",") {
			if item = strings.TrimSpace(item); item != "" {
				items = append(items, item)
			}
		}
		return items, nil
	case settingNumber:
		v := s.value(cfg)
		var err error
		switch {
		case v.CanInt():
			var n int64
			if n, err = strconv.ParseInt(text, 10, 64); err == nil {
				return n, nil
			}
		case v.CanUint():
			var n uint64
			if n, err = strconv.ParseUint(text, 10, 64); err == nil {
				return n, nil
			}
		default:
			var n float64
			if n, err = strconv.ParseFloat(text, 64); err == nil {
				return n, nil
			}
		}
		return nil, errors.New(s.label + " must be a number")
	}
	return text, nil
}

// settingsFile is the config file Settings writes: the last of
// LG_CONFIG_FILE's files (it wins), or the global config.yml.
func (a *App) settingsFile() string {
	if files := os.Getenv("LG_CONFIG_FILE"); files != "" {
		parts := strings.Split(files, ",")
		return parts[len(parts)-1]
	}
	return filepath.Join(a.repo.AppConfig.GetUserConfigDir(), config.ConfigFilename)
}

// writeSetting sets one key in a YAML file, keeping the rest of the file
// (comments and order included).
func writeSetting(file string, path []string, value any) error {
	data, err := os.ReadFile(file)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return err
	}
	if len(doc.Content) == 0 {
		doc = yaml.Node{Kind: yaml.DocumentNode, HeadComment: doc.HeadComment, Content: []*yaml.Node{{Kind: yaml.MappingNode}}}
	}
	node := doc.Content[0]
	for _, key := range path {
		if node.Kind == yaml.ScalarNode && node.Tag == "!!null" {
			*node = yaml.Node{Kind: yaml.MappingNode, HeadComment: node.HeadComment, LineComment: node.LineComment}
		}
		if node.Kind != yaml.MappingNode {
			return errors.New(strings.Join(path, ".") + ": the file has a non-map value on the way")
		}
		_, next := yaml_utils.LookupKey(node, key)
		if next == nil {
			next = &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null"}
			node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, next)
		}
		node = next
	}
	var encoded yaml.Node
	if err := encoded.Encode(value); err != nil {
		return err
	}
	if encoded.Kind == yaml.SequenceNode {
		encoded.Style = yaml.FlowStyle
	}
	encoded.HeadComment, encoded.LineComment, encoded.FootComment = node.HeadComment, node.LineComment, node.FootComment
	*node = encoded
	out, err := yaml_utils.YamlMarshal(&doc)
	if err != nil {
		return err
	}
	return os.WriteFile(file, out, 0o644)
}

// --- overlay ----------------------------------------------------------------

type settingsOverlay struct {
	filter string
	cursor int
	list   ui.ListState
}

// OpenSettings shows the settings window (Cmd+, in the app menu).
func (a *App) OpenSettings() {
	for _, o := range a.overlays {
		if _, ok := o.(*settingsOverlay); ok {
			return
		}
	}
	a.pushOverlay(&settingsOverlay{})
}

func (o *settingsOverlay) visible() []setting {
	if o.filter == "" {
		return settingsList
	}
	f := strings.ToLower(o.filter)
	var out []setting
	for _, s := range settingsList {
		text := strings.ToLower(s.label + " " + s.section + " " + strings.Join(s.path, "."))
		if strings.Contains(text, f) {
			out = append(out, s)
		}
	}
	return out
}

func (o *settingsOverlay) moveBy(d int) {
	o.cursor = clamp(o.cursor+d, 0, len(o.visible())-1)
	o.list.ScrollIntoView(o.cursor)
}

func (o *settingsOverlay) key(a *App, ch keymap.Chord) {
	switch {
	case a.keys.ret.Matches(ch):
		if o.filter != "" {
			o.filter, o.cursor = "", 0
		} else {
			a.closeOverlay(o)
		}
	case ch.Key == ui.KeyDown:
		o.moveBy(1)
	case ch.Key == ui.KeyUp:
		o.moveBy(-1)
	case ch.Key == ui.KeyPageDown:
		o.moveBy(10)
	case ch.Key == ui.KeyPageUp:
		o.moveBy(-10)
	case ch.Key == ui.KeyEnter || ch.Key == ui.KeySpace:
		if items := o.visible(); len(items) > 0 {
			a.editSetting(items[o.cursor])
		}
	case ch.Key == ui.KeyBackspace:
		if o.filter != "" {
			_, size := utf8.DecodeLastRuneInString(o.filter)
			o.filter, o.cursor = o.filter[:len(o.filter)-size], 0
		}
	case ch.Rune != 0 && ch.Mods&^ui.Shift == 0:
		o.filter += string(ch.Rune)
		o.cursor = 0
		o.list.ScrollIntoView(0)
	}
}

// editSetting flips a switch, or asks for the new value on top of Settings.
func (a *App) editSetting(s setting) {
	cfg := a.repo.Config
	switch s.kind {
	case settingBool:
		a.saveSetting(s, !s.value(cfg).Bool())
	case settingChoice:
		current := s.value(cfg).String()
		items := make([]menuItem, len(s.choices))
		for i, choice := range s.choices {
			label, widget := choice, "( )"
			if label == "" {
				label = "(none)"
			}
			if choice == current {
				widget = "(•)"
			}
			items[i] = menuItem{label: label, widget: widget, run: func() { a.saveSetting(s, choice) }}
		}
		a.pushOverlay(&menuOverlay{title: s.label, items: items, cursor: max(0, indexOf(s.choices, current)), noItemKeys: true})
	default:
		a.prompt(s.label, s.text(cfg), strings.Join(s.path, "."), func(text string) {
			v, err := s.parse(a.repo.Config, text)
			if err != nil {
				a.showError(s.label, err)
				return
			}
			a.saveSetting(s, v)
		})
	}
}

func indexOf(items []string, s string) int {
	for i, it := range items {
		if it == s {
			return i
		}
	}
	return -1
}

// saveSetting writes the file, then reloads it as lazygit does on focus.
func (a *App) saveSetting(s setting, value any) {
	if err := writeSetting(a.settingsFile(), s.path, value); err != nil {
		a.showError("Settings", err)
		return
	}
	a.FocusIn()
}

func (o *settingsOverlay) view(a *App, c *ui.Context, p *Palette) {
	items := o.visible()
	cfg := a.repo.Config
	card(c, p, 720, func() {
		ui.Row(c).PaddingX(16).PaddingY(12).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Settings").FontWeight(600)
			ui.Box(c).Grow(1)
			file := a.settingsFile()
			if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(file, home) {
				file = "~" + file[len(home):]
			}
			ui.Text(c, file).Font(monoFont).FontSize(12).TextColor(p.Faint).SingleLine().Shrink(1)
		})
		ui.Row(c).PaddingX(16).Height(32).AlignItems(ui.Center).Children(func() {
			if o.filter == "" {
				ui.Text(c, "Type to filter").FontSize(12).TextColor(p.Muted)
			} else {
				ui.RichText(c, ui.Span{Text: o.filter, Font: monoFont, Color: p.Text}, ui.Span{Text: "▏", Color: p.Accent}).FontSize(12)
			}
		})
		ui.Box(c).Height(1).FillWidth().Background(p.Hairline)
		ui.List(c, &o.list, len(items), func(i int) {
			s := items[i]
			ui.Column(c).Children(func() {
				if i == 0 || items[i-1].section != s.section {
					ui.Text(c, strings.ToUpper(s.section)).FontSize(sizeCaption).FontWeight(650).
						LetterSpacing(0.7).TextColor(p.Faint).Padding(12, 16, 4, 16)
				}
				row := ui.Row(c).Height(30).PaddingX(10).MarginX(6).Gap(12).AlignItems(ui.Center).Radius(5)
				switch {
				case i == o.cursor:
					row.Background(p.AccentSoft)
				case row.Hovered():
					row.Background(p.Hover)
				}
				edit := false
				row.Children(func() {
					ui.Text(c, s.label).SingleLine().Shrink(0)
					ui.Box(c).Grow(1)
					edit = settingControl(c, p, s, cfg)
				})
				switch {
				case edit || row.DoubleClicked():
					o.cursor = i
					a.editSetting(s)
				case row.Clicked():
					o.cursor = i
				}
			})
		}).Height(15 * 30).PaddingY(6)
		if len(items) == 0 {
			ui.Text(c, "No matches").TextColor(p.Muted).Padding(0, 16, 12, 16)
		}
		ui.Box(c).Height(1).FillWidth().Background(p.Hairline)
		ui.Row(c).PaddingX(16).PaddingY(8).Gap(6).AlignItems(ui.Center).Children(func() {
			for _, h := range [][2]string{{"↑↓", "Select"}, {"enter", "Change"}, {a.keys.ret.Label(), "Close"}} {
				keycap(c, p, h[0])
				ui.Text(c, h[1]).FontSize(12).TextColor(p.Muted).Padding(0, 8, 0, 0)
			}
		})
	})
}

// settingControl draws the value at the row's end; it reports a click on it.
func settingControl(c *ui.Context, p *Palette, s setting, cfg *config.UserConfig) bool {
	if s.kind == settingBool {
		on := s.value(cfg).Bool()
		track := ui.Row(c).Width(32).Height(18).Radius(9).Padding(2, 2, 2, 2).Cursor(ui.CursorPointer)
		if on {
			track.Background(p.Accent).Justify(ui.End)
		} else {
			track.Background(p.Idle)
		}
		track.Children(func() {
			ui.Box(c).Width(14).Height(14).Radius(7).Background(ui.Hex("#ffffff"))
		})
		return track.Clicked()
	}
	text, color := s.text(cfg), p.Muted
	switch {
	case s.kind == settingChoice:
		if text == "" {
			text = "(none)"
		}
		text += "  ▾"
	case text == "":
		text, color = "Not set", p.Faint
	}
	v := ui.Text(c, text).Font(monoFont).FontSize(12).TextColor(color).SingleLine().MaxWidth(360).Cursor(ui.CursorPointer)
	return v.Clicked()
}
