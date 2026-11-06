package app

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
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
	cmd := a.subprocessCommand(a.repo.Git.Commit.CommitInEditorWithMessageFileCmdObj(path, skipHooks))
	a.run("Committing…", func() error {
		defer os.Remove(path)
		return a.runTerminal(cmd)
	}, nil)
}
