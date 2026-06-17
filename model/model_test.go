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

func TestReviewMarkFile(t *testing.T) {
	given := NewReview("/repo", "branch", "main")

	given.MarkFile("file.go")

	expected := true
	actual := given.IsFileMarked("file.go")
	if actual != expected {
		t.Errorf("Expected file to be marked: got %v, want %v", actual, expected)
	}

	if len(given.MarkedFiles) != 1 {
		t.Errorf("Expected 1 marked file, got %d", len(given.MarkedFiles))
	}
}

func TestReviewMarkFileIsIdempotent(t *testing.T) {
	given := NewReview("/repo", "branch", "main")

	given.MarkFile("file.go")
	given.MarkFile("file.go")

	expected := 1
	actual := len(given.MarkedFiles)
	if actual != expected {
		t.Errorf("Expected marking twice to add once: got %d, want %d", actual, expected)
	}
}

func TestReviewUnmarkFile(t *testing.T) {
	given := NewReview("/repo", "branch", "main")
	given.MarkFile("file.go")

	given.UnmarkFile("file.go")

	expected := false
	actual := given.IsFileMarked("file.go")
	if actual != expected {
		t.Errorf("Expected file to be unmarked: got %v, want %v", actual, expected)
	}

	if len(given.MarkedFiles) != 0 {
		t.Errorf("Expected 0 marked files, got %d", len(given.MarkedFiles))
	}
}

func TestReviewUnmarkAbsentFileIsNoOp(t *testing.T) {
	given := NewReview("/repo", "branch", "main")
	given.MarkFile("kept.go")

	given.UnmarkFile("never-marked.go")

	expected := 1
	actual := len(given.MarkedFiles)
	if actual != expected {
		t.Errorf("Expected unmarking an absent file to leave the set intact: got %d, want %d", actual, expected)
	}

	if !given.IsFileMarked("kept.go") {
		t.Error("Expected the originally marked file to remain marked")
	}
}

func TestReviewIsFileMarkedUnknown(t *testing.T) {
	given := NewReview("/repo", "branch", "main")

	expected := false
	actual := given.IsFileMarked("unknown.go")
	if actual != expected {
		t.Errorf("Expected unknown file to be unmarked: got %v, want %v", actual, expected)
	}
}

func TestAddReplyCreatesChildComment(t *testing.T) {
	given := NewFileDiff("test.go")
	root := given.AddComment("root", 5, "Test User")

	reply := given.AddReply(root.ID, "a reply", "Test User")

	if reply.ParentID != root.ID {
		t.Errorf("Expected reply ParentID '%s', got '%s'", root.ID, reply.ParentID)
	}
	if reply.Content != "a reply" {
		t.Errorf("Expected content 'a reply', got '%s'", reply.Content)
	}
	if reply.ID == "" {
		t.Error("Expected a generated ID, got empty string")
	}

	// the reply is stored flat alongside the root.
	expected := 2
	actual := len(given.Comments)
	if actual != expected {
		t.Errorf("Expected %d comments stored flat, got %d", expected, actual)
	}
}

func TestAddReplyPreservesOrder(t *testing.T) {
	given := NewFileDiff("test.go")
	root := given.AddComment("root", 5, "Test User")

	given.AddReply(root.ID, "first", "Test User")
	given.AddReply(root.ID, "second", "Test User")
	given.AddReply(root.ID, "third", "Test User")

	expected := []string{"first", "second", "third"}
	actual := []string{}
	for _, comment := range given.Comments {
		if comment.ParentID == root.ID {
			actual = append(actual, comment.Content)
		}
	}

	if len(actual) != len(expected) {
		t.Fatalf("Expected %d replies, got %d", len(expected), len(actual))
	}
	for i, want := range expected {
		if actual[i] != want {
			t.Errorf("Reply %d: got '%s', want '%s'", i, actual[i], want)
		}
	}
}

func TestAddReplyToReplyAttachesToRoot(t *testing.T) {
	given := NewFileDiff("test.go")
	root := given.AddComment("root", 5, "Test User")
	reply := given.AddReply(root.ID, "first reply", "Test User")

	// the thread stays flat: a reply to a reply hangs off the same root.
	nested := given.AddReply(reply.ID, "reply to a reply", "Test User")

	expected := root.ID
	actual := nested.ParentID
	if actual != expected {
		t.Errorf("Expected nested reply to attach to root '%s', got '%s'", expected, actual)
	}
}

func TestDeleteRootCommentCascadesToReplies(t *testing.T) {
	given := NewFileDiff("test.go")
	root := given.AddComment("root", 5, "Test User")
	given.AddReply(root.ID, "a reply", "Test User")
	given.AddReply(root.ID, "another reply", "Test User")
	survivor := given.AddComment("unrelated", 9, "Test User")

	given.DeleteComment(root.ID)

	expected := 1
	actual := len(given.Comments)
	if actual != expected {
		t.Errorf("Expected only the unrelated comment to remain, got %d comments", actual)
	}
	if len(given.Comments) > 0 && given.Comments[0].ID != survivor.ID {
		t.Error("Expected the unrelated comment to survive the cascade")
	}
}

func TestDeleteReplyLeavesRoot(t *testing.T) {
	given := NewFileDiff("test.go")
	root := given.AddComment("root", 5, "Test User")
	reply := given.AddReply(root.ID, "a reply", "Test User")

	given.DeleteComment(reply.ID)

	if given.GetComment(root.ID) == nil {
		t.Error("Expected root comment to survive deletion of its reply")
	}
	if given.GetComment(reply.ID) != nil {
		t.Error("Expected reply to be deleted")
	}
}

func TestReviewAddComment(t *testing.T) {
	given := NewReview("/repo", "feature", "main")

	comment := given.AddComment("overall this looks good", "Test User")

	if comment.ParentID != "" {
		t.Errorf("expected a root comment, got ParentID %q", comment.ParentID)
	}
	if comment.LineNumber != 0 {
		t.Errorf("expected no line anchor, got line %d", comment.LineNumber)
	}
	if comment.Status != CommentStatusActive {
		t.Errorf("expected active status, got %v", comment.Status)
	}
	if len(given.Comments) != 1 {
		t.Errorf("expected 1 review comment, got %d", len(given.Comments))
	}
}

func TestReviewReplyAndDeleteCascade(t *testing.T) {
	given := NewReview("/repo", "feature", "main")
	root := given.AddComment("root note", "Test User")
	given.AddReply(root.ID, "a reply", "Test User")
	other := given.AddComment("unrelated note", "Test User")

	if len(given.Comments) != 3 {
		t.Fatalf("expected 3 comments, got %d", len(given.Comments))
	}

	// deleting the root removes its reply too, leaving the unrelated comment.
	given.DeleteComment(root.ID)

	if len(given.Comments) != 1 {
		t.Errorf("expected cascade to leave 1 comment, got %d", len(given.Comments))
	}
	if given.GetComment(other.ID) == nil {
		t.Error("expected the unrelated review comment to survive")
	}
}

func TestReviewCommentsInGetAllComments(t *testing.T) {
	given := NewReview("/repo", "feature", "main")
	file := given.AddFileDiff("a.go")
	file.AddComment("file comment", 3, "Test User")
	given.AddComment("review comment", "Test User")

	all := given.GetAllComments()

	expected := 2
	if len(all) != expected {
		t.Errorf("expected %d comments across file and review, got %d", expected, len(all))
	}
}
