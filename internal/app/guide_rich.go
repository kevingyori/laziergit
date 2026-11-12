package app

import (
	"encoding/base64"
	"path"
	"strings"

	"github.com/egoist/mygo/ui"
	"golang.org/x/net/html"
	"laziergit/internal/reviewbridge"
)

// richBlock is a paragraph, list item or preformatted block of an agent's
// explanation, already split into styled spans.
type richBlock struct {
	spans  []ui.Span
	bullet string
	pre    bool
}

// parseRich renders the small HTML subset agents may use (b, strong, i, em,
// code, a, br, p, ul/ol/li, pre) as native rich text. Plain text is one
// block. Anything else is shown as its text; nothing is executed or fetched.
func parseRich(src string, p *Palette, size float32, color ui.Color) ([]richBlock, []string) {
	var blocks []richBlock
	var links []string
	cur := richBlock{}
	var bold, italic, code, link, pre int
	var lists []int // item counters; -1 for unordered
	flush := func() {
		if len(cur.spans) > 0 {
			blocks = append(blocks, cur)
		}
		cur = richBlock{pre: pre > 0}
	}
	z := html.NewTokenizer(strings.NewReader(src))
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}
		tok := z.Token()
		switch tt {
		case html.TextToken:
			text := tok.Data
			if pre == 0 {
				text = strings.Join(strings.Fields(text), " ")
				if strings.HasPrefix(tok.Data, " ") || strings.HasPrefix(tok.Data, "\n") {
					text = " " + text
				}
				if strings.HasSuffix(tok.Data, " ") || strings.HasSuffix(tok.Data, "\n") {
					text += " "
				}
				if strings.TrimSpace(text) == "" && len(cur.spans) == 0 {
					continue
				}
			}
			s := ui.Span{Text: text, Size: size, Color: color}
			if bold > 0 {
				s.Weight = 650
			}
			s.Italic = italic > 0
			if code > 0 || pre > 0 {
				s.Font, s.Size, s.Color = monoFont, size-1, p.HunkText
			}
			if link > 0 {
				s.Color, s.Underline = p.Accent, true
			}
			cur.spans = append(cur.spans, s)
		case html.StartTagToken, html.SelfClosingTagToken:
			switch tok.Data {
			case "b", "strong":
				bold++
			case "i", "em":
				italic++
			case "code":
				code++
			case "a":
				link++
				for _, at := range tok.Attr {
					if at.Key == "href" && (strings.HasPrefix(at.Val, "https://") || strings.HasPrefix(at.Val, "http://")) {
						links = append(links, at.Val)
					}
				}
			case "br":
				flush()
			case "p", "div", "h1", "h2", "h3", "h4":
				flush()
				if strings.HasPrefix(tok.Data, "h") {
					bold++
				}
			case "pre":
				flush()
				pre++
				cur.pre = true
			case "ul":
				flush()
				lists = append(lists, -1)
			case "ol":
				flush()
				lists = append(lists, 0)
			case "li":
				flush()
				cur.bullet = "•"
				if n := len(lists); n > 0 && lists[n-1] >= 0 {
					lists[n-1]++
					cur.bullet = strings.Repeat(" ", 0) + itoa(lists[n-1]) + "."
				}
			}
		case html.EndTagToken:
			switch tok.Data {
			case "b", "strong":
				bold = max(0, bold-1)
			case "i", "em":
				italic = max(0, italic-1)
			case "code":
				code = max(0, code-1)
			case "a":
				link = max(0, link-1)
			case "p", "div", "li":
				flush()
			case "h1", "h2", "h3", "h4":
				bold = max(0, bold-1)
				flush()
			case "pre":
				flush()
				pre = max(0, pre-1)
				cur.pre = pre > 0
			case "ul", "ol":
				flush()
				if len(lists) > 0 {
					lists = lists[:len(lists)-1]
				}
			}
		}
	}
	flush()
	return blocks, links
}
func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}

// richText shows an explanation; maxLines > 0 clamps a collapsed one.
func (a *App) richText(c *ui.Context, p *Palette, src string, color ui.Color, maxLines int) {
	blocks, links := parseRich(src, p, 13, color)
	ui.Column(c).Gap(3).Children(func() {
		for i, b := range blocks {
			if maxLines > 0 && i > 0 {
				return
			}
			row := ui.Row(c).Gap(6)
			if b.pre {
				row.Padding(6, 8, 6, 8).Background(p.Header).Radius(4)
			}
			row.Children(func() {
				if b.bullet != "" {
					ui.Text(c, b.bullet).TextColor(p.Muted).Width(16).TextAlign(ui.End)
				}
				t := ui.RichText(c, b.spans...).LineHeight(1.35).Grow(1).Shrink(1).MinWidth(0)
				if maxLines > 0 {
					t.MaxLines(maxLines)
				}
			})
		}
		if maxLines == 0 {
			for _, l := range links {
				l := l
				a.guideButton(c, p, "↗ "+l, action{run: func() {
					if err := a.openLink(l); err != nil {
						a.guide.err = err.Error()
					}
				}})
			}
		}
	})
}

// guideMedia shows an explanation's pictures, decoded once per guide.
func (a *App) guideMedia(c *ui.Context, p *Palette, media []reviewbridge.Media, width float32) {
	for _, m := range media {
		src := a.mediaSource(m)
		ui.Column(c).Gap(3).Children(func() {
			if src == nil {
				ui.Text(c, "Picture unavailable").FontSize(sizeCaption).TextColor(p.Faint)
			} else {
				w, h := ui.ImageSize(src)
				if limit := min(width-24, 640); w > limit && w > 0 {
					w, h = limit, h*limit/w
				}
				ui.Image(c, src).Size(w, h)
			}
			if m.Caption != "" {
				ui.Text(c, m.Caption).FontSize(sizeCaption).TextColor(p.Muted)
			}
		})
	}
}

// mediaSource returns a decoded picture, or nil while it loads off the UI
// thread (repository images need git) or when it can't be shown.
func (a *App) mediaSource(m reviewbridge.Media) ui.ImageSource {
	g := a.guide
	key := m.SVG + "\x00" + m.Path + "\x00" + m.DataURL
	if src, ok := g.media[key]; ok {
		return src
	}
	if g.media == nil {
		g.media = map[string]ui.ImageSource{}
	}
	g.media[key] = nil // loading
	session := g.session
	a.bg(func() {
		src := decodeMedia(session, m)
		a.post(func() {
			if a.guide == g {
				g.media[key] = src
			}
		})
	})
	return nil
}
func decodeMedia(session string, m reviewbridge.Media) ui.ImageSource {
	var data []byte
	isSVG := m.SVG != "" || strings.EqualFold(path.Ext(m.Path), ".svg")
	switch {
	case m.SVG != "":
		data = []byte(m.SVG)
	case m.DataURL != "":
		if _, enc, ok := strings.Cut(m.DataURL, ","); ok {
			data, _ = base64.StdEncoding.DecodeString(enc)
		}
		isSVG = strings.HasPrefix(m.DataURL, "data:image/svg")
	default:
		var snapshot reviewbridge.Snapshot
		if reviewbridge.Read(session, "snapshot.json", &snapshot) == nil {
			text, _ := reviewbridge.SourceText(snapshot, m.Path, "new")
			data = []byte(text)
		}
	}
	if isSVG {
		if svg, err := ui.ParseSVG(data); err == nil {
			return svg
		}
		return nil
	}
	if bmp, err := ui.DecodeBitmap(data); err == nil {
		return bmp
	}
	return nil
}
