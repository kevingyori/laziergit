package app

import (
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
)

// A diff renderer's output (git.diffRenderers) is terminal text: SGR colour
// escapes, and OSC 1717 records in which the renderer states which diff line
// each row shows. parseANSI turns it into rows of styled spans, as lazygit's
// gocui reads it into its view (gocui/escape.go).

// ansiColor is a terminal colour: none, one of the 256 palette entries, or
// an RGB value.
type ansiColor struct {
	kind    uint8 // 0 default, 1 palette, 2 rgb
	idx     uint8
	r, g, b uint8
}

type ansiStyle struct {
	fg, bg                     ansiColor
	bold, italic, underline    bool
	reverse, strike, faintText bool
}

type ansiSpan struct {
	text  string
	style ansiStyle
}

// ansiLine is one row of rendered output.
type ansiLine struct {
	text    string     // the row's text, without escapes
	spans   []ansiSpan // the text, styled
	records []string   // the OSC 1717 payloads the row carries, in order
}

// parseANSI splits rendered output into rows. The style carries over from
// one row to the next, as it does in a terminal.
func parseANSI(s string) []ansiLine {
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil
	}
	var out []ansiLine
	var st ansiStyle
	var line ansiLine
	var text, run strings.Builder
	flush := func() {
		if run.Len() > 0 {
			line.spans = append(line.spans, ansiSpan{text: run.String(), style: st})
			run.Reset()
		}
	}
	endLine := func() {
		flush()
		line.text = text.String()
		out = append(out, line)
		line = ansiLine{}
		text.Reset()
	}
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '\n':
			endLine()
			i++
		case c == '\r':
			i++
		case c == 0x1b && i+1 < len(s) && s[i+1] == '[':
			j := i + 2
			for j < len(s) && (s[j] < 0x40 || s[j] > 0x7e) {
				j++
			}
			if j < len(s) && s[j] == 'm' {
				flush()
				applySGR(&st, s[i+2:j])
			}
			i = min(j+1, len(s))
		case c == 0x1b && i+1 < len(s) && s[i+1] == ']':
			j, end := i+2, -1
			for j < len(s) {
				if s[j] == 0x07 {
					end = j + 1
					break
				}
				if s[j] == 0x1b && j+1 < len(s) && s[j+1] == '\\' {
					end = j + 2
					break
				}
				if s[j] == '\n' {
					break
				}
				j++
			}
			if end < 0 {
				end = j
			}
			if payload, ok := strings.CutPrefix(s[i+2:j], "1717;"); ok {
				line.records = append(line.records, payload)
			}
			i = end
		case c == 0x1b:
			i += 2
		default:
			text.WriteByte(c)
			run.WriteByte(c)
			i++
		}
	}
	endLine()
	return out
}

func applySGR(st *ansiStyle, params string) {
	if params == "" {
		*st = ansiStyle{}
		return
	}
	f := strings.FieldsFunc(params, func(r rune) bool { return r == ';' || r == ':' })
	n := func(k int) int {
		if k >= len(f) {
			return 0
		}
		v, _ := strconv.Atoi(f[k])
		return v
	}
	for k := 0; k < len(f); k++ {
		switch v := n(k); {
		case v == 0:
			*st = ansiStyle{}
		case v == 1:
			st.bold = true
		case v == 2:
			st.faintText = true
		case v == 3:
			st.italic = true
		case v == 4:
			st.underline = true
		case v == 7:
			st.reverse = true
		case v == 9:
			st.strike = true
		case v == 22:
			st.bold, st.faintText = false, false
		case v == 23:
			st.italic = false
		case v == 24:
			st.underline = false
		case v == 27:
			st.reverse = false
		case v == 29:
			st.strike = false
		case v >= 30 && v <= 37:
			st.fg = ansiColor{kind: 1, idx: uint8(v - 30)}
		case v >= 90 && v <= 97:
			st.fg = ansiColor{kind: 1, idx: uint8(v - 90 + 8)}
		case v == 39:
			st.fg = ansiColor{}
		case v >= 40 && v <= 47:
			st.bg = ansiColor{kind: 1, idx: uint8(v - 40)}
		case v >= 100 && v <= 107:
			st.bg = ansiColor{kind: 1, idx: uint8(v - 100 + 8)}
		case v == 49:
			st.bg = ansiColor{}
		case v == 38 || v == 48:
			var col ansiColor
			switch n(k + 1) {
			case 5:
				col = ansiColor{kind: 1, idx: uint8(n(k + 2))}
				k += 2
			case 2:
				col = ansiColor{kind: 2, r: uint8(n(k + 2)), g: uint8(n(k + 3)), b: uint8(n(k + 4))}
				k += 4
			}
			if v == 38 {
				st.fg = col
			} else {
				st.bg = col
			}
		}
	}
}

// color maps a terminal colour onto the theme: the 16 basic colours to the
// palette's colours of the same meaning, so that a renderer's red and green
// read as the app's deletions and additions; the rest as given.
func (c ansiColor) color(p *Palette, bg bool) (ui.Color, bool) {
	switch c.kind {
	case 2:
		return ui.RGB(c.r, c.g, c.b), true
	case 1:
		if c.idx < 16 {
			basic := c.idx % 8
			if bg {
				switch basic {
				case 1:
					return p.DelBg, true
				case 2:
					return p.AddBg, true
				case 4, 6:
					return p.HunkBg, true
				case 0:
					return p.Idle, true
				}
				return p.AccentSoft, true
			}
			switch basic {
			case 0:
				return p.Faint, true
			case 1:
				return p.DelText, true
			case 2:
				return p.AddText, true
			case 3:
				return p.Unstaged, true
			case 4:
				return p.Accent, true
			case 5:
				return p.MergedPR, true
			case 6:
				return p.HunkText, true
			case 7:
				return p.Text, true
			}
		}
		return xterm256(c.idx), true
	}
	return ui.Color{}, false
}

func xterm256(i uint8) ui.Color {
	switch {
	case i < 16:
		return ui.Color{}
	case i < 232:
		i -= 16
		level := func(v uint8) uint8 {
			if v == 0 {
				return 0
			}
			return 55 + v*40
		}
		return ui.RGB(level(i/36), level(i/6%6), level(i%6))
	}
	v := 8 + (i-232)*10
	return ui.RGB(v, v, v)
}

// uiSpans draws a row's spans in the mono font, over base as the default
// text colour.
func ansiUISpans(spans []ansiSpan, p *Palette, base ui.Color) []ui.Span {
	out := make([]ui.Span, 0, len(spans))
	for _, s := range spans {
		fg, okF := s.style.fg.color(p, false)
		if !okF {
			fg = base
		}
		bg, okB := s.style.bg.color(p, true)
		if s.style.reverse {
			fg, bg, okB = p.Panel, fg, true
		}
		span := ui.Span{Text: strings.ReplaceAll(s.text, "\t", "    "), Font: monoFont, Size: 12, Color: fg,
			Italic: s.style.italic, Underline: s.style.underline, Strikethrough: s.style.strike}
		if s.style.bold {
			span.Weight = 600
		}
		if s.style.faintText {
			span.Color = fg.Alpha(0.6)
		}
		if okB {
			span.Background = bg
		}
		out = append(out, span)
	}
	return out
}
