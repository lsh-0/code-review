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

type Comment struct {
	ID            string        `json:"id"`
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
	ID           string      `json:"id"`
	RepoPath     string      `json:"repo_path"`
	SourceBranch string      `json:"source_branch"`
	TargetBranch string      `json:"target_branch"`
	Files        []*FileDiff `json:"files"`
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
