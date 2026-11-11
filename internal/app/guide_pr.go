package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"laziergit/internal/reviewbridge"
)

func runGH(dir string, stdin []byte, args ...string) ([]byte, error) {
	cmd := exec.Command("gh", args...)
	cmd.Dir = dir
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil && stderr.Len() > 0 {
		err = fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
	}
	return out, err
}

type ghPR struct {
	Number      int    `json:"number"`
	URL         string `json:"url"`
	HeadRefOid  string `json:"headRefOid"`
	BaseRefName string `json:"baseRefName"`
}
type ghReviewComment struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Side string `json:"side"`
	Body string `json:"body"`
	// A multi-line comment starts here, on the same side.
	StartLine int    `json:"start_line,omitempty"`
	StartSide string `json:"start_side,omitempty"`
}

// prPlan splits feedback into what can go to a GitHub review and why the
// rest can't: diagram proposals stay local, outdated anchors aren't in the
// PR's diff, resolved and already published threads are done.
type prPlan struct {
	lines, notes                                     []reviewbridge.Feedback
	diagrams, outdated, resolved, published, working int
}

func (g *guideState) prPlan() prPlan {
	var p prPlan
	for _, item := range g.feedback.Items {
		switch {
		case item.Published != "":
			p.published++
		case item.Resolved:
			p.resolved++
		case item.Diagram != nil:
			p.diagrams++
		case item.Anchor == nil:
			p.notes = append(p.notes, item)
		case item.Placement == "outdated":
			p.outdated++
		case item.Anchor.Layer != "Branch":
			p.working++
		default:
			p.lines = append(p.lines, item)
		}
	}
	return p
}

// guidePublishPR creates a pending review on the branch's GitHub pull
// request: nothing is visible to others until the reviewer submits it there.
func (a *App) guidePublishPR() {
	g := a.guide
	a.loadGuideFeedback()
	if g.base == "" {
		g.notice = "PR comments need a branch review: press b and enter the PR's base branch"
		return
	}
	plan := g.prPlan()
	if len(plan.lines)+len(plan.notes) == 0 {
		g.notice = "Nothing to publish · comment on lines with c first"
		return
	}
	root, head, generation := g.worktree, g.context.NewTree, g.generation
	g.notice = "Looking up the pull request…"
	a.bg(func() {
		out, err := a.runGH(root, nil, "pr", "view", "--json", "number,url,headRefOid,baseRefName")
		var pr ghPR
		if err == nil {
			err = json.Unmarshal(out, &pr)
		}
		a.post(func() {
			if a.guide != g || g.generation != generation {
				return
			}
			g.notice = ""
			if err != nil {
				g.err = "GitHub: " + err.Error()
				return
			}
			if pr.HeadRefOid != head {
				g.err = fmt.Sprintf("Push first: PR #%d is at %.7s but this review is of %.7s", pr.Number, pr.HeadRefOid, head)
				return
			}
			body := fmt.Sprintf("Creates a pending review on PR #%d with %s and %s. Only you see it until you submit it on GitHub.", pr.Number, plural(len(plan.lines), "line comment"), plural(len(plan.notes), "general note"))
			var left []string
			for _, n := range []struct {
				count int
				what  string
			}{{plan.diagrams, "diagram proposals (stay local)"}, {plan.outdated, "outdated comments"}, {plan.working, "working-tree comments"}, {plan.resolved, "resolved threads"}, {plan.published, "already published"}} {
				if n.count > 0 {
					left = append(left, fmt.Sprintf("%d %s", n.count, n.what))
					if n.count == 1 {
						left[len(left)-1] = strings.Replace(left[len(left)-1], "s ", " ", 1)
					}
				}
			}
			if len(left) > 0 {
				body += "\n\nNot included: " + strings.Join(left, ", ") + "."
			}
			if base := strings.TrimPrefix(strings.TrimPrefix(g.base, "origin/"), "upstream/"); base != pr.BaseRefName {
				body += fmt.Sprintf("\n\nThis review is against %s but the PR targets %s; GitHub rejects lines outside the PR's diff.", g.base, pr.BaseRefName)
			}
			a.confirm("Publish to GitHub?", body, false, func() { a.createPRReview(pr, plan) })
		})
	})
}
func (a *App) createPRReview(pr ghPR, plan prPlan) {
	g := a.guide
	var notes []string
	for _, n := range plan.notes {
		notes = append(notes, n.Body)
	}
	payload := map[string]any{"commit_id": pr.HeadRefOid, "body": strings.Join(notes, "\n\n"), "comments": []ghReviewComment{}}
	var comments []ghReviewComment
	var ids []string
	for _, item := range append(plan.lines, plan.notes...) {
		ids = append(ids, item.ID)
	}
	for _, item := range plan.lines {
		an := item.CurrentAnchor
		if an == nil {
			an = item.Anchor
		}
		side := "RIGHT"
		if an.Side == "old" {
			side = "LEFT"
		}
		comment := ghReviewComment{Path: an.Path, Line: an.Line, Side: side, Body: item.Body}
		if an.StartLine > 0 {
			comment.StartLine, comment.StartSide = an.StartLine, side
		}
		comments = append(comments, comment)
	}
	if comments != nil {
		payload["comments"] = comments
	}
	data, _ := json.Marshal(payload)
	root, session := g.worktree, g.session
	g.notice = "Creating pending review…"
	a.bg(func() {
		out, err := a.runGH(root, data, "api", "--method", "POST", fmt.Sprintf("repos/{owner}/{repo}/pulls/%d/reviews", pr.Number), "--input", "-")
		var review struct {
			URL string `json:"html_url"`
		}
		if err == nil {
			err = json.Unmarshal(out, &review)
		}
		if err == nil {
			if review.URL == "" {
				review.URL = pr.URL
			}
			err = reviewbridge.MarkPublished(session, ids, review.URL)
		}
		a.post(func() {
			if a.guide != g {
				return
			}
			a.loadGuideFeedback()
			if err != nil {
				g.notice, g.err = "", "GitHub review: "+err.Error()
				return
			}
			g.err = ""
			g.notice = fmt.Sprintf("Pending review on PR #%d · submit it on GitHub", pr.Number)
			a.confirm("Pending review created", "Open it on GitHub to check and submit it?", false, func() {
				if err := a.openLink(review.URL); err != nil {
					g.err = err.Error()
				}
			})
		})
	})
}

func plural(n int, what string) string {
	if n == 1 {
		return "1 " + what
	}
	return fmt.Sprintf("%d %ss", n, what)
}
