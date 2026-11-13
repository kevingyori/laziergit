package app

import (
	"testing"

	"github.com/alecthomas/chroma/v2"
)

func TestLexDiffHighlightsHunkLines(t *testing.T) {
	text := "diff --git a/main.go b/main.go\n" +
		"index 1111111..2222222 100644\n" +
		"--- a/main.go\n" +
		"+++ b/main.go\n" +
		"@@ -1,3 +1,5 @@\n" +
		" package main\n" +
		"-var s = 1\n" +
		"+var s = `one\n" +
		"+func not code`\n" +
		" \tfunc f() {}\n" +
		"diff --git a/notes.unknownext b/notes.unknownext\n" +
		"@@ -1 +1 @@\n" +
		"-func a\n" +
		"+func b\n"
	syntax := lexDiff(text)
	kindOf := func(line int, word string) chroma.TokenType {
		for _, tok := range syntax[line] {
			if tok.text == word {
				return tok.kind
			}
		}
		t.Fatalf("line %d has no %q token: %+v", line, word, syntax[line])
		return 0
	}
	for i := range 5 {
		if syntax[i] != nil {
			t.Errorf("header line %d highlighted: %+v", i, syntax[i])
		}
	}
	if !kindOf(5, "package").InCategory(chroma.Keyword) {
		t.Error("context line's keyword not highlighted")
	}
	if !kindOf(6, "var").InCategory(chroma.Keyword) {
		t.Error("deleted line's keyword not highlighted")
	}
	if !kindOf(8, "func not code`").InSubCategory(chroma.LiteralString) {
		t.Error("an added raw string's second line lost its string context")
	}
	if !kindOf(9, "func").InCategory(chroma.Keyword) {
		t.Error("context line after the string not highlighted as code")
	}
	if tokensText(syntax[9]) != "\tfunc f() {}" {
		t.Errorf("tokens %q don't rebuild the line", tokensText(syntax[9]))
	}
	for i := 10; i < len(syntax); i++ {
		if syntax[i] != nil {
			t.Errorf("line %d of a file with no lexer highlighted", i)
		}
	}
}
