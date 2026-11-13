package app

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"laziergit/internal/repo"
)

// diffSyntax is a plain diff's syntax tokens, by line of its text. A line
// outside a hunk, or in a file with no lexer, has none.
type diffSyntax [][]syntaxToken

// lexDiff highlights each hunk from the diff alone: its old side (context
// and deletions) and new side (context and additions) are lexed as two
// snippets, so a construct opened inside the hunk carries across its lines.
func lexDiff(text string) diffSyntax {
	lines := repo.SplitLines(text)
	out := make(diffSyntax, len(lines))
	path := ""
	var oldSide, newSide []int
	// The new side goes last, so a context line keeps its tokens from there.
	flush := func() {
		for _, side := range [][]int{oldSide, newSide} {
			if len(side) == 0 || path == "" {
				continue
			}
			bodies := make([]string, len(side))
			for k, i := range side {
				bodies[k] = lines[i][1:]
			}
			file := lexFile(path, strings.Join(bodies, "\n"))
			if file == nil {
				continue
			}
			for k, i := range side {
				if k < len(file.lines) && tokensText(file.lines[k]) == bodies[k] {
					out[i] = file.lines[k]
				}
			}
		}
		oldSide, newSide = oldSide[:0], newSide[:0]
	}
	inHunk := false
	for i, l := range lines {
		switch {
		case strings.HasPrefix(l, "diff --git"):
			flush()
			inHunk, path = false, diffPath(l)
		case strings.HasPrefix(l, "@@"):
			flush()
			inHunk = true
		case !inHunk || l == "":
		case l[0] == ' ':
			oldSide, newSide = append(oldSide, i), append(newSide, i)
		case l[0] == '-':
			oldSide = append(oldSide, i)
		case l[0] == '+':
			newSide = append(newSide, i)
		case l[0] != '\\': // "\ No newline at end of file" stays in the hunk
			flush()
			inHunk = false
		}
	}
	flush()
	return out
}

func tokensText(tokens []syntaxToken) string {
	var b strings.Builder
	for _, t := range tokens {
		b.WriteString(t.text)
	}
	return b.String()
}

// syntaxCache lexes a pane's text off the main thread: until the tokens
// arrive the pane draws its diff plain.
type syntaxCache struct {
	text, pending string
	lines         diffSyntax
}

func (a *App) syntaxOf(c *syntaxCache, text string) diffSyntax {
	if text == "" {
		return nil
	}
	if c.text == text {
		return c.lines
	}
	if c.pending != text {
		c.pending = text
		a.bg(func() {
			lines := lexDiff(text)
			a.post(func() {
				if c.pending == text {
					c.text, c.lines = text, lines
				}
			})
		})
	}
	return nil
}

func syntaxAt(s diffSyntax, i int) []syntaxToken {
	if i < 0 || i >= len(s) {
		return nil
	}
	return s[i]
}

// codeSpans draws a highlighted diff line: its +/-/space prefix in the
// line's own colour, then the code's tokens.
func codeSpans(prefix string, color ui.Color, tokens []syntaxToken, p *Palette) []ui.Span {
	spans := []ui.Span{{Text: prefix, Font: monoFont, Size: 12, Color: color}}
	for _, t := range tokens {
		spans = append(spans, ui.Span{Text: strings.ReplaceAll(t.text, "\t", "    "), Font: monoFont, Size: 12, Color: syntaxColor(t.kind, p)})
	}
	return spans
}
