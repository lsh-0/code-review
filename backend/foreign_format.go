package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// the external review-feedback document format, mirroring the JSON encoding
// produced by the upstream `bb-feedback` program (its `Code_feedback` types).
//
// The format is provider-independent by upstream's own design: nothing here is
// Bitbucket-specific, and a second source normalised onto the same encoding
// imports through this decoder unchanged. Provider-specific work stays upstream.
//
// This is a boundary format, deliberately kept out of `model`: these types decode
// what arrives and are then mapped onto the domain, so the domain never carries a
// wire concern.
//
// Every tagged union decodes by its tag name and rejects an unrecognised tag
// rather than defaulting to a known case. Upstream reasons the same way about its
// own author kinds: defaulting an unrecognised author to a person would misreport
// exactly the distinction the type exists to draw. The same applies to a position
// kind, where a wrong default would place a comment on the wrong side of the diff.

// the tag names of each union in the format. Named rather than inlined so a
// decoder test can pin them: a change to the upstream encoder then fails a test
// here rather than at runtime.
const (
	locationPullRequest  = "pull_request"
	locationFileRevision = "file_revision"

	positionWholeFile = "whole_file"
	positionOriginal  = "original"
	positionUpdated   = "updated"
	positionUnchanged = "unchanged"

	bodyDeleted  = "deleted"
	bodyMarkdown = "markdown"

	authorHuman       = "human"
	authorApplication = "application"
	authorUnknown     = "unknown"

	resolutionUnresolved          = "unresolved"
	resolutionResolved            = "resolved"
	resolutionResolvedWithoutInfo = "resolved_without_detail"
)

// a closed range of line numbers, `First` through `Last` inclusive. Upstream
// enforces `First <= Last` at construction, so a document violating it is a
// payload to reject rather than to propagate.
type foreignSpan struct {
	First int `json:"first"`
	Last  int `json:"last"`
}

// report whether the span is well formed. A document whose range starts beyond
// its end is rejected: upstream can emit one only from a malformed payload.
func (s foreignSpan) valid() bool {
	return s.First >= 1 && s.First <= s.Last
}

// which lines a comment addresses, as the four cases the source's nullable
// pre-image and post-image line fields can express. `Kind` selects which span
// fields are meaningful:
//   - `whole_file`: no span; the comment addresses the file, not a line
//   - `original`: `Lines` holds pre-image (old-side) lines this change removed
//   - `updated`: `Lines` holds post-image (new-side) lines this change added
//   - `unchanged`: both `OriginalLines` and `UpdatedLines`; a context line
type foreignPosition struct {
	Kind          string      `json:"kind"`
	Lines         foreignSpan `json:"lines"`
	OriginalLines foreignSpan `json:"original_lines"`
	UpdatedLines  foreignSpan `json:"updated_lines"`
}

// what a thread is attached to. `Kind` is `pull_request` for review-level
// discussion, in which case `Path` and `Position` are absent; `file_revision`
// carries both.
type foreignLocation struct {
	Kind     string          `json:"kind"`
	Path     string          `json:"path"`
	Position foreignPosition `json:"position"`
}

// a comment's text, or the fact that there is none. Upstream treats deletion and
// emptiness as one state, so `deleted` carries no text and the question "empty
// string or deleted comment" cannot be asked.
type foreignBody struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

// who wrote a comment. `Kind` distinguishes a person from a machine, taken from
// the source's own discriminator rather than by matching names against a list of
// known bots.
type foreignAuthor struct {
	DisplayName     string `json:"display_name"`
	AccountID       string `json:"account_id"`
	UUID            string `json:"uuid"`
	Nickname        string `json:"nickname"`
	Kind            string `json:"kind"`
	ApplicationKind string `json:"application_kind"`
	ReportedType    string `json:"reported_type"`
}

// whether a thread has been dealt with at the source. `resolved_without_detail`
// is distinct from `resolved`: the source returns it when the caller omitted the
// field expansion, meaning resolved but withholding by whom and when.
type foreignResolution struct {
	State string        `json:"state"`
	By    foreignAuthor `json:"by"`
	At    string        `json:"at"`
}

// one comment in the source, with its replies nested to arbitrary depth.
type foreignComment struct {
	ID        int              `json:"id"`
	Author    foreignAuthor    `json:"author"`
	Body      foreignBody      `json:"body"`
	CreatedAt string           `json:"created_at"`
	UpdatedAt string           `json:"updated_at"`
	Pending   bool             `json:"pending"`
	Replies   []foreignComment `json:"replies"`
}

// a thread: where it sits, whether it is resolved at the source, and its root
// comment. A reply never resolves separately, so resolution is thread-level.
type foreignThread struct {
	Location   foreignLocation   `json:"location"`
	Resolution foreignResolution `json:"resolution"`
	Root       foreignComment    `json:"root"`
}

// a branch and the commit it pointed at.
type foreignRevision struct {
	Branch string `json:"branch"`
	Commit string `json:"commit"`
}

// the pull request the feedback was raised on. Only the fields the import needs
// for its source registry are used; the rest are decoded so a malformed document
// is caught here rather than silently ignored.
type foreignPullRequest struct {
	ID          int             `json:"id"`
	Title       string          `json:"title"`
	State       string          `json:"state"`
	Draft       bool            `json:"draft"`
	Author      foreignAuthor   `json:"author"`
	CreatedAt   string          `json:"created_at"`
	Source      foreignRevision `json:"source"`
	Destination foreignRevision `json:"destination"`
}

// the whole document. Upstream partitions its threads by location into these two
// collections, but the collections are not trusted: `threads` repartitions by each
// thread's own `location`, so a hand-edited document whose arrays contradict their
// locations is corrected rather than believed.
type foreignDocument struct {
	PullRequest  foreignPullRequest `json:"pull_request"`
	Discussion   []foreignThread    `json:"discussion"`
	FileFeedback []foreignThread    `json:"file_feedback"`
}

// every thread in the document, discussion first, each classified by its own
// `location` rather than by the array it was found in.
func (d foreignDocument) threads() []foreignThread {
	all := make([]foreignThread, 0, len(d.Discussion)+len(d.FileFeedback))
	all = append(all, d.Discussion...)
	all = append(all, d.FileFeedback...)
	return all
}

// decode an external review-feedback document from `data`.
//
// Returns an error naming the offending tag and its field for an unrecognised
// union tag, for a malformed line span, and for a thread missing the path its
// location requires. `json.Unmarshal` rejects invalid JSON and type mismatches
// before any of those checks run.
func decodeForeignDocument(data []byte) (*foreignDocument, error) {
	var doc foreignDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("failed to decode review document: %w", err)
	}

	if doc.PullRequest.ID == 0 {
		return nil, fmt.Errorf("review document has no pull request id")
	}

	for _, thread := range doc.threads() {
		if err := validateForeignThread(thread); err != nil {
			return nil, err
		}
	}

	return &doc, nil
}

// check one thread's tags and required fields, recursing into its replies. Every
// error names the value it rejected so a format change upstream is diagnosable
// from the message alone.
func validateForeignThread(thread foreignThread) error {
	switch thread.Location.Kind {
	case locationPullRequest:
	case locationFileRevision:
		if thread.Location.Path == "" {
			return fmt.Errorf("a %s thread has no path", locationFileRevision)
		}
		if err := validateForeignPosition(thread.Location.Position, thread.Location.Path); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unrecognised location.kind %q", thread.Location.Kind)
	}

	switch thread.Resolution.State {
	case resolutionUnresolved, resolutionResolved, resolutionResolvedWithoutInfo:
	default:
		return fmt.Errorf("unrecognised resolution.state %q", thread.Resolution.State)
	}

	return validateForeignComment(thread.Root)
}

// check a position's tag and the spans that tag makes meaningful.
func validateForeignPosition(position foreignPosition, path string) error {
	switch position.Kind {
	case positionWholeFile:
	case positionOriginal, positionUpdated:
		if !position.Lines.valid() {
			return fmt.Errorf("%s: malformed %s line span %d-%d", path, position.Kind, position.Lines.First, position.Lines.Last)
		}
	case positionUnchanged:
		if !position.OriginalLines.valid() {
			return fmt.Errorf("%s: malformed original line span %d-%d", path, position.OriginalLines.First, position.OriginalLines.Last)
		}
		if !position.UpdatedLines.valid() {
			return fmt.Errorf("%s: malformed updated line span %d-%d", path, position.UpdatedLines.First, position.UpdatedLines.Last)
		}
	default:
		return fmt.Errorf("%s: unrecognised position.kind %q", path, position.Kind)
	}
	return nil
}

// check a comment's body and author tags, then its replies to any depth.
//
// Upstream declares a markdown body's text and an author's display name never empty,
// so a document carrying either empty is malformed rather than merely sparse. Both are
// rejected here: an empty body would otherwise map to a comment with no content, which
// renders as a blank thread the reviewer cannot act on or account for.
func validateForeignComment(comment foreignComment) error {
	switch comment.Body.Kind {
	case bodyDeleted:
	case bodyMarkdown:
		if strings.TrimSpace(comment.Body.Text) == "" {
			return fmt.Errorf("comment %d: a %s body carries no text", comment.ID, bodyMarkdown)
		}
	default:
		return fmt.Errorf("comment %d: unrecognised body.kind %q", comment.ID, comment.Body.Kind)
	}

	switch comment.Author.Kind {
	case authorHuman, authorApplication, authorUnknown:
	default:
		return fmt.Errorf("comment %d: unrecognised author.kind %q", comment.ID, comment.Author.Kind)
	}

	if strings.TrimSpace(comment.Author.DisplayName) == "" {
		return fmt.Errorf("comment %d: author carries no display name", comment.ID)
	}

	for _, reply := range comment.Replies {
		if err := validateForeignComment(reply); err != nil {
			return err
		}
	}
	return nil
}
