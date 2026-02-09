package main

import (
	"code-review/model"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveAndLoadReview(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "review_state.json")

	review := model.NewReview("/repo/path", "feature", "main")
	diff := review.AddFileDiff("test.go")
	diff.AddComment("This needs refactoring", 10)
	diff.AddComment("Add error handling here", 25)

	err := SaveReview(statePath, review)
	if err != nil {
		t.Fatalf("Failed to save review: %v", err)
	}

	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		t.Fatal("Expected state file to exist")
	}

	loadedReview, err := LoadReview(statePath)
	if err != nil {
		t.Fatalf("Failed to load review: %v", err)
	}

	if loadedReview.ID != review.ID {
		t.Errorf("Expected ID %s, got %s", review.ID, loadedReview.ID)
	}

	if loadedReview.RepoPath != review.RepoPath {
		t.Errorf("Expected repo path %s, got %s", review.RepoPath, loadedReview.RepoPath)
	}

	if loadedReview.SourceBranch != review.SourceBranch {
		t.Errorf("Expected source branch %s, got %s", review.SourceBranch, loadedReview.SourceBranch)
	}

	if loadedReview.TargetBranch != review.TargetBranch {
		t.Errorf("Expected target branch %s, got %s", review.TargetBranch, loadedReview.TargetBranch)
	}

	if len(loadedReview.Files) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(loadedReview.Files))
	}

	if len(loadedReview.Files[0].Comments) != 2 {
		t.Fatalf("Expected 2 comments, got %d", len(loadedReview.Files[0].Comments))
	}

	if loadedReview.Files[0].Comments[0].Content != "This needs refactoring" {
		t.Errorf("Expected comment content to match")
	}
}

func TestLoadReviewNonExistent(t *testing.T) {
	_, err := LoadReview("/nonexistent/file.json")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestSaveReviewInvalidPath(t *testing.T) {
	review := model.NewReview("/repo", "feature", "main")
	err := SaveReview("/nonexistent/dir/file.json", review)
	if err == nil {
		t.Error("Expected error for invalid path")
	}
}

func TestGetXDGDataDir(t *testing.T) {
	t.Run("with XDG_DATA_HOME set", func(t *testing.T) {
		oldEnv := os.Getenv("XDG_DATA_HOME")
		defer os.Setenv("XDG_DATA_HOME", oldEnv)

		os.Setenv("XDG_DATA_HOME", "/custom/data")
		dataDir := GetXDGDataDir()

		expected := "/custom/data/code-review"
		if dataDir != expected {
			t.Errorf("Expected %s, got %s", expected, dataDir)
		}
	})

	t.Run("without XDG_DATA_HOME set", func(t *testing.T) {
		oldEnv := os.Getenv("XDG_DATA_HOME")
		defer os.Setenv("XDG_DATA_HOME", oldEnv)

		os.Unsetenv("XDG_DATA_HOME")
		dataDir := GetXDGDataDir()

		home, _ := os.UserHomeDir()
		expected := filepath.Join(home, ".local", "share", "code-review")
		if dataDir != expected {
			t.Errorf("Expected %s, got %s", expected, dataDir)
		}
	})
}

func TestGetReviewStatePath(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	repoPath := "/path/to/my-project"
	sourceBranch := "feature-branch"
	targetBranch := "main"

	statePath := GetReviewStatePath(dataDir, repoPath, sourceBranch, targetBranch)

	if !filepath.IsAbs(statePath) {
		t.Error("Expected absolute path")
	}

	dir := filepath.Dir(statePath)
	if dir != dataDir {
		t.Errorf("Expected state file in %s, got %s", dataDir, dir)
	}

	filename := filepath.Base(statePath)
	if filename == "" {
		t.Error("Expected non-empty filename")
	}

	if filepath.Ext(filename) != ".json" {
		t.Errorf("Expected .json extension, got %s", filepath.Ext(filename))
	}

	if !strings.Contains(filename, "feature-branch") {
		t.Error("Expected filename to contain source branch")
	}

	if !strings.Contains(filename, "main") {
		t.Error("Expected filename to contain target branch")
	}
}

func TestGetReviewStatePathDifferentBranches(t *testing.T) {
	dataDir := t.TempDir()
	repoPath := "/path/to/repo"

	path1 := GetReviewStatePath(dataDir, repoPath, "feature-1", "main")
	path2 := GetReviewStatePath(dataDir, repoPath, "feature-2", "main")
	path3 := GetReviewStatePath(dataDir, repoPath, "feature-1", "develop")

	if path1 == path2 {
		t.Error("Expected different paths for different source branches")
	}

	if path1 == path3 {
		t.Error("Expected different paths for different target branches")
	}

	if path2 == path3 {
		t.Error("Expected different paths for different branch combinations")
	}
}
