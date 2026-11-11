package app

import (
	"runtime"
	"strings"
	"sync"

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

// loadGuideSyntax tokenizes whole old/new file versions, shared across hunks:
// one batched blob read, then lexing in parallel. It runs off the UI thread
// after the guide is already on screen.
func loadGuideSyntax(r *repo.Repo, hunks []guideHunk) guideSyntax {
	type want struct{ path, hash string }
	var wants []want
	var reqs []repo.GuideBlobRequest
	seen := map[string]bool{}
	for _, h := range hunks {
		if lexers.Match(h.Path) == nil {
			continue
		}
		for side, hash := range []string{h.OldHash, h.NewHash} {
			key := syntaxKey(h.Path, hash)
			if seen[key] {
				continue
			}
			seen[key] = true
			path := ""
			if side == 1 && h.Layer == "Unstaged" {
				path = h.Path
			}
			wants = append(wants, want{h.Path, hash})
			reqs = append(reqs, repo.GuideBlobRequest{Hash: hash, Path: path})
		}
	}
	blobs := r.GuideBlobs(reqs)
	out := guideSyntax{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	work := make(chan want)
	for range max(1, runtime.NumCPU()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for w := range work {
				file := lexGuideFile(w.path, blobs[w.hash])
				mu.Lock()
				out[syntaxKey(w.path, w.hash)] = file
				mu.Unlock()
			}
		}()
	}
	for _, w := range wants {
		if _, ok := blobs[w.hash]; ok {
			work <- w
		}
	}
	close(work)
	wg.Wait()
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
	if lexer == nil || strings.ContainsRune(text, 0) {
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

// applySyntax colours the streams once highlighting arrives, in place, so the
// reader's position and selection stay where they are.
func (g *guideState) applySyntax(syntax guideSyntax) {
	g.syntax = syntax
	for _, rows := range g.streams {
		for i := range rows {
			rows[i].tokens = g.syntaxTokens(rows[i])
		}
	}
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
