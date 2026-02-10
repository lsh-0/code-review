//go:build js
// +build js

package main

import (
	"code-review/model"
	"testing"
)

func TestGetFileCommentStatus(t *testing.T) {
	tests := []struct {
		name     string
		comments []*model.Comment
		expected string
	}{
		{
			name:     "no comments",
			comments: []*model.Comment{},
			expected: "none",
		},
		{
			name: "single active comment",
			comments: []*model.Comment{
				model.NewComment("test", 1),
			},
			expected: "active",
		},
		{
			name: "single resolved comment",
			comments: []*model.Comment{
				{ID: "1", Content: "test", LineNumber: 1, Status: model.CommentStatusResolved},
			},
			expected: "resolved",
		},
		{
			name: "single ignored comment",
			comments: []*model.Comment{
				{ID: "1", Content: "test", LineNumber: 1, Status: model.CommentStatusIgnored},
			},
			expected: "ignored",
		},
		{
			name: "active takes precedence over resolved",
			comments: []*model.Comment{
				model.NewComment("test1", 1),
				{ID: "2", Content: "test2", LineNumber: 2, Status: model.CommentStatusResolved},
			},
			expected: "active",
		},
		{
			name: "active takes precedence over ignored",
			comments: []*model.Comment{
				model.NewComment("test1", 1),
				{ID: "2", Content: "test2", LineNumber: 2, Status: model.CommentStatusIgnored},
			},
			expected: "active",
		},
		{
			name: "resolved takes precedence over ignored",
			comments: []*model.Comment{
				{ID: "1", Content: "test1", LineNumber: 1, Status: model.CommentStatusResolved},
				{ID: "2", Content: "test2", LineNumber: 2, Status: model.CommentStatusIgnored},
			},
			expected: "ignored",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commentsCache = make(map[string][]*model.Comment)
			commentsCache["test.go"] = tt.comments

			result := getFileCommentStatus("test.go")
			if result != tt.expected {
				t.Errorf("getFileCommentStatus() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetFileCommentStatusNotInCache(t *testing.T) {
	commentsCache = make(map[string][]*model.Comment)
	result := getFileCommentStatus("nonexistent.go")
	if result != "none" {
		t.Errorf("getFileCommentStatus() = %v, want none", result)
	}
}

func TestGetLineContext(t *testing.T) {
	diffFiles = []DiffFile{
		{
			Path: "test.go",
			Hunks: []DiffHunk{
				{
					Lines: []DiffLine{
						{Type: LineContext, Content: "line 1", NewLineNo: 1},
						{Type: LineAdded, Content: "line 2", NewLineNo: 2},
						{Type: LineContext, Content: "line 3", NewLineNo: 3},
					},
				},
			},
		},
	}

	before, line, after := getLineContext("test.go", 2)
	if before != "line 1" {
		t.Errorf("contextBefore = %v, want 'line 1'", before)
	}
	if line != "line 2" {
		t.Errorf("contextLine = %v, want 'line 2'", line)
	}
	if after != "line 3" {
		t.Errorf("contextAfter = %v, want 'line 3'", after)
	}
}

func TestGetLineContextFirstLine(t *testing.T) {
	diffFiles = []DiffFile{
		{
			Path: "test.go",
			Hunks: []DiffHunk{
				{
					Lines: []DiffLine{
						{Type: LineAdded, Content: "line 1", NewLineNo: 1},
						{Type: LineContext, Content: "line 2", NewLineNo: 2},
					},
				},
			},
		},
	}

	before, line, after := getLineContext("test.go", 1)
	if before != "" {
		t.Errorf("contextBefore = %v, want empty", before)
	}
	if line != "line 1" {
		t.Errorf("contextLine = %v, want 'line 1'", line)
	}
	if after != "line 2" {
		t.Errorf("contextAfter = %v, want 'line 2'", after)
	}
}

func TestGetLineContextLastLine(t *testing.T) {
	diffFiles = []DiffFile{
		{
			Path: "test.go",
			Hunks: []DiffHunk{
				{
					Lines: []DiffLine{
						{Type: LineContext, Content: "line 1", NewLineNo: 1},
						{Type: LineAdded, Content: "line 2", NewLineNo: 2},
					},
				},
			},
		},
	}

	before, line, after := getLineContext("test.go", 2)
	if before != "line 1" {
		t.Errorf("contextBefore = %v, want 'line 1'", before)
	}
	if line != "line 2" {
		t.Errorf("contextLine = %v, want 'line 2'", line)
	}
	if after != "" {
		t.Errorf("contextAfter = %v, want empty", after)
	}
}

func TestGetLineContextNotFound(t *testing.T) {
	diffFiles = []DiffFile{
		{
			Path: "test.go",
			Hunks: []DiffHunk{
				{
					Lines: []DiffLine{
						{Type: LineContext, Content: "line 1", NewLineNo: 1},
					},
				},
			},
		},
	}

	before, line, after := getLineContext("test.go", 999)
	if before != "" || line != "" || after != "" {
		t.Errorf("getLineContext() should return empty strings for non-existent line")
	}
}

func TestGetLineContextFileNotFound(t *testing.T) {
	diffFiles = []DiffFile{
		{
			Path: "test.go",
			Hunks: []DiffHunk{},
		},
	}

	before, line, after := getLineContext("nonexistent.go", 1)
	if before != "" || line != "" || after != "" {
		t.Errorf("getLineContext() should return empty strings for non-existent file")
	}
}
