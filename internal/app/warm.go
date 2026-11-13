package app

import "github.com/egoist/mygo/ui"

// Warm draws each font the panels use, so a pre-launched process pays
// CoreText's first font lookups and shaping before a repository arrives.
func Warm(c *ui.Context) {
	const sample = "STATUS Files abc 0123 ↑↓ … ─│╮●"
	ui.Text(c, sample).FontSize(sizeCaption).FontWeight(650)
	ui.Text(c, sample).FontSize(sizeCaption)
	ui.Text(c, sample).FontSize(sizeBody)
	ui.Text(c, sample).FontSize(sizeBody).FontWeight(600)
	ui.Text(c, sample).FontSize(sizeTitle).FontWeight(600)
	ui.Text(c, sample).Font(monoFont).FontSize(12)
	ui.Text(c, sample).Font(monoFont).FontSize(sizeCaption)
	ui.Text(c, sample).Font(monoFont).FontSize(sizeBody)
}
