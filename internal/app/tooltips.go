package app

import (
	"reflect"

	"github.com/jesseduffield/lazygit/pkg/i18n"
)

// scopeContexts is the lazygit context whose bindings each scope mirrors.
var scopeContexts = map[scope]string{
	scopeGlobal: "", scopeStatus: "status", scopeFiles: "files", scopeBranches: "localBranches",
	scopeCommits: "commits", scopeCommitFiles: "commitFiles", scopeStash: "stash", scopeStaging: "main",
	scopeMain: "main", scopeRemotes: "remotes", scopeTags: "tags", scopeRefCommits: "subCommits",
	scopeRemoteBranches: "remoteBranches", scopeWorktrees: "worktrees", scopeSubmodules: "submodules",
	scopeReflog: "reflogCommits",
}

// guideTooltips explains the review guide's actions whose label doesn't say
// enough on its own.
var guideTooltips = map[string]string{
	"Git":                    "Leave the review guide for the git panels",
	"Expand / collapse note": "Show or hide how the section works, below why it exists",
	"Comment":                "Write a private draft comment on the selected lines. Send review hands drafts to your agent",
	"Feedback":               "List this review's comment threads and the agent's replies",
	"Send review":            "Send your draft comments to your agent through MCP and copy a prompt asking it to answer them. Nothing is posted to a pull request",
	"Commit":                 "Close the guide and commit the working tree with the Files panel's commit flow",
	"Publish to GitHub PR":   "Add your comments to a pending review on the branch's GitHub pull request. Nobody sees it until you submit it there",
	"Diagram / code":         "Switch between the section's diagram and its code",
	"Jump to code":           "Show the code the selected shape links to",
	"Baseline / proposal":    "Switch between the agent's diagram, read-only, and your edited copy",
	"Reviewed":               "Mark the selected hunk reviewed, or unmark it. A hunk that changes afterwards is unread again",
	"Next unread":            "Jump to the next hunk you haven't reviewed",
	"Previous unread":        "Jump to the previous hunk you haven't reviewed",
	"Guide status details":   "Show the guide's full error, notice or source",
	"Copy MCP config":        "Copy the MCP server config that lets your agent read this review and publish a guide",
	"Copy agent prompt":      "Copy a prompt asking your agent to write a review guide for this diff",
	"Base":                   "Review a branch's commits from its merge base to HEAD, or leave the base empty to review the working tree",
	"Select range":           "Start or end a range of lines to comment on",
	"Read code":              "Move from the story into the selected hunk's code",
	"Reload":                 "Read the diff and the agent's guide again",
}

// lazygitTips is lazygitTooltips in the user's language: lazygit context →
// binding description → tooltip.
func lazygitTips(tr *i18n.TranslationSet) map[string]map[string]string {
	v := reflect.ValueOf(tr).Elem()
	tips := make(map[string]map[string]string, len(lazygitTooltips))
	for ctx, pairs := range lazygitTooltips {
		m := make(map[string]string, len(pairs))
		for _, p := range pairs {
			label := v.FieldByName(p[0]).String()
			if _, ok := m[label]; !ok {
				m[label] = v.FieldByName(p[1]).String()
			}
		}
		tips[ctx] = m
	}
	return tips
}

// contextTip is lazygit's tooltip for the binding labelled label in sc.
func (a *App) contextTip(sc scope, label string) string {
	if sc == scopeGuide || sc == scopeGuideDiagram {
		return guideTooltips[label]
	}
	if a.tips == nil {
		a.tips = lazygitTips(a.repo.Common.Tr)
	}
	return a.tips[scopeContexts[sc]][label]
}

// guideAct is the review guide's action labelled label.
func (a *App) guideAct(label string) action {
	for _, act := range a.actions {
		if (act.scope == scopeGuide || act.scope == scopeGuideDiagram) && act.label == label {
			return act
		}
	}
	return action{scope: scopeGuide, label: label}
}
