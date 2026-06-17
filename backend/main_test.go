//go:build !js

package main

import (
	"code-review/model"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestApp_CommentStatusChanges(t *testing.T) {
	tmpDir := t.TempDir()

	app := &App{
		review:    model.NewReview("/tmp/repo", "feature", "main"),
		repoPath:  "/tmp/repo",
		dataDir:   tmpDir,
		statePath: filepath.Join(tmpDir, "test.json"),
	}

	filePath := "test.go"
	content := "test comment"
	lineNumber := 10

	err := app.AddComment(filePath, content, lineNumber, "before", "line", "after")
	if err != nil {
		t.Fatalf("AddComment failed: %v", err)
	}

	fileDiff := app.review.GetFileDiff(filePath)
	if fileDiff == nil {
		t.Fatal("FileDiff not found after adding comment")
	}

	if len(fileDiff.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(fileDiff.Comments))
	}

	comment := fileDiff.Comments[0]
	commentID := comment.ID

	if comment.Status != model.CommentStatusActive {
		t.Errorf("expected status Active, got %v", comment.Status)
	}

	err = app.ResolveComment(filePath, commentID)
	if err != nil {
		t.Fatalf("ResolveComment failed: %v", err)
	}

	if comment.Status != model.CommentStatusResolved {
		t.Errorf("expected status Resolved, got %v", comment.Status)
	}

	_, err = os.Stat(app.statePath)
	if err != nil {
		t.Errorf("state file not saved after ResolveComment: %v", err)
	}

	err = app.ReactivateComment(filePath, commentID)
	if err != nil {
		t.Fatalf("ReactivateComment failed: %v", err)
	}

	if comment.Status != model.CommentStatusActive {
		t.Errorf("expected status Active after reactivate, got %v", comment.Status)
	}

	err = app.IgnoreComment(filePath, commentID)
	if err != nil {
		t.Fatalf("IgnoreComment failed: %v", err)
	}

	if comment.Status != model.CommentStatusIgnored {
		t.Errorf("expected status Ignored, got %v", comment.Status)
	}

	err = app.DeleteComment(filePath, commentID)
	if err != nil {
		t.Fatalf("DeleteComment failed: %v", err)
	}

	if len(fileDiff.Comments) != 0 {
		t.Errorf("expected 0 comments after delete, got %d", len(fileDiff.Comments))
	}
}

func TestApp_CommentStatusErrors(t *testing.T) {
	tmpDir := t.TempDir()

	app := &App{
		review:    model.NewReview("/tmp/repo", "feature", "main"),
		repoPath:  "/tmp/repo",
		dataDir:   tmpDir,
		statePath: filepath.Join(tmpDir, "test.json"),
	}

	err := app.ResolveComment("nonexistent.go", "fake-id")
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}

	app.review.AddFileDiff("test.go")

	err = app.ResolveComment("test.go", "fake-id")
	if err == nil {
		t.Error("expected error for nonexistent comment, got nil")
	}

	err = app.IgnoreComment("test.go", "fake-id")
	if err == nil {
		t.Error("expected error for nonexistent comment, got nil")
	}

	err = app.ReactivateComment("test.go", "fake-id")
	if err == nil {
		t.Error("expected error for nonexistent comment, got nil")
	}

	err = app.UpdateComment("test.go", "fake-id", "new content")
	if err == nil {
		t.Error("expected error for nonexistent comment, got nil")
	}
}

func TestApp_GetFileLines(t *testing.T) {
	tmp_dir := setupTestRepo(t)
	branch, err := GetCurrentBranch(tmp_dir)
	if err != nil {
		t.Fatalf("failed to get branch: %v", err)
	}

	app := &App{
		review:   model.NewReview(tmp_dir, branch, branch),
		repoPath: tmp_dir,
	}
	app.fileCache = newFileContentCache(func(rev, path string) (string, error) {
		return GetFileAtRevision(app.repoPath, rev, path)
	})

	t.Run("returns requested range and total", func(t *testing.T) {
		// the test repo's test.txt is a single line "initial content".
		raw, err := app.GetFileLines("test.txt", 1, 1, 0)
		if err != nil {
			t.Fatalf("GetFileLines failed: %v", err)
		}

		var result struct {
			Lines    []DiffLine
			TotalNew int
		}
		if err := json.Unmarshal([]byte(raw), &result); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		if result.TotalNew != 1 {
			t.Errorf("expected total 1, got %d", result.TotalNew)
		}
		if len(result.Lines) != 1 || result.Lines[0].Content != "initial content" {
			t.Errorf("expected single line 'initial content', got %+v", result.Lines)
		}
	})

	t.Run("missing path errors", func(t *testing.T) {
		_, err := app.GetFileLines("nonexistent.txt", 1, 1, 0)
		if err == nil {
			t.Error("expected error for missing path, got nil")
		}
	})
}

func TestApp_UpdateComment(t *testing.T) {
	tmpDir := t.TempDir()

	app := &App{
		review:    model.NewReview("/tmp/repo", "feature", "main"),
		repoPath:  "/tmp/repo",
		dataDir:   tmpDir,
		statePath: filepath.Join(tmpDir, "test.json"),
	}

	filePath := "test.go"
	originalContent := "original comment"
	lineNumber := 10

	err := app.AddComment(filePath, originalContent, lineNumber, "", "", "")
	if err != nil {
		t.Fatalf("AddComment failed: %v", err)
	}

	fileDiff := app.review.GetFileDiff(filePath)
	commentID := fileDiff.Comments[0].ID

	newContent := "updated comment"
	err = app.UpdateComment(filePath, commentID, newContent)
	if err != nil {
		t.Fatalf("UpdateComment failed: %v", err)
	}

	comment := fileDiff.GetComment(commentID)
	if comment.Content != newContent {
		t.Errorf("expected content %q, got %q", newContent, comment.Content)
	}
}

func TestApp_GetCommentedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	app := &App{
		review:    model.NewReview("/tmp/repo", "feature", "main"),
		repoPath:  "/tmp/repo",
		dataDir:   tmpDir,
		statePath: filepath.Join(tmpDir, "test.json"),
		// the overview iterates diffFiles for ordering; loadDiff normally fills
		// it from git, so set it directly for the test.
		diffFiles: []DiffFile{
			{Path: "a.go"},
			{Path: "b.go"},
			{Path: "c.go"},
		},
	}

	// comments on a.go and c.go only; b.go stays comment-free and must be omitted.
	if err := app.AddComment("c.go", "third", 3, "", "", ""); err != nil {
		t.Fatalf("AddComment failed: %v", err)
	}
	if err := app.AddComment("a.go", "first", 1, "", "", ""); err != nil {
		t.Fatalf("AddComment failed: %v", err)
	}

	data, err := app.GetCommentedFiles()
	if err != nil {
		t.Fatalf("GetCommentedFiles failed: %v", err)
	}

	var actual []struct {
		Path     string           `json:"path"`
		Comments []*model.Comment `json:"comments"`
	}
	if err := json.Unmarshal([]byte(data), &actual); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	// only the two commented files, in diff order (a before c), b.go omitted.
	expectedPaths := []string{"a.go", "c.go"}
	if len(actual) != len(expectedPaths) {
		t.Fatalf("expected %d commented files, got %d", len(expectedPaths), len(actual))
	}
	for i, expected := range expectedPaths {
		if actual[i].Path != expected {
			t.Errorf("file %d: expected path %q, got %q", i, expected, actual[i].Path)
		}
		if len(actual[i].Comments) != 1 {
			t.Errorf("file %q: expected 1 comment, got %d", actual[i].Path, len(actual[i].Comments))
		}
	}
}

func TestApp_GetCommentedFilesEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	app := &App{
		review:    model.NewReview("/tmp/repo", "feature", "main"),
		repoPath:  "/tmp/repo",
		dataDir:   tmpDir,
		statePath: filepath.Join(tmpDir, "test.json"),
		diffFiles: []DiffFile{{Path: "a.go"}},
	}

	data, err := app.GetCommentedFiles()
	if err != nil {
		t.Fatalf("GetCommentedFiles failed: %v", err)
	}
	if data != "[]" {
		t.Errorf("expected empty array, got %q", data)
	}
}

func TestApp_ReviewLevelComments(t *testing.T) {
	tmpDir := t.TempDir()
	app := &App{
		review:    model.NewReview("/tmp/repo", "feature", "main"),
		repoPath:  "/tmp/repo",
		dataDir:   tmpDir,
		statePath: filepath.Join(tmpDir, "test.json"),
		userName:  "Test User",
	}

	// add a review-level comment (no file anchor).
	if err := app.AddReviewComment("overall feedback"); err != nil {
		t.Fatalf("AddReviewComment failed: %v", err)
	}
	if len(app.review.Comments) != 1 {
		t.Fatalf("expected 1 review comment, got %d", len(app.review.Comments))
	}
	commentID := app.review.Comments[0].ID

	// the empty filePath routes status/reply/delete to the review level.
	if err := app.AddReply("", commentID, "a reply"); err != nil {
		t.Fatalf("AddReply (review-level) failed: %v", err)
	}
	if err := app.ResolveComment("", commentID); err != nil {
		t.Fatalf("ResolveComment (review-level) failed: %v", err)
	}
	if app.review.GetComment(commentID).Status != model.CommentStatusResolved {
		t.Errorf("expected review comment resolved, got %v", app.review.GetComment(commentID).Status)
	}

	// the state file is persisted with the review-level comment and its reply.
	if _, err := os.Stat(app.statePath); err != nil {
		t.Errorf("state not saved: %v", err)
	}
	if len(app.review.Comments) != 2 {
		t.Errorf("expected comment plus reply, got %d", len(app.review.Comments))
	}

	// deleting the root cascades to the reply.
	if err := app.DeleteComment("", commentID); err != nil {
		t.Fatalf("DeleteComment (review-level) failed: %v", err)
	}
	if len(app.review.Comments) != 0 {
		t.Errorf("expected cascade to empty review comments, got %d", len(app.review.Comments))
	}
}

func TestApp_GetReviewComments(t *testing.T) {
	tmpDir := t.TempDir()
	app := &App{
		review:    model.NewReview("/tmp/repo", "feature", "main"),
		repoPath:  "/tmp/repo",
		dataDir:   tmpDir,
		statePath: filepath.Join(tmpDir, "test.json"),
		userName:  "Test User",
	}

	if data, _ := app.GetReviewComments(); data != "[]" {
		t.Errorf("expected empty array before any comment, got %q", data)
	}

	app.AddReviewComment("note")

	data, err := app.GetReviewComments()
	if err != nil {
		t.Fatalf("GetReviewComments failed: %v", err)
	}
	var comments []*model.Comment
	if err := json.Unmarshal([]byte(data), &comments); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(comments) != 1 || comments[0].Content != "note" {
		t.Errorf("expected one review comment 'note', got %+v", comments)
	}
}
