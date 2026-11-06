package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
	"github.com/jesseduffield/lazygit/pkg/gocui"
	"github.com/samber/lo"
)

// commitMenu is lazygit's CommitsHelper.OpenCommitMenu.
func (a *App) commitMenu(o *commitOverlay, field *ui.Handle) {
	tr := a.repo.Common.Tr
	o.refocus = field
	open := menuItem{label: tr.OpenInEditor, key: "e", chord: runeBind('e'), run: func() { a.switchCommitToEditor(o) }}
	if o.onSwitchToEditor == nil {
		open.disabled = tr.CommandDoesNotSupportOpeningInEditor
	}
	a.pushOverlay(&menuOverlay{title: tr.CommitMenuTitle, items: []menuItem{
		open,
		{label: tr.AddCoAuthor, key: "c", chord: runeBind('c'), run: func() {
			a.promptSuggestions(tr.AddCoAuthorPromptTitle, "", "", "authors", func(value string) {
				o.draft.description = git_commands.AddCoAuthorToDescription(o.draft.description, value)
			})
		}},
		{label: tr.PasteCommitMessageFromClipboard, key: "p", chord: runeBind('p'), run: func() { a.pasteCommitMessage(o) }},
	}})
}

// switchCommitToEditor is lazygit's CommitsHelper.SwitchToEditor.
func (a *App) switchCommitToEditor(o *commitOverlay) {
	message := lo.Ternary(o.draft.description == "", o.draft.summary, o.draft.summary+"\n\n"+o.draft.description)
	dir := filepath.Join(a.repo.OS.GetTempDir(), a.repo.Git.RepoPaths.RepoName())
	path := filepath.Join(dir, time.Now().Format("Jan _2 15.04.05.000000000")+".msg")
	if err := a.repo.OS.CreateFileWithContent(path, message); err != nil {
		a.errorPopup(err.Error())
		return
	}
	a.closeOverlay(o)
	o.onSwitchToEditor(path)
}

// pasteCommitMessage is lazygit's CommitsHelper.pasteCommitMessageFromClipboard.
func (a *App) pasteCommitMessage(o *commitOverlay) {
	tr := a.repo.Common.Tr
	message, err := a.repo.OS.PasteFromClipboard()
	if err != nil {
		a.errorPopup(err.Error())
		return
	}
	if message == "" {
		return
	}
	set := func() {
		summary, description, _ := strings.Cut(message, "\n")
		o.draft.summary, o.draft.description = summary, strings.TrimPrefix(description, "\n")
	}
	if o.draft.summary == "" && o.draft.description == "" {
		set()
		return
	}
	a.confirm(tr.PasteCommitMessageFromClipboard, tr.SurePasteCommitMessage, false, set)
}

// commitInEditor runs git commit --edit on a message file in a terminal,
// lazygit's switchFromCommitMessagePanelToEditor.
func (a *App) commitInEditor(path string, skipHooks bool) {
	a.draft = commitDraft{}
	a.setPreservedCommitMessage("")
	cmd := a.subprocessCommand(a.repo.Git.Commit.CommitInEditorWithMessageFileCmdObj(path, skipHooks))
	a.run("Committing…", func() error {
		defer os.Remove(path)
		return a.runTerminal(cmd)
	}, nil)
}

// commitHistory is lazygit's CommitMessageController.handleCommitIndexChange:
// <up>/<down> in the summary walk through earlier commit messages.
func (a *App) commitHistory(o *commitOverlay, delta int) {
	current := o.hist - 1 // lazygit's selected index; -1 is NoCommitIndex
	if delta < 0 && current == -1 {
		return
	}
	next := current + delta
	if next == -1 {
		o.hist = 0
		o.draft.summary, o.draft.description, _ = strings.Cut(o.historyMessage, "\n")
		o.gen++
		return
	}
	if current == -1 {
		o.historyMessage = o.draft.summary
		if o.draft.description != "" {
			o.historyMessage += "\n" + o.draft.description
		}
	}
	cfg := a.repo.Config.Git.Commit
	a.bg(func() {
		msg, err := a.repo.Git.Commit.GetCommitMessageFromHistory(next)
		a.post(func() {
			if err != nil {
				if !errors.Is(err, git_commands.ErrInvalidCommitIndex) {
					a.errorPopup(a.repo.Common.Tr.CommitWithoutMessageErr)
				}
				return
			}
			if cfg.AutoWrapCommitMessage {
				msg = tryRemoveHardLineBreaks(msg, cfg.AutoWrapWidth)
			}
			summary, description, _ := strings.Cut(msg, "\n")
			o.draft.summary, o.draft.description = summary, strings.TrimPrefix(description, "\n")
			o.hist = next + 1
			o.gen++
		})
	})
}

// tryRemoveHardLineBreaks is lazygit's helpers.TryRemoveHardLineBreaks.
func tryRemoveHardLineBreaks(message string, autoWrapWidth int) string {
	lastHardLineStart := 0
	result := message
	for i, b := range message {
		if b == '\n' {
			str := message[lastHardLineStart:i] + " " + message[i+1:]
			soft := gocui.AutoWrapContent(str, autoWrapWidth)
			if len(soft) > 0 && soft[0] == i-lastHardLineStart+1 {
				result = result[:i] + " " + result[i+1:]
			}
			lastHardLineStart = i + 1
		}
	}
	return result
}

// wrappedDescription is the description as lazygit's auto-wrapping text
// area holds it: soft line breaks at git.commit.autoWrapWidth become line
// feeds, which is also what gets committed.
func (o *commitOverlay) wrappedDescription(a *App) string {
	cfg := a.repo.Config.Git.Commit
	d := o.draft.description
	if !cfg.AutoWrapCommitMessage {
		return d
	}
	soft := gocui.AutoWrapContent(d, cfg.AutoWrapWidth)
	for i := len(soft) - 1; i >= 0; i-- {
		d = d[:soft[i]] + "\n" + d[soft[i]:]
	}
	return d
}
