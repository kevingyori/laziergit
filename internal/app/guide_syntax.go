package app

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/egoist/mygo/ui"
	"laziergit/internal/repo"
)

type guideToken struct {
	kind chroma.TokenType
	text string
}
type guideSyntaxFile struct{ lines [][]guideToken }
type guideSyntax map[string]*guideSyntaxFile

func guideBlobHashes(text string) (string, string) {
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "@@") {
			break
		}
		if strings.HasPrefix(line, "index ") {
			fields := strings.Fields(line)
			if len(fields) > 1 {
				a, b, ok := strings.Cut(fields[1], "..")
				if ok {
					return a, b
				}
			}
		}
	}
	return "", ""
}
func syntaxKey(path, hash string) string { return path + "\x00" + hash }
func loadGuideSyntax(r *repo.Repo, hunks []guideHunk) guideSyntax {
	out := guideSyntax{}
	// Share tokenized whole blobs across hunks. Work is bounded and off the UI thread.
	bytes := 0
	for _, h := range hunks {
		if lexers.Match(h.Path) == nil {
			continue
		}
		oldHash, newHash := h.OldHash, h.NewHash
		for side, hash := range []string{oldHash, newHash} {
			key := syntaxKey(h.Path, hash)
			if _, ok := out[key]; ok {
				continue
			}
			if len(out) >= 256 || bytes >= 32<<20 {
				return out
			}
			out[key] = nil
			path := ""
			if side == 1 && h.Layer == "Unstaged" {
				path = h.Path
			}
			if text, ok := r.GuideBlob(hash, path); ok {
				bytes += len(text)
				out[key] = lexGuideFile(h.Path, text)
			}
		}
	}
	return out
}
func lexGuideFile(path, text string) (result *guideSyntaxFile) {
	// A lexer failure is a plain-text fallback, never a broken review surface.
	defer func() {
		if recover() != nil {
			result = nil
		}
	}()
	lexer := lexers.Match(path)
	if lexer == nil || len(text) > repo.GuideBlobLimit || strings.ContainsRune(text, 0) {
		return nil
	}
	iterator, err := chroma.Coalesce(lexer).Tokenise(nil, text)
	if err != nil {
		return nil
	}
	lines := [][]guideToken{{}}
	for token := iterator(); token != chroma.EOF; token = iterator() {
		parts := strings.Split(token.Value, "\n")
		for i, part := range parts {
			if i > 0 {
				lines = append(lines, []guideToken{})
			}
			part = strings.TrimSuffix(part, "\r")
			if part != "" {
				last := len(lines) - 1
				lines[last] = append(lines[last], guideToken{kind: token.Type, text: part})
			}
		}
	}
	return &guideSyntaxFile{lines: lines}
}
func (g *guideState) syntaxTokens(row guideStreamRow) []guideToken {
	if row.hunk < 0 || (row.line.kind != lineAdd && row.line.kind != lineDel && row.line.kind != lineCtx) {
		return nil
	}
	h := g.hunks[row.hunk]
	oldHash, newHash := h.OldHash, h.NewHash
	hash, n := newHash, row.newLine
	if row.line.kind == lineDel {
		hash, n = oldHash, row.oldLine
	}
	file := g.syntax[syntaxKey(h.Path, hash)]
	if file == nil || n < 1 || n > len(file.lines) {
		return nil
	}
	tokens := file.lines[n-1]
	// Git may use filters or CRLF conversion. Never apply mismatched source tokens.
	var source strings.Builder
	for _, t := range tokens {
		source.WriteString(t.text)
	}
	body := row.line.text[1:] // exactly the unified-diff prefix
	if strings.ReplaceAll(source.String(), "\t", "    ") != body {
		return nil
	}
	return tokens
}
func guideTokenColor(kind chroma.TokenType, p *Palette) ui.Color {
	switch {
	case kind.InCategory(chroma.Keyword), kind == chroma.GenericHeading, kind == chroma.GenericSubheading:
		return p.Accent
	case kind.InSubCategory(chroma.LiteralString):
		return p.Staged
	case kind.InCategory(chroma.Comment):
		return p.Muted
	case kind.InSubCategory(chroma.LiteralNumber):
		return p.Unstaged
	case kind.InSubCategory(chroma.NameFunction), kind == chroma.NameClass, kind.InSubCategory(chroma.NameBuiltin):
		return p.HunkText
	default:
		return p.Text
	}
}
