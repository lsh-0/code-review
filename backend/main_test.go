//go:build !js
// +build !js

package main

import (
	"code-review/model"
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
