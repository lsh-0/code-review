package model

import (
	"crypto/rand"
	"encoding/hex"
)

type CommentStatus string

const (
	CommentStatusActive   CommentStatus = "active"
	CommentStatusResolved CommentStatus = "resolved"
	CommentStatusIgnored  CommentStatus = "ignored"
)

// Reply is a child note under a root Comment. Replies form a flat,
// sequential thread; they carry no status of their own and cannot nest
// further. Only the root comment is resolvable.
type Reply struct {
	ID      string `json:"id"`
	Author  string `json:"author"`
	Content string `json:"content"`
}

type Comment struct {
	ID            string        `json:"id"`
	Author        string        `json:"author"`
	Content       string        `json:"content"`
	LineNumber    int           `json:"line_number"`
	Status        CommentStatus `json:"status"`
	ContextBefore string        `json:"context_before"`
	ContextLine   string        `json:"context_line"`
	ContextAfter  string        `json:"context_after"`
	Replies       []*Reply      `json:"replies,omitempty"`
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
	MarkedFiles  []string    `json:"marked_files"`
}

// ReadmeText describes the state file for a tool (typically an AI) reading it
// directly: what the file is, the schema, and how to act on comments. It is
// stored in the Review's `_readme` field and refreshed on every save.
const ReadmeText = "This is a code-review state file for the 'code-review' tool. " +
	"It records review comments against a git diff (source_branch compared to target_branch in repo_path). " +
	"Schema: `files` is an array of { file_path, comments[] }. Each comment has: " +
	"`id` (stable identifier), `author`, `content` (the review note, in markdown), " +
	"`line_number` (1-based line in the new version of the file), " +
	"`status` (one of 'active', 'resolved', 'ignored'), and " +
	"`context_before`/`context_line`/`context_after` (the surrounding source lines captured when the comment was made, " +
	"used to relocate the comment if line numbers shift), and " +
	"`replies` (an optional flat array of { id, author, content } child notes forming a thread under the comment; " +
	"replies have no status of their own and only the root comment is resolvable). " +
	"To act on a review: address every comment with status 'active'. Do the smaller, mechanical changes first, then the " +
	"larger ones. A comment must be addressed unless it is genuinely impossible; if a comment seems impossible, you have " +
	"probably misunderstood it, so re-read it rather than skip it. Make the requested change in the actual source file at " +
	"file_path within repo_path, then set that comment's `status` to 'resolved'. " +
	"Do not set `status` to 'ignored' on your own; leave such a comment 'active', add a reply explaining what blocked you, " +
	"and let the reviewer decide. " +
	"You may append entries to a comment's `replies` array (each a { id, author, content } object with a new unique id); " +
	"use replies to record a blocker, a question, or a note for the reviewer. " +
	"Do not change `id`, `line_number`, or the context fields, do not edit or delete existing replies, and do not add or " +
	"remove root comments. " +
	"`marked_files` is an array of file_path strings the reviewer has marked as reviewed. Whenever you modify a source file " +
	"while acting on this review, remove that file's path from `marked_files` if present, so the reviewer can see at a glance " +
	"which reviewed files have changed and need revisiting. This applies to every file you edit, including files changed " +
	"because feedback in one file applies to others that have no comments of their own. Do not add paths to `marked_files`. " +
	"Apart from comment `status` values, appended replies, and removing entries from `marked_files`, preserve this `_readme` " +
	"field and all other fields as-is. " +
	"The file is JSON; write it back with the same structure."

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

func NewReply(content string, author string) *Reply {
	return &Reply{
		ID:      GenerateID(),
		Author:  author,
		Content: content,
	}
}

// append a child reply to the comment's flat reply thread.
func (c *Comment) AddReply(content string, author string) *Reply {
	reply := NewReply(content, author)
	c.Replies = append(c.Replies, reply)
	return reply
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

func (f *FileDiff) GetComment(commentID string) *Comment {
	for _, comment := range f.Comments {
		if comment.ID == commentID {
			return comment
		}
	}
	return nil
}

func (f *FileDiff) DeleteComment(commentID string) {
	for i, comment := range f.Comments {
		if comment.ID == commentID {
			f.Comments = append(f.Comments[:i], f.Comments[i+1:]...)
			return
		}
	}
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
	return &Review{
		Readme:       ReadmeText,
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

// report whether `filePath` is in the marked-files set.
func (r *Review) IsFileMarked(filePath string) bool {
	for _, marked := range r.MarkedFiles {
		if marked == filePath {
			return true
		}
	}
	return false
}

// add `filePath` to the marked-files set, with no effect if already present.
func (r *Review) MarkFile(filePath string) {
	if !r.IsFileMarked(filePath) {
		r.MarkedFiles = append(r.MarkedFiles, filePath)
	}
}

// remove `filePath` from the marked-files set, with no effect if absent.
func (r *Review) UnmarkFile(filePath string) {
	for i, marked := range r.MarkedFiles {
		if marked == filePath {
			r.MarkedFiles = append(r.MarkedFiles[:i], r.MarkedFiles[i+1:]...)
			return
		}
	}
}

func (r *Review) GetAllComments() []*Comment {
	allComments := make([]*Comment, 0)
	for _, file := range r.Files {
		allComments = append(allComments, file.Comments...)
	}
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
