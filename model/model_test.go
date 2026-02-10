package model

import (
	"testing"
)

func TestNewComment(t *testing.T) {
	comment := NewComment("This is a test comment", 10, "Test Author")

	if comment.ID == "" {
		t.Error("Expected comment to have an ID")
	}

	if comment.Author != "Test Author" {
		t.Errorf("Expected author 'Test Author', got '%s'", comment.Author)
	}

	if comment.Content != "This is a test comment" {
		t.Errorf("Expected content 'This is a test comment', got '%s'", comment.Content)
	}

	if comment.LineNumber != 10 {
		t.Errorf("Expected line number 10, got %d", comment.LineNumber)
	}

	if comment.Status != CommentStatusActive {
		t.Errorf("Expected status Active, got %s", comment.Status)
	}
}

func TestCommentResolve(t *testing.T) {
	comment := NewComment("test", 1, "Test User")
	comment.Resolve()

	if comment.Status != CommentStatusResolved {
		t.Errorf("Expected status Resolved, got %s", comment.Status)
	}
}

func TestCommentIgnore(t *testing.T) {
	comment := NewComment("test", 1, "Test User")
	comment.Ignore()

	if comment.Status != CommentStatusIgnored {
		t.Errorf("Expected status Ignored, got %s", comment.Status)
	}
}

func TestCommentReactivate(t *testing.T) {
	comment := NewComment("test", 1, "Test User")
	comment.Resolve()
	comment.Reactivate()

	if comment.Status != CommentStatusActive {
		t.Errorf("Expected status Active, got %s", comment.Status)
	}
}

func TestCommentUpdate(t *testing.T) {
	comment := NewComment("original", 1, "Test User")
	comment.UpdateContent("updated content")

	if comment.Content != "updated content" {
		t.Errorf("Expected content 'updated content', got '%s'", comment.Content)
	}
}

func TestNewFileDiff(t *testing.T) {
	diff := NewFileDiff("path/to/file.go")

	if diff.FilePath != "path/to/file.go" {
		t.Errorf("Expected file path 'path/to/file.go', got '%s'", diff.FilePath)
	}

	if len(diff.Comments) != 0 {
		t.Errorf("Expected no comments, got %d", len(diff.Comments))
	}
}

func TestFileDiffAddComment(t *testing.T) {
	diff := NewFileDiff("file.go")
	comment := diff.AddComment("test comment", 5, "Test User")

	if len(diff.Comments) != 1 {
		t.Fatalf("Expected 1 comment, got %d", len(diff.Comments))
	}

	if diff.Comments[0] != comment {
		t.Error("Expected added comment to be in comments list")
	}
}

func TestFileDiffGetComment(t *testing.T) {
	diff := NewFileDiff("file.go")
	comment := diff.AddComment("test", 5, "Test User")

	found := diff.GetComment(comment.ID)
	if found == nil {
		t.Fatal("Expected to find comment")
	}

	if found.ID != comment.ID {
		t.Errorf("Expected comment ID %s, got %s", comment.ID, found.ID)
	}

	notFound := diff.GetComment("nonexistent")
	if notFound != nil {
		t.Error("Expected nil for nonexistent comment")
	}
}

func TestFileDiffDeleteComment(t *testing.T) {
	diff := NewFileDiff("file.go")
	comment1 := diff.AddComment("comment1", 5, "Test User")
	comment2 := diff.AddComment("comment2", 10, "Test User")

	diff.DeleteComment(comment1.ID)

	if len(diff.Comments) != 1 {
		t.Errorf("Expected 1 comment after deletion, got %d", len(diff.Comments))
	}

	if diff.Comments[0].ID != comment2.ID {
		t.Error("Expected remaining comment to be comment2")
	}
}

func TestFileDiffGetCommentsByLine(t *testing.T) {
	diff := NewFileDiff("file.go")
	diff.AddComment("comment1", 5, "Test User")
	diff.AddComment("comment2", 5, "Test User")
	diff.AddComment("comment3", 10, "Test User")

	line5Comments := diff.GetCommentsByLine(5)
	if len(line5Comments) != 2 {
		t.Errorf("Expected 2 comments for line 5, got %d", len(line5Comments))
	}

	line10Comments := diff.GetCommentsByLine(10)
	if len(line10Comments) != 1 {
		t.Errorf("Expected 1 comment for line 10, got %d", len(line10Comments))
	}

	noComments := diff.GetCommentsByLine(99)
	if len(noComments) != 0 {
		t.Errorf("Expected 0 comments for line 99, got %d", len(noComments))
	}
}

func TestNewReview(t *testing.T) {
	review := NewReview("/path/to/repo", "feature-branch", "main")

	if review.ID == "" {
		t.Error("Expected review to have an ID")
	}

	if review.RepoPath != "/path/to/repo" {
		t.Errorf("Expected repo path '/path/to/repo', got '%s'", review.RepoPath)
	}

	if review.SourceBranch != "feature-branch" {
		t.Errorf("Expected source branch 'feature-branch', got '%s'", review.SourceBranch)
	}

	if review.TargetBranch != "main" {
		t.Errorf("Expected target branch 'main', got '%s'", review.TargetBranch)
	}

	if len(review.Files) != 0 {
		t.Errorf("Expected no files, got %d", len(review.Files))
	}
}

func TestReviewAddFileDiff(t *testing.T) {
	review := NewReview("/repo", "branch", "main")
	diff := review.AddFileDiff("file.go")

	if len(review.Files) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(review.Files))
	}

	if review.Files[0] != diff {
		t.Error("Expected added diff to be in files list")
	}
}

func TestReviewGetFileDiff(t *testing.T) {
	review := NewReview("/repo", "branch", "main")
	diff := review.AddFileDiff("file.go")

	found := review.GetFileDiff("file.go")
	if found == nil {
		t.Fatal("Expected to find file diff")
	}

	if found != diff {
		t.Error("Expected found diff to match added diff")
	}

	notFound := review.GetFileDiff("nonexistent.go")
	if notFound != nil {
		t.Error("Expected nil for nonexistent file")
	}
}

func TestReviewGetAllComments(t *testing.T) {
	review := NewReview("/repo", "branch", "main")

	diff1 := review.AddFileDiff("file1.go")
	diff1.AddComment("comment1", 5, "Test User")
	diff1.AddComment("comment2", 10, "Test User")

	diff2 := review.AddFileDiff("file2.go")
	diff2.AddComment("comment3", 3, "Test User")

	allComments := review.GetAllComments()
	if len(allComments) != 3 {
		t.Errorf("Expected 3 total comments, got %d", len(allComments))
	}
}

func TestReviewGetActiveCommentsCount(t *testing.T) {
	review := NewReview("/repo", "branch", "main")

	diff := review.AddFileDiff("file.go")
	comment1 := diff.AddComment("comment1", 5, "Test User")
	comment2 := diff.AddComment("comment2", 10, "Test User")
	diff.AddComment("comment3", 15, "Test User")

	comment1.Resolve()
	comment2.Ignore()

	activeCount := review.GetActiveCommentsCount()
	if activeCount != 1 {
		t.Errorf("Expected 1 active comment, got %d", activeCount)
	}
}

func TestGenerateID(t *testing.T) {
	id1 := GenerateID()
	id2 := GenerateID()

	if id1 == "" {
		t.Error("Expected non-empty ID")
	}

	if id1 == id2 {
		t.Error("Expected unique IDs")
	}

	if len(id1) != 16 {
		t.Errorf("Expected ID length 16, got %d", len(id1))
	}
}

func TestNewCommentWithContext(t *testing.T) {
	contextBefore := "line before"
	contextLine := "target line"
	contextAfter := "line after"

	comment := NewCommentWithContext("test comment", 10, "Test Author", contextBefore, contextLine, contextAfter)

	if comment.ID == "" {
		t.Error("Expected comment to have an ID")
	}

	if comment.Content != "test comment" {
		t.Errorf("Expected content 'test comment', got '%s'", comment.Content)
	}

	if comment.LineNumber != 10 {
		t.Errorf("Expected line number 10, got %d", comment.LineNumber)
	}

	if comment.Status != CommentStatusActive {
		t.Errorf("Expected status Active, got %s", comment.Status)
	}

	if comment.ContextBefore != contextBefore {
		t.Errorf("Expected context before '%s', got '%s'", contextBefore, comment.ContextBefore)
	}

	if comment.ContextLine != contextLine {
		t.Errorf("Expected context line '%s', got '%s'", contextLine, comment.ContextLine)
	}

	if comment.ContextAfter != contextAfter {
		t.Errorf("Expected context after '%s', got '%s'", contextAfter, comment.ContextAfter)
	}
}

func TestFileDiffAddCommentWithContext(t *testing.T) {
	diff := NewFileDiff("file.go")
	comment := diff.AddCommentWithContext("test comment", 5, "Test User", "before", "target", "after")

	if len(diff.Comments) != 1 {
		t.Fatalf("Expected 1 comment, got %d", len(diff.Comments))
	}

	if diff.Comments[0] != comment {
		t.Error("Expected added comment to be in comments list")
	}

	if diff.Comments[0].ContextLine != "target" {
		t.Errorf("Expected context line 'target', got '%s'", diff.Comments[0].ContextLine)
	}
}
