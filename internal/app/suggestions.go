package app

import (
	"slices"
	"strings"

	"github.com/jesseduffield/lazygit/pkg/utils"
)

func (o *promptOverlay) matches() []string {
	if o.suggestions == nil {
		return nil
	}
	items := o.suggestions(o.value)
	o.suggestionCursor = clamp(o.suggestionCursor, 0, len(items)-1)
	return items
}

func (a *App) suggestionFilter(values []string) func(string) []string {
	return func(input string) []string {
		if input == "" {
			return values
		}
		return utils.FilterStrings(input, values, a.repo.Config.Gui.UseFuzzySearch())
	}
}

func (a *App) suggestionValues(preset string) []string {
	var out []string
	if a.raw == nil {
		return out
	}
	if preset == "branches" || preset == "refs" || preset == "worktree" {
		for _, b := range a.raw.Branches {
			eligible := true
			if preset == "worktree" {
				for _, w := range a.raw.Worktrees {
					if w.Branch == b.Name {
						eligible = false
					}
				}
			}
			if eligible {
				out = append(out, b.Name)
			}
		}
	}
	for _, r := range a.raw.Remotes {
		if preset == "remotes" {
			out = append(out, r.Name)
		}
		if preset == "remoteBranches" || preset == "refs" || preset == "upstream" || preset == "worktree" || strings.HasPrefix(preset, "remote:") {
			for _, b := range r.Branches {
				value := r.Name + "/" + b.Name
				if preset == "upstream" {
					value = r.Name + " " + b.Name
				}
				if strings.HasPrefix(preset, "remote:") {
					if r.Name != strings.TrimPrefix(preset, "remote:") {
						continue
					}
					value = b.Name
				}
				if preset == "worktree" {
					exists := false
					for _, local := range a.raw.Branches {
						if local.Name == b.Name {
							exists = true
						}
					}
					if exists {
						continue
					}
				}
				out = append(out, value)
			}
		}
	}
	if preset == "tags" || preset == "refs" {
		for _, t := range a.raw.Tags {
			out = append(out, t.Name)
		}
	}
	if preset == "refs" {
		out = append(out, "HEAD", "FETCH_HEAD", "MERGE_HEAD", "ORIG_HEAD")
	}
	if preset == "files" {
		for _, f := range a.raw.Files {
			out = append(out, f.Path)
		}
	}
	if preset == "authors" {
		for _, author := range a.authors {
			out = append(out, author)
		}
		slices.Sort(out)
	}
	return out
}

func (a *App) promptSuggestions(title, initial, hint, preset string, submit func(string)) {
	a.prompt(title, initial, hint, submit)
	a.topOverlay().(*promptOverlay).suggestions = a.suggestionFilter(a.suggestionValues(preset))
}
