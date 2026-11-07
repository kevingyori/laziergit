package app

import (
	"testing"

	"github.com/jesseduffield/lazygit/pkg/commands/patch"
)

func TestParseANSIKeepsRecordsOffTheRow(t *testing.T) {
	lines := parseANSI("\x1b]1717;1\x07HEAD\n\x1b[32m+one\x1b[m\n\x1b]1717;1;d;2;2;/repo/f\x1b\\-two \x1b]1717;1;a;2;;f\x07+TWO\n")
	if len(lines) != 3 || lines[0].text != "HEAD" || lines[1].text != "+one" || lines[2].text != "-two +TWO" {
		t.Fatalf("texts: %+v", lines)
	}
	if lines[1].spans[0].style.fg != (ansiColor{kind: 1, idx: 2}) {
		t.Errorf("colour: %+v", lines[1].spans)
	}
	if len(lines[2].records) != 2 {
		t.Errorf("records: %q", lines[2].records)
	}
	d := parseRenderedDiff("\x1b]1717;1\x07HEAD\n\x1b]1717;1;d;2;2;/repo/f\x07-two \x1b]1717;1;a;2;;f\x07+TWO\n", "/repo")
	if !d.byRecord || d.rows[0].ok {
		t.Fatalf("rows: %+v", d.rows)
	}
	if r := d.rows[1]; !r.ok || r.path != "f" || r.kind != patch.DELETION || r.oldLine != 2 || len(d.alts[1]) != 2 {
		t.Errorf("row: %+v alts %+v", r, d.alts[1])
	}
}

func TestParseRenderedDiffReadsADiffWithoutRecords(t *testing.T) {
	d := parseRenderedDiff("RENDERED\n\x1b[1mdiff --git a/f b/f\x1b[m\nindex 1..2 100644\n--- a/f\n+++ b/f\n@@ -1 +1 @@\n\x1b[31m-a\x1b[m\n\x1b[32m+b\x1b[m\n", "/repo")
	if d.byRecord || d.rows[0].ok || !d.rows[6].isChange() || d.rows[7].kind != patch.ADDITION || d.rows[7].newLine != 1 {
		t.Fatalf("rows: %+v", d.rows)
	}
}
