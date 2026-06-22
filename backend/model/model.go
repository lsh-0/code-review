package model

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
)

type CommentStatus string

const (
	CommentStatusActive   CommentStatus = "active"
	CommentStatusResolved CommentStatus = "resolved"
	CommentStatusIgnored  CommentStatus = "ignored"
)

// An Anchor records where a comment's code lived against one version of its
// file, keyed by that file's git blob SHA. `Context` is the captured window of
// raw line contents around the anchored line; `Offset` is the index of the
// anchored line within `Context` (0-based), which need not be the centre when
// the window is clipped by a hunk edge; `LineNumber` is the new-side line it was
// placed at. An anchor with an empty `Context` is adrift: the content could not
// be located against that blob, so it carries no meaningful line or offset. A
// comment owns an ordered history of these, one per distinct blob it has been
// reconciled against.
type Anchor struct {
	Blob       string   `json:"blob"`
	LineNumber int      `json:"line_number"`
	Offset     int      `json:"offset,omitempty"`
	Context    []string `json:"context,omitempty"`
}

// report whether the anchor failed to locate its content (no captured context).
func (a Anchor) IsAdrift() bool {
	return len(a.Context) == 0
}

// A Comment is a review note against a line of the diff. A reply is just a
// Comment whose ParentID is the id of the comment it answers; a root comment
// has an empty ParentID. Replies form a flat thread under their root and are
// not themselves resolvable: only root comments carry a meaningful status.
//
// A comment is anchored through `Anchors`, an ordered history of `Anchor`
// records (one per distinct blob it has been reconciled against). `Anchors[0]`
// is the creation anchor and is always good. The comment's current placement
// and whether it is outdated are derived from the most-recent anchor, never
// stored.
type Comment struct {
	ID       string        `json:"id"`
	ParentID string        `json:"parent_id,omitempty"`
	Author   string        `json:"author"`
	Content  string        `json:"content"`
	Status   CommentStatus `json:"status"`
	Anchors  []Anchor      `json:"anchors,omitempty"`
}

type FileDiff struct {
	FilePath string     `json:"file_path"`
	Comments []*Comment `json:"comments"`
}

type Review struct {
	Readme       string      `json:"_readme"`
	ID           string      `json:"id"`
	RepoPath     string      `json:"repo_path"`
	SourceBranch string      `json:"source_branch"`
	TargetBranch string      `json:"target_branch"`
	Files        []*FileDiff `json:"files"`
	// Comments are review-level notes not anchored to any file or line — overall
	// feedback. They behave exactly like file comments (status, replies) but
	// carry no file path, line number, or context.
	Comments    []*Comment  `json:"comments,omitempty"`
	MarkedFiles MarkedFiles `json:"marked_files"`
}

// a file the reviewer has marked as done, recording the git blob SHA of the
// file's content at the review's source-branch HEAD when it was marked. The SHA
// lets a later open detect that the file changed (different SHA) or was deleted
// (no current SHA) and drop the mark. An empty `Blob` is a legacy mark awaiting
// backfill.
type FileMark struct {
	Path string `json:"path"`
	Blob string `json:"blob,omitempty"`
}

// the marked-files set. It serialises as a list of `FileMark` records, but
// unmarshals from either the new record list or the legacy bare-path list, so
// old state files load without a migration step.
type MarkedFiles []FileMark

// accept both the new `[{"path":…,"blob":…}]` form and the legacy `["a.go"]`
// form. A legacy path becomes a record with an empty blob, flagged for backfill
// on first open.
func (m *MarkedFiles) UnmarshalJSON(data []byte) error {
	var records []FileMark
	if err := json.Unmarshal(data, &records); err == nil {
		*m = records
		return nil
	}

	var paths []string
	if err := json.Unmarshal(data, &paths); err != nil {
		return err
	}
	legacy := make([]FileMark, len(paths))
	for i, path := range paths {
		legacy[i] = FileMark{Path: path}
	}
	*m = legacy
	return nil
}

func GenerateID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// build a comment anchored at `lineNumber` with no captured context. Used for
// replies and review-level comments, which carry no anchor of their own (line
// 0, empty history). A line-anchored comment with real context is made with
// `NewCommentWithContext`.
func NewComment(content string, lineNumber int, author string) *Comment {
	comment := &Comment{
		ID:      GenerateID(),
		Author:  author,
		Content: content,
		Status:  CommentStatusActive,
	}
	if lineNumber != 0 {
		comment.Anchors = []Anchor{{LineNumber: lineNumber}}
	}
	return comment
}

// build a line-anchored comment whose first anchor records `blob`, `lineNumber`,
// the captured `context` window, and `offset` (the anchored line's index within
// `context`). The first anchor is good (non-empty context) and becomes the
// baseline the reconciler matches against as the file changes.
func NewCommentWithContext(content string, lineNumber int, author string, blob string, context []string, offset int) *Comment {
	return &Comment{
		ID:      GenerateID(),
		Author:  author,
		Content: content,
		Status:  CommentStatusActive,
		Anchors: []Anchor{{Blob: blob, LineNumber: lineNumber, Offset: offset, Context: context}},
	}
}

// the comment's most-recent anchor, or nil if it has none (replies and
// review-level comments).
func (c *Comment) currentAnchor() *Anchor {
	if len(c.Anchors) == 0 {
		return nil
	}
	return &c.Anchors[len(c.Anchors)-1]
}

// the comment's current new-side line number, taken from its most-recent
// anchor. Returns 0 when the comment has no anchor or its current anchor is
// adrift.
func (c *Comment) CurrentLineNumber() int {
	anchor := c.currentAnchor()
	if anchor == nil {
		return 0
	}
	return anchor.LineNumber
}

// report whether the comment can no longer be placed against the current diff:
// it has an anchor history whose most-recent anchor is adrift. A comment with
// no anchors (reply, review-level) is never outdated.
func (c *Comment) IsOutdated() bool {
	anchor := c.currentAnchor()
	return anchor != nil && anchor.IsAdrift()
}

// the most recent anchor that is not adrift, or nil if the comment has none.
func (c *Comment) lastGoodAnchor() *Anchor {
	for i := len(c.Anchors) - 1; i >= 0; i-- {
		if !c.Anchors[i].IsAdrift() {
			return &c.Anchors[i]
		}
	}
	return nil
}

// the captured context of the most recent anchor that has one — the target the
// reconciler matches against, and the content rendered for an outdated comment.
// Returns nil if the comment has no good anchor.
func (c *Comment) LastGoodContext() []string {
	if anchor := c.lastGoodAnchor(); anchor != nil {
		return anchor.Context
	}
	return nil
}

// accept both the current anchor-based form and the legacy form, which carried
// a bare `line_number` plus `context_before`/`context_line`/`context_after` and
// no anchor history. A legacy line-anchored comment is upgraded in place to a
// single good first anchor with an empty blob, flagging it for baseline
// backfill on the first reconciliation — mirroring how a legacy bare-path mark
// carries an empty blob. So old state files load without a migration step.
func (c *Comment) UnmarshalJSON(data []byte) error {
	type wire struct {
		ID            string        `json:"id"`
		ParentID      string        `json:"parent_id,omitempty"`
		Author        string        `json:"author"`
		Content       string        `json:"content"`
		Status        CommentStatus `json:"status"`
		Anchors       []Anchor      `json:"anchors,omitempty"`
		LineNumber    int           `json:"line_number,omitempty"`
		ContextBefore string        `json:"context_before,omitempty"`
		ContextLine   string        `json:"context_line,omitempty"`
		ContextAfter  string        `json:"context_after,omitempty"`
	}
	var w wire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	c.ID = w.ID
	c.ParentID = w.ParentID
	c.Author = w.Author
	c.Content = w.Content
	c.Status = w.Status
	c.Anchors = w.Anchors

	// already in anchor form, or an unanchored comment (reply, review-level):
	// nothing to upgrade.
	if len(c.Anchors) > 0 || w.LineNumber == 0 {
		return nil
	}

	// legacy context was exactly [before, line, after]: the anchored line sits
	// at index 1.
	c.Anchors = []Anchor{{
		Blob:       "",
		LineNumber: w.LineNumber,
		Offset:     1,
		Context:    []string{w.ContextBefore, w.ContextLine, w.ContextAfter},
	}}
	return nil
}

func (c *Comment) Resolve() {
	c.Status = CommentStatusResolved
}

func (c *Comment) Ignore() {
	c.Status = CommentStatusIgnored
}

func (c *Comment) Reactivate() {
	c.Status = CommentStatusActive
}

func (c *Comment) UpdateContent(content string) {
	c.Content = content
}

// find a comment by id within a flat comment list, or nil if absent.
func findComment(comments []*Comment, commentID string) *Comment {
	for _, comment := range comments {
		if comment.ID == commentID {
			return comment
		}
	}
	return nil
}

// append a reply to `parentID` within a flat comment list and return the new
// list and the reply. The thread stays flat: replying to a reply re-roots to
// the same root, so a reply never becomes a grandchild. A reply carries no line
// number or context.
func appendReply(comments []*Comment, parentID string, content string, author string) ([]*Comment, *Comment) {
	root := parentID
	if parent := findComment(comments, parentID); parent != nil && parent.ParentID != "" {
		root = parent.ParentID
	}

	reply := NewComment(content, 0, author)
	reply.ParentID = root
	return append(comments, reply), reply
}

// the aggregate review status of a file from its flat comment list, matching
// the file-list indicator: `active` if any root is active, else `resolved` if
// every root is resolved, else `ignored` if any root is ignored, else `none`.
// Replies carry no status and are ignored. Pure, so both the backend (building
// a mutation result) and the frontend (rendering the file pill) share one
// definition rather than duplicating the rules.
func FileCommentStatus(comments []*Comment) string {
	hasActive := false
	hasIgnored := false
	allResolved := true
	rootCount := 0

	for _, comment := range comments {
		if comment.ParentID != "" {
			continue
		}
		rootCount++
		switch comment.Status {
		case CommentStatusActive:
			hasActive = true
			allResolved = false
		case CommentStatusIgnored:
			hasIgnored = true
			allResolved = false
		}
	}

	if hasActive {
		return "active"
	}
	if allResolved && rootCount > 0 {
		return "resolved"
	}
	if hasIgnored {
		return "ignored"
	}
	return "none"
}

// the line number of a comment's root within a flat comment list. For a root
// comment that is its own line; for a reply it is the root's line. Returns 0 if
// the comment is absent or its root is missing (review-level comments carry no
// line and report 0).
func CommentRootLine(comments []*Comment, commentID string) int {
	comment := findComment(comments, commentID)
	if comment == nil {
		return 0
	}
	if comment.ParentID == "" {
		return comment.CurrentLineNumber()
	}
	if root := findComment(comments, comment.ParentID); root != nil {
		return root.CurrentLineNumber()
	}
	return 0
}

// remove a comment by id from a flat comment list, returning the new list.
// Deleting a root comment also removes every reply whose `ParentID` points at
// it, so a removed thread leaves no orphans.
func removeComment(comments []*Comment, commentID string) []*Comment {
	kept := comments[:0]
	for _, comment := range comments {
		if comment.ID == commentID || comment.ParentID == commentID {
			continue
		}
		kept = append(kept, comment)
	}
	return kept
}

func NewFileDiff(filePath string) *FileDiff {
	return &FileDiff{
		FilePath: filePath,
		Comments: make([]*Comment, 0),
	}
}

func (f *FileDiff) AddComment(content string, lineNumber int, author string) *Comment {
	comment := NewComment(content, lineNumber, author)
	f.Comments = append(f.Comments, comment)
	return comment
}

func (f *FileDiff) AddCommentWithContext(content string, lineNumber int, author string, blob string, context []string, offset int) *Comment {
	comment := NewCommentWithContext(content, lineNumber, author, blob, context, offset)
	f.Comments = append(f.Comments, comment)
	return comment
}

func (f *FileDiff) AddReply(parentID string, content string, author string) *Comment {
	comments, reply := appendReply(f.Comments, parentID, content, author)
	f.Comments = comments
	return reply
}

func (f *FileDiff) GetComment(commentID string) *Comment {
	return findComment(f.Comments, commentID)
}

func (f *FileDiff) DeleteComment(commentID string) {
	f.Comments = removeComment(f.Comments, commentID)
}

func (f *FileDiff) GetCommentsByLine(lineNumber int) []*Comment {
	result := make([]*Comment, 0)
	for _, comment := range f.Comments {
		if comment.CurrentLineNumber() == lineNumber {
			result = append(result, comment)
		}
	}
	return result
}

func NewReview(repoPath, sourceBranch, targetBranch string) *Review {
	// `Readme` is left empty here; `SaveReview` stamps the embedded readme on
	// every write, so it is populated by the time the state file is persisted.
	return &Review{
		ID:           GenerateID(),
		RepoPath:     repoPath,
		SourceBranch: sourceBranch,
		TargetBranch: targetBranch,
		Files:        make([]*FileDiff, 0),
	}
}

func (r *Review) AddFileDiff(filePath string) *FileDiff {
	diff := NewFileDiff(filePath)
	r.Files = append(r.Files, diff)
	return diff
}

func (r *Review) GetFileDiff(filePath string) *FileDiff {
	for _, file := range r.Files {
		if file.FilePath == filePath {
			return file
		}
	}
	return nil
}

// add a review-level comment: overall feedback not anchored to any file or
// line. It carries no line number or context.
func (r *Review) AddComment(content string, author string) *Comment {
	comment := NewComment(content, 0, author)
	r.Comments = append(r.Comments, comment)
	return comment
}

func (r *Review) AddReply(parentID string, content string, author string) *Comment {
	comments, reply := appendReply(r.Comments, parentID, content, author)
	r.Comments = comments
	return reply
}

func (r *Review) GetComment(commentID string) *Comment {
	return findComment(r.Comments, commentID)
}

func (r *Review) DeleteComment(commentID string) {
	r.Comments = removeComment(r.Comments, commentID)
}

// report whether `filePath` is in the marked-files set.
func (r *Review) IsFileMarked(filePath string) bool {
	for _, marked := range r.MarkedFiles {
		if marked.Path == filePath {
			return true
		}
	}
	return false
}

// add `filePath` to the marked-files set with the blob SHA of its content at
// mark-time, with no effect if already present. The SHA is later compared
// against the file's current content to detect a change.
func (r *Review) MarkFile(filePath string, blob string) {
	if !r.IsFileMarked(filePath) {
		r.MarkedFiles = append(r.MarkedFiles, FileMark{Path: filePath, Blob: blob})
	}
}

// remove `filePath` from the marked-files set, with no effect if absent.
func (r *Review) UnmarkFile(filePath string) {
	for i, marked := range r.MarkedFiles {
		if marked.Path == filePath {
			r.MarkedFiles = append(r.MarkedFiles[:i], r.MarkedFiles[i+1:]...)
			return
		}
	}
}

// reconcile the marked-files set against the files' current blob SHAs at the
// source-branch HEAD. `current` maps a path to its present SHA; a path absent
// from the map has been deleted at that revision. For each mark: a legacy mark
// with an empty stored blob is backfilled with the current SHA and kept (its
// baseline is established without evicting it); a mark whose file is deleted or
// whose SHA differs from the stored one is evicted; an unchanged mark is kept.
// Pure: it mutates only the marked-files slice from its inputs, so it is
// testable without git.
func (r *Review) EvictChangedMarks(current map[string]string) {
	if len(r.MarkedFiles) == 0 {
		return
	}

	kept := make([]FileMark, 0, len(r.MarkedFiles))
	for _, marked := range r.MarkedFiles {
		sha, present := current[marked.Path]
		if !present {
			// deleted at this revision: evict.
			continue
		}
		if marked.Blob == "" {
			// legacy mark: adopt the current SHA as the baseline and keep.
			marked.Blob = sha
			kept = append(kept, marked)
			continue
		}
		if marked.Blob == sha {
			kept = append(kept, marked)
		}
		// differing SHA: changed since marked, evict.
	}
	r.MarkedFiles = kept
}

func (r *Review) GetAllComments() []*Comment {
	allComments := make([]*Comment, 0)
	for _, file := range r.Files {
		allComments = append(allComments, file.Comments...)
	}
	allComments = append(allComments, r.Comments...)
	return allComments
}

func (r *Review) GetActiveCommentsCount() int {
	count := 0
	for _, comment := range r.GetAllComments() {
		if comment.Status == CommentStatusActive {
			count++
		}
	}
	return count
}
