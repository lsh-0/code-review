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

// A Comment is a review note against a line of the diff. A reply is just a
// Comment whose ParentID is the id of the comment it answers; a root comment
// has an empty ParentID. Replies form a flat thread under their root and are
// not themselves resolvable: only root comments carry a meaningful status.
type Comment struct {
	ID            string        `json:"id"`
	ParentID      string        `json:"parent_id,omitempty"`
	Author        string        `json:"author"`
	Content       string        `json:"content"`
	LineNumber    int           `json:"line_number"`
	Status        CommentStatus `json:"status"`
	ContextBefore string        `json:"context_before"`
	ContextLine   string        `json:"context_line"`
	ContextAfter  string        `json:"context_after"`
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

func NewComment(content string, lineNumber int, author string) *Comment {
	return &Comment{
		ID:         GenerateID(),
		Author:     author,
		Content:    content,
		LineNumber: lineNumber,
		Status:     CommentStatusActive,
	}
}

func NewCommentWithContext(content string, lineNumber int, author string, contextBefore string, contextLine string, contextAfter string) *Comment {
	return &Comment{
		ID:            GenerateID(),
		Author:        author,
		Content:       content,
		LineNumber:    lineNumber,
		Status:        CommentStatusActive,
		ContextBefore: contextBefore,
		ContextLine:   contextLine,
		ContextAfter:  contextAfter,
	}
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
		return comment.LineNumber
	}
	if root := findComment(comments, comment.ParentID); root != nil {
		return root.LineNumber
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

func (f *FileDiff) AddCommentWithContext(content string, lineNumber int, author string, contextBefore string, contextLine string, contextAfter string) *Comment {
	comment := NewCommentWithContext(content, lineNumber, author, contextBefore, contextLine, contextAfter)
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
		if comment.LineNumber == lineNumber {
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
