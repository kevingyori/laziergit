package reviewbridge

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Anchor struct {
	Path  string `json:"path"`
	Layer string `json:"layer"`
	Side  string `json:"side"`
	Line  int    `json:"line"`
	Hunk  string `json:"hunk_fingerprint"`
	Text  string `json:"text"`
}
type Reply struct {
	Kind  string `json:"kind"`
	Body  string `json:"body"`
	Agent string `json:"agent"`
}
type DiagramProposal struct {
	Section          string          `json:"section"`
	Baseline         Diagram         `json:"agent_baseline"`
	Proposed         Diagram         `json:"reviewer_proposed_changes"`
	Changes          []string        `json:"changes"`
	BaselineDocument json.RawMessage `json:"baseline_tldr,omitempty"`
	ProposedDocument json.RawMessage `json:"proposal_tldr,omitempty"`
}
type Feedback struct {
	Placement     string           `json:"placement,omitempty"`
	CurrentAnchor *Anchor          `json:"current_anchor,omitempty"`
	ID            string           `json:"id"`
	Kind          string           `json:"kind"`
	Revision      string           `json:"revision"`
	Round         int              `json:"round"`
	Anchor        *Anchor          `json:"anchor,omitempty"`
	Body          string           `json:"body"`
	Diagram       *DiagramProposal `json:"diagram,omitempty"`
	Status        string           `json:"status"`
	Batch         string           `json:"batch_id,omitempty"`
	Resolved      bool             `json:"resolved"`
	Replies       []Reply          `json:"replies,omitempty"`
	Updated       int              `json:"updated_cursor"`
}
type FeedbackState struct {
	Cursor   int        `json:"cursor"`
	Revision string     `json:"revision"`
	Round    int        `json:"round"`
	Items    []Feedback `json:"items"`
}
type feedbackEvent struct {
	Seq      int       `json:"seq"`
	At       string    `json:"at"`
	Type     string    `json:"type"`
	Revision string    `json:"revision,omitempty"`
	Item     *Feedback `json:"item,omitempty"`
	IDs      []string  `json:"ids,omitempty"`
	Batch    string    `json:"batch,omitempty"`
	Reply    *Reply    `json:"reply,omitempty"`
	Resolved bool      `json:"resolved,omitempty"`
}

func NewID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
func readEvents(dir string) (FeedbackState, int64, error) {
	state := FeedbackState{}
	f, err := os.Open(filepath.Join(dir, "events.jsonl"))
	if errors.Is(err, os.ErrNotExist) {
		return state, 0, nil
	}
	if err != nil {
		return state, 0, err
	}
	defer f.Close()
	reader := bufio.NewReader(f)
	var valid int64
	for {
		line, err := reader.ReadBytes('\n')
		if err == io.EOF {
			return state, valid, nil
		} // A torn final line is discarded on next append.
		if err != nil {
			return state, valid, err
		}
		var event feedbackEvent
		if err := json.Unmarshal(line, &event); err != nil {
			return state, valid, fmt.Errorf("review journal: %w", err)
		}
		if event.Seq != state.Cursor+1 {
			return state, valid, fmt.Errorf("review journal sequence gap")
		}
		state.Cursor = event.Seq
		valid += int64(len(line))
		switch event.Type {
		case "round":
			state.Revision = event.Revision
			state.Round++
		case "draft":
			if event.Item != nil {
				found := false
				for i, item := range state.Items {
					if item.ID == event.Item.ID {
						state.Items[i] = *event.Item
						found = true
						break
					}
				}
				if !found {
					state.Items = append(state.Items, *event.Item)
				}
			}
		case "delete":
			for i, item := range state.Items {
				if len(event.IDs) > 0 && item.ID == event.IDs[0] {
					state.Items = append(state.Items[:i], state.Items[i+1:]...)
					break
				}
			}
		default:
			for i := range state.Items {
				item := &state.Items[i]
				for _, id := range event.IDs {
					if item.ID != id {
						continue
					}
					item.Updated = event.Seq
					switch event.Type {
					case "send":
						item.Status = "sent"
						item.Batch = event.Batch
					case "reply":
						if event.Reply != nil {
							item.Replies = append(item.Replies, *event.Reply)
							item.Status = event.Reply.Kind
							if event.Reply.Kind == ReviewerReply {
								// A reviewer reply hands the thread back to agents.
								item.Status = "sent"
							}
						}
					case "resolve":
						item.Resolved = event.Resolved
					}
				}
			}
		}
	}
}
func LoadFeedback(dir string) (FeedbackState, error) {
	var state FeedbackState
	err := withStoreLock(dir, func() error { var e error; state, _, e = readEvents(dir); return e })
	if err == nil {
		var snapshot Snapshot
		if Read(dir, "snapshot.json", &snapshot) == nil {
			for i := range state.Items {
				state.Items[i].Placement, state.Items[i].CurrentAnchor = PlaceFeedback(state.Items[i], snapshot)
			}
		}
	}
	return state, err
}
func appendEvent(dir string, prepare func(FeedbackState) (feedbackEvent, error)) error {
	return withStoreLock(dir, func() error {
		state, valid, err := readEvents(dir)
		if err != nil {
			return err
		}
		event, err := prepare(state)
		if err != nil {
			return err
		}
		if event.Type == "" {
			return nil
		}
		event.Seq = state.Cursor + 1
		event.At = time.Now().UTC().Format(time.RFC3339Nano)
		b, err := json.Marshal(event)
		if err != nil {
			return err
		}
		b = append(b, '\n')
		f, err := os.OpenFile(filepath.Join(dir, "events.jsonl"), os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		defer f.Close()
		if err = f.Truncate(valid); err != nil {
			return err
		}
		if _, err = f.Seek(valid, io.SeekStart); err != nil {
			return err
		}
		if _, err = f.Write(b); err != nil {
			return err
		}
		return f.Sync()
	})
}
func SaveRound(dir string, s Snapshot) error {
	if err := os.MkdirAll(filepath.Join(dir, "rounds"), 0700); err != nil {
		return err
	}
	// Revisions are hashes, never caller-supplied paths.
	if len(s.Revision) != 64 {
		return fmt.Errorf("invalid snapshot revision")
	}
	if _, err := hex.DecodeString(s.Revision); err != nil {
		return err
	}
	if err := Write(filepath.Join(dir, "rounds"), s.Revision+".json", s); err != nil {
		return err
	}
	return appendEvent(dir, func(state FeedbackState) (feedbackEvent, error) {
		if state.Revision == s.Revision {
			return feedbackEvent{}, nil
		}
		return feedbackEvent{Type: "round", Revision: s.Revision}, nil
	})
}
func SaveDraft(dir string, item Feedback) error {
	if strings.TrimSpace(item.Body) == "" && item.Diagram == nil {
		return fmt.Errorf("feedback cannot be empty")
	}
	if len(item.Body) > 16000 {
		return fmt.Errorf("feedback is too long")
	}
	return appendEvent(dir, func(state FeedbackState) (feedbackEvent, error) {
		if item.Revision != state.Revision {
			return feedbackEvent{}, fmt.Errorf("snapshot changed; reload before editing")
		}
		if item.ID == "" {
			item.ID = NewID()
		}
		for _, old := range state.Items {
			if old.ID == item.ID && old.Status != "draft" {
				return feedbackEvent{}, fmt.Errorf("sent feedback is immutable")
			}
		}
		item.Round = state.Round
		item.Status = "draft"
		item.Updated = state.Cursor + 1
		return feedbackEvent{Type: "draft", Item: &item}, nil
	})
}
func DeleteDraft(dir, id string) error {
	return appendEvent(dir, func(state FeedbackState) (feedbackEvent, error) {
		for _, item := range state.Items {
			if item.ID == id && item.Status == "draft" {
				return feedbackEvent{Type: "delete", IDs: []string{id}}, nil
			}
		}
		return feedbackEvent{}, fmt.Errorf("draft not found")
	})
}
func SendFeedback(dir string) error {
	return appendEvent(dir, func(state FeedbackState) (feedbackEvent, error) {
		var ids []string
		for _, item := range state.Items {
			if item.Status == "draft" {
				if item.Revision != state.Revision {
					var snapshot Snapshot
					if err := Read(dir, "snapshot.json", &snapshot); err != nil {
						return feedbackEvent{}, err
					}
					if placement, _ := PlaceFeedback(item, snapshot); placement == "outdated" {
						return feedbackEvent{}, fmt.Errorf("outdated drafts need re-anchoring or deletion before sending")
					}
				}
				ids = append(ids, item.ID)
			}
		}
		if len(ids) == 0 {
			return feedbackEvent{}, fmt.Errorf("no drafts to send")
		}
		return feedbackEvent{Type: "send", IDs: ids, Batch: NewID()}, nil
	})
}
func ReplyFeedback(dir, id string, reply Reply) error {
	if reply.Kind != "acknowledged" && reply.Kind != "addressed" && reply.Kind != "question" && reply.Kind != "declined" {
		return fmt.Errorf("kind must be acknowledged, addressed, question or declined")
	}
	if strings.TrimSpace(reply.Body) == "" || len(reply.Body) > 16000 {
		return fmt.Errorf("reply requires a bounded body")
	}
	return appendEvent(dir, func(state FeedbackState) (feedbackEvent, error) {
		for _, item := range state.Items {
			if item.ID == id && item.Status != "draft" {
				return feedbackEvent{Type: "reply", IDs: []string{id}, Reply: &reply}, nil
			}
		}
		return feedbackEvent{}, fmt.Errorf("sent feedback not found")
	})
}

// ReviewerReply marks a human answer in a sent thread. Agents read it through
// get_review_feedback; it is sent as soon as the reviewer submits it.
const ReviewerReply = "reviewer"

func ReplyAsReviewer(dir, id, body string) error {
	if strings.TrimSpace(body) == "" || len(body) > 16000 {
		return fmt.Errorf("reply requires a bounded body")
	}
	return appendEvent(dir, func(state FeedbackState) (feedbackEvent, error) {
		for _, item := range state.Items {
			if item.ID == id && item.Status != "draft" {
				return feedbackEvent{Type: "reply", IDs: []string{id}, Reply: &Reply{Kind: ReviewerReply, Body: strings.TrimSpace(body), Agent: "Reviewer"}}, nil
			}
		}
		return feedbackEvent{}, fmt.Errorf("sent feedback not found")
	})
}
func ResolveFeedback(dir, id string, resolved bool) error {
	return appendEvent(dir, func(state FeedbackState) (feedbackEvent, error) {
		for _, item := range state.Items {
			if item.ID == id && item.Status != "draft" {
				return feedbackEvent{Type: "resolve", IDs: []string{id}, Resolved: resolved}, nil
			}
		}
		return feedbackEvent{}, fmt.Errorf("sent feedback not found")
	})
}

// AgentFeedback never exposes unsent drafts, even when a cursor predates them.
func AgentFeedback(dir string, since int) (FeedbackState, error) {
	state, err := LoadFeedback(dir)
	if err != nil {
		return state, err
	}
	items := []Feedback{}
	for _, item := range state.Items {
		if item.Status != "draft" && item.Updated > since {
			items = append(items, item)
		}
	}
	state.Items = items
	return state, nil
}
