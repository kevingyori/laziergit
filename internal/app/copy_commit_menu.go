package app

import (
	"fmt"
	"strings"

	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
)

// copyCommitAttributeMenu is lazygit's copyCommitAttribute (`y` in commits).
func (a *App) copyCommitAttributeMenu() {
	c := a.selectedCommit()
	if c == nil || a.commitFiles != nil {
		return
	}
	r := a.repo
	a.bg(func() {
		msg, err := r.Git.Commit.GetCommitMessage(c.Hash())
		a.post(func() {
			if a.repo != r {
				return
			}
			if err != nil {
				a.errorPopup(err.Error())
				return
			}
			a.pushCopyCommitAttributeMenu(c, msg)
		})
	})
}

func (a *App) pushCopyCommitAttributeMenu(c *models.Commit, msg string) {
	tr := a.repo.Common.Tr
	r := a.repo
	g := r.Git.Commit
	_, body, _ := strings.Cut(msg, "\n")
	body = strings.TrimPrefix(body, "\n")
	noBody, noTags := "", ""
	if body == "" {
		noBody = tr.CommitHasNoMessageBody
	}
	if len(c.Tags) == 0 {
		noTags = tr.CommitHasNoTags
	}
	// Git reads run in the background; clipboard access and status return to the UI.
	copy := func(toast string, text func() (string, error)) func() {
		return func() {
			a.bg(func() {
				s, err := text()
				a.post(func() {
					if a.repo != r {
						return
					}
					if err == nil {
						err = a.copyText(s)
					}
					if err != nil {
						a.errorPopup(err.Error())
						return
					}
					a.setStatus(toast, false)
				})
			})
		}
	}
	str := func(s string) func() (string, error) { return func() (string, error) { return s, nil } }
	item := func(label string, key rune, disabled string, run func()) menuItem {
		it := menuItem{label: label, disabled: disabled, run: run}
		if key != 0 {
			it.key, it.chord = string(key), runeBind(key)
		}
		return it
	}
	a.pushOverlay(&menuOverlay{title: tr.Actions.CopyCommitAttributeToClipboard, items: []menuItem{
		item(tr.CommitHash, 0, "", copy(fmt.Sprintf("'%s' %s", c.Hash(), tr.CopiedToClipboard), str(c.Hash()))),
		item(tr.CommitSubject, 's', "", copy(tr.CommitSubjectCopiedToClipboard, func() (string, error) { return g.GetCommitSubject(c.Hash()) })),
		item(tr.CommitMessage, 'm', "", copy(tr.CommitMessageCopiedToClipboard, func() (string, error) { return g.GetCommitMessage(c.Hash()) })),
		item(tr.CommitMessageBody, 'b', noBody, copy(tr.CommitMessageBodyCopiedToClipboard, str(body))),
		item(tr.CommitURL, 'u', "", copy(tr.CommitURLCopiedToClipboard, func() (string, error) { return r.Git.HostingService.GetCommitURL(c.Hash()) })),
		item(tr.CommitDiff, 'd', "", copy(tr.CommitDiffCopiedToClipboard, func() (string, error) { return g.GetCommitDiff(c.Hash()) })),
		item(tr.CommitAuthor, 'a', "", copy(tr.CommitAuthorCopiedToClipboard, func() (string, error) { return authorString(g.GetCommitAuthor(c.Hash())) })),
		item(tr.CommitTags, 't', noTags, copy(tr.CommitTagsCopiedToClipboard, str(strings.Join(c.Tags, "\n")))),
	}})
}

func authorString(au git_commands.Author, err error) (string, error) {
	return fmt.Sprintf("%s <%s>", au.Name, au.Email), err
}
