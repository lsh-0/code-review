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
