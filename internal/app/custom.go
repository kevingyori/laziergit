package app

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	"github.com/jesseduffield/lazygit/pkg/config"
	"github.com/jesseduffield/lazygit/pkg/gui/style"
	"github.com/jesseduffield/lazygit/pkg/utils"
	"laziergit/internal/keymap"
)

func (a *App) customActions() []action {
	var acts []action
	for _, cmd := range a.repo.Config.CustomCommands {
		scopes := a.customActionsForContext(cmd)
		if len(cmd.CommandMenu) > 0 {
			scopes = []scope{scopeGlobal}
		}
		for _, sc := range scopes {
			cmd := cmd
			acts = append(acts, action{sc, keymap.Parse(cmd.Key), cmd.GetDescription(), func() { a.customFlow(cmd) }, false})
		}
	}
	return acts
}

func (a *App) customFlow(cmd config.CustomCommand) {
	if len(cmd.CommandMenu) > 0 {
		var items []menuItem
		for _, child := range cmd.CommandMenu {
			child := child
			allowed := child.Context == "" || child.Context == "global"
			for _, act := range a.customActionsForContext(child) {
				if act == a.scope() {
					allowed = true
				}
			}
			if !allowed {
				continue
			}
			label := child.GetDescription()
			binding := keymap.Parse(child.Key)
			if binding.LabelAll() == a.keys.confirmMenu.LabelAll() || binding.LabelAll() == "↓" || binding.LabelAll() == "↑" {
				binding = keymap.Parse(nil)
			}
			items = append(items, menuItem{label: label, key: binding.LabelAll(), chord: binding, run: func() { a.customFlow(child) }})
		}
		a.pushOverlay(&menuOverlay{title: cmd.GetDescription(), items: items, itemKeysFirst: true})
		return
	}
	selectedBranch := ""
	if b := a.selectedBranch(); b != nil {
		selectedBranch = b.Name
	}
	form := map[string]string{}
	for _, p := range cmd.Prompts {
		form[p.Key] = ""
	}
	responses := []string{}
	data := struct {
		*customSessionState
		Form            map[string]string
		PromptResponses []string
	}{a.customSession(), form, responses}
	resolve := func(s string) (string, error) {
		data.PromptResponses = responses
		return utils.ResolveTemplate(s, data, template.FuncMap{"quote": a.repo.OS.Quote, "runCommand": a.repo.Git.Custom.TemplateFunctionRunCommand})
	}
	var next func(int)
	next = func(i int) {
		if i == len(cmd.Prompts) {
			label := cmd.LoadingText
			if label == "" {
				label = a.repo.Common.Tr.RunningCustomCommandStatus
			}
			var output, text string
			a.run(label, func() error {
				var err error
				text, err = resolve(cmd.Command)
				if err != nil {
					return err
				}
				if cmd.Output == "terminal" {
					return a.runTerminal(text)
				}
				output, err = a.repo.RunCustomOutput(text, cmd.Output)
				return err
			}, func() {
				if selectedBranch != "" {
					for i, b := range a.snap.Branches {
						if b.Name == selectedBranch {
							a.cursor[panelBranches] = i
							a.lists[panelBranches].ScrollIntoView(i)
							a.loadMain()
							break
						}
					}
				}
				if cmd.Output == "popup" {
					title, err := resolve(cmd.OutputTitle)
					if cmd.OutputTitle == "" {
						title, err = text, nil
					}
					if err != nil {
						a.showError("Custom command", err)
						return
					}
					if strings.TrimSpace(output) == "" {
						output = a.repo.Common.Tr.EmptyOutput
					}
					a.alert(title, output)
				} else if cmd.Output == "log" || cmd.Output == "logWithPty" {
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
			options := append([]config.CustomCommandMenuOption(nil), p.Options...)
			var menuErr error
			preset, presetErr := resolve(p.Suggestions.Preset)
			var suggestions []string
			if preset != "" && p.Suggestions.Command != "" {
				menuErr = fmt.Errorf("Custom command prompt cannot have both a preset and a command for suggestions. Preset: '%s', Command: '%s'", preset, p.Suggestions.Command)
			}
			if preset != "" && !strings.Contains("|authors|branches|files|refs|remotes|remoteBranches|tags|", "|"+preset+"|") {
				menuErr = fmt.Errorf("Unknown value for suggestionsPreset in custom command: %s. Valid values: files, branches, remotes, remoteBranches, refs", preset)
			}
			if menuErr == nil && p.Type == "input" && (preset == "files" || preset == "authors") {
				command := "git ls-files --cached --others --exclude-standard"
				if preset == "authors" {
					command = "git log --all --format='%aN <%aE>'"
				}
				var output string
				output, menuErr = a.repo.RunCustom(command)
				seen := map[string]bool{}
				for _, value := range strings.Split(strings.TrimSpace(output), "\n") {
					if value != "" && !seen[value] {
						suggestions = append(suggestions, value)
						seen[value] = true
					}
				}
			}
			if menuErr == nil && p.Type == "input" && p.Suggestions.Command != "" {
				var command string
				command, menuErr = resolve(p.Suggestions.Command)
				if menuErr == nil {
					var output string
					output, menuErr = a.repo.RunCustom(command)
					if menuErr == nil {
						if output != "" {
							suggestions = strings.Split(strings.TrimSuffix(output, "\n"), "\n")
						}
					}
				}
			}
			if p.Type == "menu" {
				for j := range options {
					options[j].Name, menuErr = resolve(options[j].Name)
					if menuErr != nil {
						break
					}
					options[j].Value, menuErr = resolve(options[j].Value)
					if menuErr != nil {
						break
					}
				}
			}
			if p.Type == "menuFromCommand" && (p.Condition == "" || condition != "" && condition != "false") {
				command, err := resolve(p.Command)
				menuErr = err
				if err == nil {
					output, err := a.repo.RunCustom(command)
					menuErr = err
					if err == nil {
						filter, err := resolve(p.Filter)
						menuErr = err
						if err == nil {
							options, menuErr = commandOptions(output, filter, p.ValueFormat, p.LabelFormat)
						}
					}
				}
			}
			a.post(func() {
				for _, err := range []error{e, e2, e3, e4, presetErr, menuErr} {
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
					if preset != "" && preset != "files" && preset != "authors" {
						suggestions = a.suggestionValues(preset)
					}
					o := a.topOverlay().(*promptOverlay)
					if preset != "" {
						o.suggestions = a.suggestionFilter(suggestions)
					}
					if p.Suggestions.Command != "" {
						o.suggestions = func(input string) []string {
							var matches []string
							for _, value := range suggestions {
								if strings.Contains(strings.ToLower(value), strings.ToLower(input)) {
									matches = append(matches, value)
								}
							}
							return matches
						}
					}
				case "confirm":
					a.confirm(title, body, false, func() { accept("true") })
				case "menu", "menuFromCommand":
					var items []menuItem
					for _, option := range options {
						option := option
						label := option.Name
						if label == "" {
							label = option.Value
						}
						items = append(items, menuItem{label: label, key: keymap.Parse(option.Key).LabelAll(), chord: keymap.Parse(option.Key), run: func() { accept(option.Value) }})
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

func commandOptions(output, filter, valueFormat, labelFormat string) ([]config.CustomCommandMenuOption, error) {
	var options []config.CustomCommandMenuOption
	re, err := regexp.Compile(filter)
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		label, value := line, line
		if filter != "" || valueFormat != "" || labelFormat != "" {
			groups := map[string]string{}
			if match := re.FindStringSubmatch(line); match != nil {
				for i, v := range match {
					groups["group_"+strconv.Itoa(i)] = v
					if name := re.SubexpNames()[i]; name != "" {
						groups[name] = v
					}
				}
			}
			value, err = utils.ResolveTemplate(valueFormat, groups, nil)
			if err != nil {
				return nil, err
			}
			label = value
			if labelFormat != "" {
				label, err = utils.ResolveTemplate(labelFormat, groups, style.TemplateFuncMapAddColors(template.FuncMap{}))
				if err != nil {
					return nil, err
				}
			}
		}
		options = append(options, config.CustomCommandMenuOption{Name: strings.TrimSpace(label), Value: strings.TrimSpace(value)})
	}
	return options, nil
}

func (a *App) customActionsForContext(cmd config.CustomCommand) []scope {
	contexts := map[string]scope{"global": scopeGlobal, "files": scopeFiles, "commits": scopeCommits, "localBranches": scopeBranches, "remotes": scopeRemotes, "remoteBranches": scopeRemoteBranches, "tags": scopeTags, "subCommits": scopeRefCommits, "reflogCommits": scopeReflog, "commitFiles": scopeCommitFiles, "stash": scopeStash, "worktrees": scopeWorktrees, "submodules": scopeSubmodules, "status": scopeStatus}
	var result []scope
	for _, ctx := range strings.Split(cmd.Context, ",") {
		if sc, ok := contexts[strings.TrimSpace(ctx)]; ok {
			result = append(result, sc)
		}
	}
	return result
}
