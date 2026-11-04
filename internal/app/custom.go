package app

import (
	"fmt"
	"strings"
	"text/template"

	"github.com/jesseduffield/lazygit/pkg/config"
	"github.com/jesseduffield/lazygit/pkg/utils"
	"laziergit/internal/keymap"
)

func (a *App) customActions() []action {
	contexts := map[string]scope{"global": scopeGlobal, "status": scopeStatus, "files": scopeFiles, "localBranches": scopeBranches, "commits": scopeCommits, "commitFiles": scopeCommitFiles, "stash": scopeStash}
	var acts []action
	for _, cmd := range a.repo.Config.CustomCommands {
		for _, ctx := range strings.Split(cmd.Context, ",") {
			if sc, ok := contexts[strings.TrimSpace(ctx)]; ok {
				cmd := cmd
				acts = append(acts, action{sc, keymap.Parse(cmd.Key), cmd.GetDescription(), func() { a.customFlow(cmd) }, false})
			}
		}
	}
	return acts
}

func (a *App) customFlow(cmd config.CustomCommand) {
	if len(cmd.CommandMenu) > 0 {
		var items []menuItem
		for _, child := range cmd.CommandMenu {
			child := child
			items = append(items, menuItem{label: child.GetDescription(), key: keymap.Parse(child.Key).LabelAll(), chord: keymap.Parse(child.Key), run: func() { a.customFlow(child) }})
		}
		a.pushOverlay(&menuOverlay{title: cmd.GetDescription(), items: items})
		return
	}
	// A terminal-owning command needs an external terminal integration.
	if cmd.Output == "terminal" {
		a.showError("Custom command", fmt.Errorf("terminal output is not supported; use popup or log for non-interactive commands"))
		return
	}
	form := map[string]string{}
	responses := []string{}
	data := map[string]any{"Form": form, "PromptResponses": responses, "SelectedLocalBranch": a.selectedBranch(), "SelectedStashEntry": a.selectedStash()}
	if c := a.selectedCommit(); c != nil {
		data["SelectedLocalCommit"] = map[string]any{"Hash": c.Hash(), "Sha": c.Hash(), "Name": c.Name}
	}
	if f := a.selectedFile(); f != nil {
		data["SelectedFile"] = map[string]any{"Name": f.Path, "Path": f.Path, "ShortStatus": f.ShortStatus}
	}
	resolve := func(s string) (string, error) {
		data["PromptResponses"] = responses
		return utils.ResolveTemplate(s, data, template.FuncMap{"quote": a.repo.OS.Quote, "runCommand": a.repo.Git.Custom.TemplateFunctionRunCommand})
	}
	var next func(int)
	next = func(i int) {
		if i == len(cmd.Prompts) {
			label := cmd.LoadingText
			if label == "" {
				label = "Running custom command…"
			}
			var output string
			a.run(label, func() error {
				text, err := resolve(cmd.Command)
				if err != nil {
					return err
				}
				output, err = a.repo.RunCustom(text)
				return err
			}, func() {
				if cmd.Output == "popup" {
					title := cmd.OutputTitle
					if title == "" {
						title = cmd.GetDescription()
					}
					a.pushOverlay(&errorOverlay{title: title, body: output, neutral: true})
				} else if cmd.Output == "log" || cmd.Output == "logWithPty" {
					a.log = append(a.log, logEntry{text: output})
					a.showLog = true
				}
			})
			return
		}
		p := cmd.Prompts[i]
		a.bg(func() {
			title, e := resolve(p.Title)
			initial, e2 := resolve(p.InitialValue)
			body, e3 := resolve(p.Body)
			condition, e4 := resolve(p.Condition)
			a.post(func() {
				for _, err := range []error{e, e2, e3, e4} {
					if err != nil {
						a.showError("Custom command", err)
						return
					}
				}
				if p.Condition != "" && (condition == "" || condition == "false") {
					next(i + 1)
					return
				}
				accept := func(v string) { form[p.Key] = v; responses = append(responses, v); next(i + 1) }
				switch p.Type {
				case "input":
					a.prompt(title, initial, body, accept)
				case "confirm":
					a.confirm(title, body, false, func() { accept("true") })
				case "menu":
					var items []menuItem
					for _, option := range p.Options {
						option := option
						label := option.Name
						if label == "" {
							label = option.Value
						}
						items = append(items, menuItem{label: label, run: func() { accept(option.Value) }})
					}
					a.pushOverlay(&menuOverlay{title: title, items: items})
				default:
					a.showError("Custom command", fmt.Errorf("unsupported prompt type: %s", p.Type))
				}
			})
		})
	}
	next(0)
}
