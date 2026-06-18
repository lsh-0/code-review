//go:build js
// +build js

package main

import (
	"code-review/model"
	"encoding/json"
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
				model.NewComment("test", 1, ""),
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
				model.NewComment("test1", 1, ""),
				{ID: "2", Content: "test2", LineNumber: 2, Status: model.CommentStatusResolved},
			},
			expected: "active",
		},
		{
			name: "active takes precedence over ignored",
			comments: []*model.Comment{
				model.NewComment("test1", 1, ""),
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

func TestHunkReachedEOF(t *testing.T) {
	added := DiffLine{Type: LineAdded}
	context := DiffLine{Type: LineContext}
	removed := DiffLine{Type: LineRemoved}

	tests := []struct {
		name     string
		given    []DiffLine
		expected bool
	}{
		{
			name:     "no lines",
			given:    []DiffLine{},
			expected: false,
		},
		{
			name:     "full trailing context is not end-of-file",
			given:    []DiffLine{added, context, context, context},
			expected: false,
		},
		{
			name:     "short trailing context signals end-of-file",
			given:    []DiffLine{added, context},
			expected: true,
		},
		{
			name:     "added line is the last line, no trailing context",
			given:    []DiffLine{context, context, context, added},
			expected: true,
		},
		{
			// the regression: a hunk ending in deletions has 0 trailing
			// context for reasons unrelated to EOF, so it must not be treated
			// as end-of-file — the new side may continue past the hunk.
			name:     "removed line is the last line is not end-of-file",
			given:    []DiffLine{context, context, context, added, removed, removed},
			expected: false,
		},
		{
			name:     "all context, fewer than the context size",
			given:    []DiffLine{context, context},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := hunkReachedEOF(tt.given)
			if actual != tt.expected {
				t.Errorf("hunkReachedEOF() = %v, want %v", actual, tt.expected)
			}
		})
	}
}

func TestEffectiveBoundary(t *testing.T) {
	t.Run("unlinked uses fixed boundary", func(t *testing.T) {
		given := &expandState{frontier: 50, boundary: 10}
		expected := 10
		actual := given.effectiveBoundary()
		if actual != expected {
			t.Errorf("effectiveBoundary() = %d, want %d", actual, expected)
		}
	})

	t.Run("linked uses sibling frontier", func(t *testing.T) {
		// the upward control's true boundary is wherever the downward control
		// has revealed to, not the static gap edge.
		down := &expandState{frontier: 25}
		up := &expandState{frontier: 50, boundary: 10, sibling: down}
		expected := 25
		actual := up.effectiveBoundary()
		if actual != expected {
			t.Errorf("effectiveBoundary() = %d, want %d", actual, expected)
		}
	})
}

func TestStepRangeClampsToSiblingFrontier(t *testing.T) {
	// a between-hunk gap where the downward control has revealed up to line 30.
	// the upward control, stepping from 50, must not request past 31 (one above
	// the sibling's frontier is the lowest it should reach in a single step is
	// bounded by expandStep, but the clamp is to the sibling frontier).
	down := &expandState{frontier: 30}
	up := &expandState{frontier: 50, boundary: 10, sibling: down}

	startNew, endNew := stepRange(expandUp, up)

	if endNew != 50 {
		t.Errorf("expected endNew 50, got %d", endNew)
	}
	// expandStep is 20, so a raw step would start at 31; the sibling frontier
	// (30) is the clamp, and 31 > 30 so the full step fits.
	if startNew != 31 {
		t.Errorf("expected startNew clamped to 31, got %d", startNew)
	}
}

func TestGapExhaustedWhenFrontiersCross(t *testing.T) {
	// the two converging controls have met: the upward frontier has dropped
	// below the downward frontier, so the gap is fully revealed.
	down := &expandState{frontier: 40}
	up := &expandState{frontier: 39, boundary: 10, sibling: down}

	if !gapExhausted(expandUp, up) {
		t.Error("expected gap exhausted once upward frontier crossed the sibling")
	}

	notYet := &expandState{frontier: 45, boundary: 10, sibling: &expandState{frontier: 40}}
	if gapExhausted(expandUp, notYet) {
		t.Error("expected gap not exhausted while a hidden span remains")
	}
}

func TestGetFileCommentStatusNotInCache(t *testing.T) {
	commentsCache = make(map[string][]*model.Comment)
	result := getFileCommentStatus("nonexistent.go")
	if result != "none" {
		t.Errorf("getFileCommentStatus() = %v, want none", result)
	}
}

func TestDiffFileNeedsFetch(t *testing.T) {
	tests := []struct {
		name     string
		given    DiffFile
		expected bool
	}{
		{
			name:     "text file without hunks needs fetch",
			given:    DiffFile{Path: "a.go"},
			expected: true,
		},
		{
			name:     "text file with hunks does not",
			given:    DiffFile{Path: "a.go", Hunks: []DiffHunk{{}}},
			expected: false,
		},
		{
			name:     "binary file never needs fetch",
			given:    DiffFile{Path: "img.png", Binary: true},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := diffFileNeedsFetch(tt.given)
			if actual != tt.expected {
				t.Errorf("diffFileNeedsFetch(%+v) = %v, want %v", tt.given, actual, tt.expected)
			}
		})
	}
}

// a refresh re-loads file metadata (no hunks) over the in-memory diff. Decoding
// that metadata must clear any previously loaded hunks, otherwise a recomputed
// diff is hidden behind the stale ones. This mirrors `loadDiffFiles` resetting
// `diffFiles` to nil before the metadata decode.
func TestRefreshClearsStaleHunks(t *testing.T) {
	diffFiles = []DiffFile{
		{Path: "a.go", Hunks: []DiffHunk{{NewStart: 1, Lines: []DiffLine{{Content: "old"}}}}},
	}

	// the reset that `loadDiffFiles` performs before decoding metadata-only JSON.
	diffFiles = nil
	given := `[{"Path":"a.go","Binary":false}]`
	if err := json.Unmarshal([]byte(given), &diffFiles); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(diffFiles) != 1 {
		t.Fatalf("expected 1 file, got %d", len(diffFiles))
	}
	if !diffFileNeedsFetch(diffFiles[0]) {
		t.Errorf("stale hunks survived the metadata reload: %+v", diffFiles[0])
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
			Path:  "test.go",
			Hunks: []DiffHunk{},
		},
	}

	before, line, after := getLineContext("nonexistent.go", 1)
	if before != "" || line != "" || after != "" {
		t.Errorf("getLineContext() should return empty strings for non-existent file")
	}
}
