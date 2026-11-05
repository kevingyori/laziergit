package app

import (
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/utils"
	"github.com/samber/lo"
)

// selectedTags is the tags context's GetSelectedItems.
func (a *App) selectedTags() []*models.Tag {
	if a.snap == nil || a.focus != panelBranches || a.branchTab != 2 || a.browse(panelBranches) != nil || a.count(panelBranches) == 0 {
		return nil
	}
	start, end := a.selectionRange(panelBranches)
	return a.snap.Tags[start : end+1]
}

func tagNames(tags []*models.Tag) []string {
	return lo.Map(tags, func(t *models.Tag, _ int) string { return t.Name })
}

// checkoutTag is lazygit's TagsController.checkout.
func (a *App) checkoutTag() {
	tags := a.selectedTags()
	if len(tags) != 1 {
		return
	}
	a.checkoutRef(tags[0].FullRefName())
}

// newTagFlow is lazygit's TagsController.create.
func (a *App) newTagFlow() {
	a.openCreateTagPrompt("", func() {
		a.cancelRange(panelBranches)
		a.cursor[panelBranches] = 0
		a.lists[panelBranches].ScrollTo(0, 0)
	})
}

// pushTagFlow is lazygit's TagsController.push.
func (a *App) pushTagFlow() {
	tags := a.selectedTags()
	if len(tags) == 0 {
		return
	}
	tr := a.repo.Common.Tr
	title := tr.PushTagsTitle
	if len(tags) == 1 {
		title = utils.ResolvePlaceholderString(tr.PushTagTitle, map[string]string{"tagName": tags[0].Name})
	}
	names := tagNames(tags)
	a.promptSuggestions(title, "origin", "", "remotes", func(remote string) {
		a.run(tr.PushingStatus, func() error { return a.repo.PushTags(remote, names) }, nil)
	})
}

func (a *App) deleteTagsTitle(tags []*models.Tag) string {
	tr := a.repo.Common.Tr
	if len(tags) > 1 {
		return tr.DeleteTagsTitle
	}
	return utils.ResolvePlaceholderString(tr.DeleteTagTitle, map[string]string{"tagName": tags[0].Name})
}

// deleteTagsMenu is lazygit's TagsController.delete.
func (a *App) deleteTagsMenu() {
	tags := a.selectedTags()
	if len(tags) == 0 {
		return
	}
	tr := a.repo.Common.Tr
	many := len(tags) > 1
	names := tagNames(tags)
	item := func(label string, r rune, run func()) menuItem {
		return menuItem{label: label, key: string(r), chord: runeBind(r), run: run}
	}
	afterLocal := func() {
		if a.selectingRange(panelBranches) {
			a.collapseRangeToTop(panelBranches)
		}
	}
	a.pushOverlay(&menuOverlay{title: a.deleteTagsTitle(tags), items: []menuItem{
		item(lo.Ternary(many, tr.DeleteLocalTags, tr.DeleteLocalTag), 'c', func() {
			a.run(tr.DeletingStatus, func() error { return a.repo.DeleteLocalTags(names) }, afterLocal)
		}),
		item(lo.Ternary(many, tr.DeleteRemoteTags, tr.DeleteRemoteTag), 'r', func() {
			a.confirmRemoteTagDelete(tags, lo.Ternary(many, tr.DeleteRemoteTagsPrompt, tr.DeleteRemoteTagPrompt), func(remote string) {
				a.run(tr.DeletingStatus, func() error { return a.repo.DeleteRemoteTags(remote, names) }, a.done(lo.Ternary(many, tr.RemoteTagsDeletedMessage, tr.RemoteTagDeletedMessage)))
			})
		}),
		item(lo.Ternary(many, tr.DeleteLocalAndRemoteTags, tr.DeleteLocalAndRemoteTag), 'b', func() {
			a.confirmRemoteTagDelete(tags, lo.Ternary(many, tr.DeleteLocalAndRemoteTagsPrompt, tr.DeleteLocalAndRemoteTagPrompt), func(remote string) {
				a.run(tr.DeletingStatus, func() error {
					if err := a.repo.DeleteRemoteTags(remote, names); err != nil {
						return err
					}
					return a.repo.DeleteLocalTags(names)
				}, afterLocal)
			})
		}),
	}})
}

// confirmRemoteTagDelete is lazygit's TagsController.confirmRemoteDelete.
func (a *App) confirmRemoteTagDelete(tags []*models.Tag, template string, del func(remote string)) {
	tr := a.repo.Common.Tr
	title := tr.SelectRemoteTagsUpstream
	if len(tags) == 1 {
		title = utils.ResolvePlaceholderString(tr.SelectRemoteTagUpstream, map[string]string{"tagName": tags[0].Name})
	}
	a.promptSuggestions(title, "origin", "", "remotes", func(remote string) {
		prompt := utils.ResolvePlaceholderString(template, map[string]string{"tagName": tags[0].Name, "upstream": remote})
		a.confirm(a.deleteTagsTitle(tags), prompt, true, func() { del(remote) })
	})
}
