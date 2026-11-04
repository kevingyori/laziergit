package app

import "github.com/egoist/mygo/ui"

// Palette holds semantic tokens. Chrome is neutral; saturated colour is
// reserved for meaning (state, diff, focus, graph lanes).
type Palette struct {
	Bg, Panel, Header, Hairline     ui.Color
	Text, Muted, Faint              ui.Color
	Accent, AccentSoft, Hover, Idle ui.Color

	Staged, Unstaged, Untracked, Conflict ui.Color
	Unpushed, Pushed, Merged              ui.Color

	AddBg, AddText, DelBg, DelText, HunkBg, HunkText, Meta ui.Color

	Lanes []ui.Color // commit graph lanes
}

const (
	monoFont = "SF Mono, JetBrains Mono, Menlo, monospace"

	sizeCaption   = 11 // section headers, key caps
	sizeBody      = 13 // rows
	sizeTitle     = 16 // detail header subject
	rowHeight     = 22
	diffRowHeight = 18
)

func palette(dark bool) *Palette {
	if dark {
		return &Palette{
			Bg: ui.Hex("#141518"), Panel: ui.Hex("#18191d"), Header: ui.Hex("#1c1d22"), Hairline: ui.Hex("#2a2c32"),
			Text: ui.Hex("#e7e7ea"), Muted: ui.Hex("#9a9da6"), Faint: ui.Hex("#62656d"),
			Accent: ui.Hex("#7aa7ff"), AccentSoft: ui.Hex("#7aa7ff2e"), Hover: ui.Hex("#ffffff0a"), Idle: ui.Hex("#ffffff12"),
			Staged: ui.Hex("#5fc27e"), Unstaged: ui.Hex("#e5b45a"), Untracked: ui.Hex("#9a9da6"), Conflict: ui.Hex("#f0716f"),
			Unpushed: ui.Hex("#e5b45a"), Pushed: ui.Hex("#9a9da6"), Merged: ui.Hex("#5fc27e"),
			AddBg: ui.Hex("#5fc27e1f"), AddText: ui.Hex("#a6e3b5"), DelBg: ui.Hex("#f0716f1f"), DelText: ui.Hex("#f5a9a8"),
			HunkBg: ui.Hex("#7aa7ff14"), HunkText: ui.Hex("#9dbcff"), Meta: ui.Hex("#62656d"),
			Lanes: []ui.Color{ui.Hex("#7aa7ff"), ui.Hex("#5fc27e"), ui.Hex("#e5b45a"), ui.Hex("#e58bc4"),
				ui.Hex("#5cc8d6"), ui.Hex("#b394f5"), ui.Hex("#f0916f")},
		}
	}
	return &Palette{
		Bg: ui.Hex("#f6f6f7"), Panel: ui.Hex("#ffffff"), Header: ui.Hex("#fafafb"), Hairline: ui.Hex("#e3e4e8"),
		Text: ui.Hex("#1d1e22"), Muted: ui.Hex("#5f626b"), Faint: ui.Hex("#9a9ca3"),
		Accent: ui.Hex("#2f6bff"), AccentSoft: ui.Hex("#2f6bff22"), Hover: ui.Hex("#0000000a"), Idle: ui.Hex("#00000010"),
		Staged: ui.Hex("#1f8f4a"), Unstaged: ui.Hex("#b27400"), Untracked: ui.Hex("#5f626b"), Conflict: ui.Hex("#d23c3a"),
		Unpushed: ui.Hex("#b27400"), Pushed: ui.Hex("#5f626b"), Merged: ui.Hex("#1f8f4a"),
		AddBg: ui.Hex("#1f8f4a1a"), AddText: ui.Hex("#17663a"), DelBg: ui.Hex("#d23c3a1a"), DelText: ui.Hex("#9c2422"),
		HunkBg: ui.Hex("#2f6bff10"), HunkText: ui.Hex("#2f5bd0"), Meta: ui.Hex("#9a9ca3"),
		Lanes: []ui.Color{ui.Hex("#2f6bff"), ui.Hex("#1f8f4a"), ui.Hex("#b27400"), ui.Hex("#c2418f"),
			ui.Hex("#0f8a9a"), ui.Hex("#7a4fe0"), ui.Hex("#d0572c")},
	}
}
