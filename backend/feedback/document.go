// Package feedback defines the import document: review feedback from a hosted pull
// request, or any source normalised onto the same encoding, in a form that does not
// depend on its provider.
//
// The encoding mirrors the JSON written by the upstream `bb-feedback` program (its
// `Code_feedback` types). The normative definition is the JSON Schema embedded in
// this package and printed by `code-review schema`. Producers code against that
// schema. The Go types here only decode and encode what it accepts.
//
// Nothing in this package depends on a provider. A provider package fetches its own
// wire format and normalises onto `Document`.
package feedback

import "encoding/json"

// the tag names of each union in the format. Named rather than inlined so a test can
// pin them: a change to the encoding then fails a test rather than an import.
const (
	LocationPullRequest  = "pull_request"
	LocationFileRevision = "file_revision"

	PositionWholeFile = "whole_file"
	PositionOriginal  = "original"
	PositionUpdated   = "updated"
	PositionUnchanged = "unchanged"

	BodyDeleted  = "deleted"
	BodyMarkdown = "markdown"

	AuthorHuman       = "human"
	AuthorApplication = "application"
	AuthorUnknown     = "unknown"

	ResolutionUnresolved          = "unresolved"
	ResolutionResolved            = "resolved"
	ResolutionResolvedWithoutInfo = "resolved_without_detail"
)

// the only version of the encoding. A document without `schema_version` is read as
// this version.
const SchemaVersion = 1

// the layout every timestamp is written in: UTC, six fractional digits, a `Z`
// suffix. This is the layout upstream renders, so an encoded document compares equal
// to upstream output.
const TimestampLayout = "2006-01-02T15:04:05.000000Z"

// a closed range of line numbers, `First` through `Last` inclusive, with `First` no
// greater than `Last`. The schema cannot compare two properties, so `Decode`
// enforces the ordering.
type Span struct {
	First int `json:"first"`
	Last  int `json:"last"`
}

// reports whether the span starts at line 1 or later and does not end before it
// starts.
func (s Span) Valid() bool {
	return s.First >= 1 && s.First <= s.Last
}

// which lines a comment addresses. `Kind` selects which spans are meaningful:
//   - `whole_file`: no span. The comment addresses the file, not a line.
//   - `original`: `Lines` holds pre-image (old-side) lines the change removed.
//   - `updated`: `Lines` holds post-image (new-side) lines the change added.
//   - `unchanged`: `OriginalLines` and `UpdatedLines`. A context line.
type Position struct {
	Kind          string `json:"kind"`
	Lines         Span   `json:"lines"`
	OriginalLines Span   `json:"original_lines"`
	UpdatedLines  Span   `json:"updated_lines"`
}

// writes only the spans `Kind` makes meaningful.
func (p Position) MarshalJSON() ([]byte, error) {
	switch p.Kind {
	case PositionOriginal, PositionUpdated:
		return json.Marshal(struct {
			Kind  string `json:"kind"`
			Lines Span   `json:"lines"`
		}{p.Kind, p.Lines})
	case PositionUnchanged:
		return json.Marshal(struct {
			Kind          string `json:"kind"`
			OriginalLines Span   `json:"original_lines"`
			UpdatedLines  Span   `json:"updated_lines"`
		}{p.Kind, p.OriginalLines, p.UpdatedLines})
	default:
		return json.Marshal(struct {
			Kind string `json:"kind"`
		}{p.Kind})
	}
}

// what a thread is attached to. `Kind` is `pull_request` for review-level
// discussion, which has no `Path` or `Position`. `file_revision` carries both.
type Location struct {
	Kind     string   `json:"kind"`
	Path     string   `json:"path"`
	Position Position `json:"position"`
}

// writes `Path` and `Position` only for a file-anchored location.
func (l Location) MarshalJSON() ([]byte, error) {
	if l.Kind != LocationFileRevision {
		return json.Marshal(struct {
			Kind string `json:"kind"`
		}{l.Kind})
	}
	return json.Marshal(struct {
		Kind     string   `json:"kind"`
		Path     string   `json:"path"`
		Position Position `json:"position"`
	}{l.Kind, l.Path, l.Position})
}

// a comment's text, or the fact that there is none. Deletion and emptiness are one
// state, so a `deleted` body carries no text.
type Body struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

// writes `Text` only for a markdown body.
func (b Body) MarshalJSON() ([]byte, error) {
	if b.Kind != BodyMarkdown {
		return json.Marshal(struct {
			Kind string `json:"kind"`
		}{b.Kind})
	}
	return json.Marshal(struct {
		Kind string `json:"kind"`
		Text string `json:"text"`
	}{b.Kind, b.Text})
}

// who wrote a comment. `Kind` tells a person from a machine, taken from the source's
// own discriminator and never from matching names against a list of known bots.
// `ApplicationKind` accompanies `application`. `ReportedType` accompanies `unknown`
// and holds the discriminator the source reported.
type Author struct {
	DisplayName     string  `json:"display_name"`
	AccountID       *string `json:"account_id"`
	UUID            *string `json:"uuid"`
	Nickname        *string `json:"nickname"`
	Kind            string  `json:"kind"`
	ApplicationKind string  `json:"application_kind"`
	ReportedType    string  `json:"reported_type"`
}

// writes the optional identifiers as `null` when absent, and only the kind detail
// that `Kind` makes meaningful.
func (a Author) MarshalJSON() ([]byte, error) {
	type common struct {
		DisplayName string  `json:"display_name"`
		AccountID   *string `json:"account_id"`
		UUID        *string `json:"uuid"`
		Nickname    *string `json:"nickname"`
		Kind        string  `json:"kind"`
	}
	base := common{a.DisplayName, a.AccountID, a.UUID, a.Nickname, a.Kind}
	switch a.Kind {
	case AuthorApplication:
		return json.Marshal(struct {
			common
			ApplicationKind string `json:"application_kind"`
		}{base, a.ApplicationKind})
	case AuthorUnknown:
		return json.Marshal(struct {
			common
			ReportedType string `json:"reported_type"`
		}{base, a.ReportedType})
	default:
		return json.Marshal(base)
	}
}

// whether a thread has been dealt with at the source. `resolved_without_detail` is
// resolved, with by whom and when withheld by the source. `By` and `At` are present
// only for `resolved`.
type Resolution struct {
	State string  `json:"state"`
	By    *Author `json:"by"`
	At    string  `json:"at"`
}

// writes `By` and `At` only for a resolution that carries them.
func (r Resolution) MarshalJSON() ([]byte, error) {
	if r.State != ResolutionResolved {
		return json.Marshal(struct {
			State string `json:"state"`
		}{r.State})
	}
	return json.Marshal(struct {
		State string  `json:"state"`
		By    *Author `json:"by"`
		At    string  `json:"at"`
	}{r.State, r.By, r.At})
}

// one comment, with its replies nested to any depth, ordered by creation time and
// then by identifier.
type Comment struct {
	ID        int       `json:"id"`
	Author    Author    `json:"author"`
	Body      Body      `json:"body"`
	CreatedAt string    `json:"created_at"`
	UpdatedAt *string   `json:"updated_at"`
	Pending   bool      `json:"pending"`
	Replies   []Comment `json:"replies"`
}

// writes an absent reply list as `[]`, as upstream does.
func (c Comment) MarshalJSON() ([]byte, error) {
	type plain Comment
	value := plain(c)
	if value.Replies == nil {
		value.Replies = []Comment{}
	}
	return json.Marshal(value)
}

// a thread: where it sits, whether it is resolved at the source, and its root
// comment. A reply never resolves separately, so resolution is thread-level.
type Thread struct {
	Location   Location   `json:"location"`
	Resolution Resolution `json:"resolution"`
	Root       Comment    `json:"root"`
}

// a branch and the commit it pointed at, when known.
type Revision struct {
	Branch string  `json:"branch"`
	Commit *string `json:"commit"`
}

// a person taking part in the review. `Role` is retained verbatim, so a role outside
// the known set survives.
type Participant struct {
	User     Author `json:"user"`
	Role     string `json:"role"`
	Approved bool   `json:"approved"`
}

// the pull request the feedback was raised on. `State` is retained verbatim, so a
// state outside the known set survives. `ReportedCommentCount` is the count the
// source reported, used to reconcile against the comments present.
type PullRequest struct {
	ID                   int           `json:"id"`
	Title                string        `json:"title"`
	Description          *string       `json:"description"`
	State                string        `json:"state"`
	Draft                bool          `json:"draft"`
	Author               Author        `json:"author"`
	CreatedAt            string        `json:"created_at"`
	UpdatedAt            *string       `json:"updated_at"`
	Source               Revision      `json:"source"`
	Destination          Revision      `json:"destination"`
	Participants         []Participant `json:"participants"`
	ReportedCommentCount int           `json:"reported_comment_count"`
	ReportedTaskCount    int           `json:"reported_task_count"`
}

// writes an absent participant list as `[]`, as upstream does.
func (p PullRequest) MarshalJSON() ([]byte, error) {
	type plain PullRequest
	value := plain(p)
	if value.Participants == nil {
		value.Participants = []Participant{}
	}
	return json.Marshal(value)
}

// the whole document. A producer partitions threads by location into `Discussion`
// and `FileFeedback`, but the partition is not trusted: `Threads` returns both, and
// each thread is classified by its own `Location`.
//
// `SchemaVersion` is zero when the document did not state one, which reads as
// version 1.
type Document struct {
	SchemaVersion int         `json:"schema_version,omitempty"`
	PullRequest   PullRequest `json:"pull_request"`
	Discussion    []Thread    `json:"discussion"`
	FileFeedback  []Thread    `json:"file_feedback"`
}

// writes absent thread lists as `[]`, as upstream does.
func (d Document) MarshalJSON() ([]byte, error) {
	type plain Document
	value := plain(d)
	if value.Discussion == nil {
		value.Discussion = []Thread{}
	}
	if value.FileFeedback == nil {
		value.FileFeedback = []Thread{}
	}
	return json.Marshal(value)
}

// every thread in the document, discussion first, in document order.
func (d Document) Threads() []Thread {
	all := make([]Thread, 0, len(d.Discussion)+len(d.FileFeedback))
	all = append(all, d.Discussion...)
	all = append(all, d.FileFeedback...)
	return all
}
