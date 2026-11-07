package app

import (
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"laziergit/internal/testrepo"
)

func TestGuideSyntaxUsesWholeOldAndNewBlobs(t *testing.T) {
	dir := testrepo.New(t)
	original := "package a\n\nconst text = `\n" + strings.Repeat("this is string content\n", 40) + "`\n"
	testrepo.Write(t, dir, "multiline.go", original)
	git(t, dir, "add", "multiline.go")
	git(t, dir, "commit", "-qm", "add multiline string")
	lines := strings.Split(original, "\n")
	lines[24] = "return is still inside a string"
	testrepo.Write(t, dir, "multiline.go", strings.Join(lines, "\n"))
	a, tt := openApp(t, dir)
	press(tt, "~")
	defer a.closeGuide()
	g := a.guide
	additions, deletions := 0, 0
	for _, rows := range g.streams {
		for _, row := range rows {
			if row.hunk < 0 || g.hunks[row.hunk].Path != "multiline.go" {
				continue
			}
			if row.line.kind != lineAdd && row.line.kind != lineDel {
				continue
			}
			if strings.Contains(row.line.text, "const text") {
				t.Fatal("fixture must have opening delimiter outside the hunk")
			}
			if len(row.tokens) == 0 {
				t.Fatalf("no full-blob tokens: %+v", row)
			}
			for _, token := range row.tokens {
				if !token.kind.InSubCategory(chroma.LiteralString) {
					t.Fatalf("fragment lost multiline string context: %+v", token)
				}
			}
			if row.line.kind == lineAdd {
				additions++
			} else {
				deletions++
			}
		}
	}
	if additions != 1 || deletions != 1 {
		t.Fatalf("missing changed string lines: +%d -%d", additions, deletions)
	}
	// A later working-tree edit must not silently supply a different blob to this snapshot.
	_, hash := guideBlobHashes(g.hunks[len(g.hunks)-1].Text)
	for _, h := range g.hunks {
		if h.Path == "multiline.go" {
			_, hash = guideBlobHashes(h.Text)
			break
		}
	}
	testrepo.Write(t, dir, "multiline.go", "package changed\n")
	if _, ok := a.repo.GuideBlob(hash, "multiline.go"); ok {
		t.Fatal("accepted worktree content that no longer matches snapshot hash")
	}
}
func TestGuideSyntaxLanguagesAndFallback(t *testing.T) {
	for _, tc := range []struct {
		path, text string
		kind       chroma.TokenType
	}{
		{"file.go", "package a\nfunc Answer() int { return 42 }\n", chroma.Keyword},
		{"view.ts", "export const message: string = `hello`;\n", chroma.Keyword},
		{"notes.md", "# Heading\n\n**Important** change.\n", chroma.GenericHeading},
	} {
		t.Run(tc.path, func(t *testing.T) {
			file := lexGuideFile(tc.path, tc.text)
			if file == nil {
				t.Fatal("missing lexer")
			}
			found := false
			for _, line := range file.lines {
				for _, token := range line {
					if token.kind == tc.kind || token.kind.InSubCategory(tc.kind) {
						found = true
					}
				}
			}
			if !found {
				t.Fatalf("missing expected tokens: %+v", file.lines)
			}
		})
	}
	if lexGuideFile("binary.go", "a\x00b") != nil {
		t.Fatal("binary source should be plain")
	}
	// No size cap: highlighting runs off the UI thread after the guide shows.
	if lexGuideFile("large.go", "package a\n"+strings.Repeat("var x = 1\n", 150000)) == nil {
		t.Fatal("large source should still be highlighted")
	}
	if lexGuideFile("file.unrecognized-123", "unknown") != nil {
		t.Fatal("unknown source should be plain")
	}
	p := palette(true)
	if guideTokenColor(chroma.LiteralNumberInteger, p) == guideTokenColor(chroma.LiteralString, p) {
		t.Fatal("numbers and strings need distinct semantic colors")
	}
}
func TestGuideSyntaxShots(t *testing.T) {
	if shotDir() == "" {
		t.Skip("SHOT_DIR not set")
	}
	dir := testrepo.New(t)
	testrepo.Write(t, dir, "a.go", `package a

// DisplayName provides a fallback for people without a profile.
func DisplayName(name string, attempts int) string {
 if name == "" && attempts > 2 {
  return "Anonymous"
 }
 return name
}
`)
	testrepo.Write(t, dir, "view.ts", "export function title(name: string): string {\n  return name || 'Anonymous';\n}\n")
	a, tt := openApp(t, dir)
	press(tt, "~")
	defer a.closeGuide()
	for _, dark := range []bool{false, true} {
		suffix := "light"
		if dark {
			suffix = "dark"
		}
		tt.SetDark(dark)
		tt.Frame()
		shot(t, tt, shotDir()+"/guide-syntax-"+suffix+".png")
	}
	press(tt, "enter")
	press(tt, "j")
	shot(t, tt, shotDir()+"/guide-code-cursor.png")
}
