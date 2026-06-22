package model

import (
	"encoding/json"
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

	if comment.CurrentLineNumber() != 10 {
		t.Errorf("Expected line number 10, got %d", comment.CurrentLineNumber())
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
	context := []string{"line before", "target line", "line after"}

	comment := NewCommentWithContext("test comment", 10, "Test Author", "blob-a", context, 1)

	if comment.ID == "" {
		t.Error("Expected comment to have an ID")
	}

	if comment.Content != "test comment" {
		t.Errorf("Expected content 'test comment', got '%s'", comment.Content)
	}

	if comment.CurrentLineNumber() != 10 {
		t.Errorf("Expected line number 10, got %d", comment.CurrentLineNumber())
	}

	if comment.Status != CommentStatusActive {
		t.Errorf("Expected status Active, got %s", comment.Status)
	}

	if len(comment.Anchors) != 1 {
		t.Fatalf("Expected 1 anchor, got %d", len(comment.Anchors))
	}
	anchor := comment.Anchors[0]
	if anchor.Blob != "blob-a" {
		t.Errorf("Expected anchor blob 'blob-a', got '%s'", anchor.Blob)
	}
	if len(anchor.Context) != len(context) {
		t.Fatalf("Expected %d context lines, got %d", len(context), len(anchor.Context))
	}
	for i, want := range context {
		if anchor.Context[i] != want {
			t.Errorf("Context[%d]: expected '%s', got '%s'", i, want, anchor.Context[i])
		}
	}
}

func TestFileDiffAddCommentWithContext(t *testing.T) {
	diff := NewFileDiff("file.go")
	context := []string{"before", "target", "after"}
	comment := diff.AddCommentWithContext("test comment", 5, "Test User", "blob-a", context, 1)

	if len(diff.Comments) != 1 {
		t.Fatalf("Expected 1 comment, got %d", len(diff.Comments))
	}

	if diff.Comments[0] != comment {
		t.Error("Expected added comment to be in comments list")
	}

	if len(diff.Comments[0].Anchors) != 1 {
		t.Fatalf("Expected 1 anchor, got %d", len(diff.Comments[0].Anchors))
	}
	if diff.Comments[0].Anchors[0].Context[1] != "target" {
		t.Errorf("Expected context centre 'target', got '%s'", diff.Comments[0].Anchors[0].Context[1])
	}
}

func TestReviewMarkFile(t *testing.T) {
	given := NewReview("/repo", "branch", "main")

	given.MarkFile("file.go", "sha1")

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

	given.MarkFile("file.go", "sha1")
	given.MarkFile("file.go", "sha1")

	expected := 1
	actual := len(given.MarkedFiles)
	if actual != expected {
		t.Errorf("Expected marking twice to add once: got %d, want %d", actual, expected)
	}
}

func TestReviewUnmarkFile(t *testing.T) {
	given := NewReview("/repo", "branch", "main")
	given.MarkFile("file.go", "sha1")

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
	given.MarkFile("kept.go", "sha1")

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
	if comment.CurrentLineNumber() != 0 {
		t.Errorf("expected no line anchor, got line %d", comment.CurrentLineNumber())
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

func TestEvictChangedMarksUnchangedStays(t *testing.T) {
	given := NewReview("/repo", "feature", "main")
	given.MarkFile("a.go", "sha-a")

	// the current SHA matches the stored one: the file is unchanged, so kept.
	given.EvictChangedMarks(map[string]string{"a.go": "sha-a"})

	if !given.IsFileMarked("a.go") {
		t.Error("expected an unchanged marked file to stay marked")
	}
}

func TestEvictChangedMarksChangedEvicts(t *testing.T) {
	given := NewReview("/repo", "feature", "main")
	given.MarkFile("a.go", "sha-a")
	given.MarkFile("b.go", "sha-b")

	// a.go's content changed (different SHA); b.go did not.
	given.EvictChangedMarks(map[string]string{"a.go": "sha-a-changed", "b.go": "sha-b"})

	if given.IsFileMarked("a.go") {
		t.Error("expected a changed marked file to be evicted")
	}
	if !given.IsFileMarked("b.go") {
		t.Error("expected an unchanged marked file to stay marked")
	}
}

func TestEvictChangedMarksDeletedEvicts(t *testing.T) {
	given := NewReview("/repo", "feature", "main")
	given.MarkFile("gone.go", "sha-gone")

	// gone.go is absent from the current SHAs: deleted at this revision, evict.
	given.EvictChangedMarks(map[string]string{})

	if given.IsFileMarked("gone.go") {
		t.Error("expected a deleted marked file to be evicted")
	}
}

func TestEvictChangedMarksLegacyBackfillsAndStays(t *testing.T) {
	given := NewReview("/repo", "feature", "main")
	// a legacy mark carries no blob SHA.
	given.MarkFile("a.go", "")

	given.EvictChangedMarks(map[string]string{"a.go": "sha-a"})

	if !given.IsFileMarked("a.go") {
		t.Fatal("expected a legacy mark to be backfilled and kept on first open")
	}
	if given.MarkedFiles[0].Blob != "sha-a" {
		t.Errorf("expected legacy mark backfilled to sha-a, got %q", given.MarkedFiles[0].Blob)
	}

	// once backfilled, a later content change evicts it.
	given.EvictChangedMarks(map[string]string{"a.go": "sha-a-changed"})
	if given.IsFileMarked("a.go") {
		t.Error("expected a backfilled mark to evict when the file later changes")
	}
}

func TestFileCommentStatus(t *testing.T) {
	active := &Comment{ID: "1", Status: CommentStatusActive}
	resolved := &Comment{ID: "2", Status: CommentStatusResolved}
	ignored := &Comment{ID: "3", Status: CommentStatusIgnored}
	reply := &Comment{ID: "4", ParentID: "2", Status: CommentStatusActive}

	cases := []struct {
		given    []*Comment
		expected string
	}{
		{nil, "none"},
		{[]*Comment{active}, "active"},
		{[]*Comment{resolved}, "resolved"},
		{[]*Comment{ignored}, "ignored"},
		{[]*Comment{active, resolved}, "active"},
		{[]*Comment{resolved, ignored}, "ignored"},
		// a reply's status never counts; only the resolved root does.
		{[]*Comment{resolved, reply}, "resolved"},
	}

	for _, c := range cases {
		actual := FileCommentStatus(c.given)
		if actual != c.expected {
			t.Errorf("FileCommentStatus(%v) = %q, want %q", c.given, actual, c.expected)
		}
	}
}

func TestCommentRootLine(t *testing.T) {
	root := &Comment{ID: "root", Anchors: []Anchor{{LineNumber: 42}}}
	reply := &Comment{ID: "reply", ParentID: "root"}
	comments := []*Comment{root, reply}

	if actual := CommentRootLine(comments, "root"); actual != 42 {
		t.Errorf("root line = %d, want 42", actual)
	}
	// a reply reports its root's line, not its own.
	if actual := CommentRootLine(comments, "reply"); actual != 42 {
		t.Errorf("reply's root line = %d, want 42", actual)
	}
	// an absent comment reports 0.
	if actual := CommentRootLine(comments, "missing"); actual != 0 {
		t.Errorf("missing comment line = %d, want 0", actual)
	}
}

func TestMarkedFilesUnmarshalNewShape(t *testing.T) {
	var actual MarkedFiles
	given := `[{"path":"a.go","blob":"sha-a"},{"path":"b.go"}]`
	if err := json.Unmarshal([]byte(given), &actual); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(actual) != 2 {
		t.Fatalf("expected 2 marks, got %d", len(actual))
	}
	if actual[0].Path != "a.go" || actual[0].Blob != "sha-a" {
		t.Errorf("first mark = %+v, want {a.go sha-a}", actual[0])
	}
	if actual[1].Path != "b.go" || actual[1].Blob != "" {
		t.Errorf("second mark = %+v, want {b.go }", actual[1])
	}
}

func TestMarkedFilesUnmarshalLegacyShape(t *testing.T) {
	var actual MarkedFiles
	given := `["a.go","b.go"]`
	if err := json.Unmarshal([]byte(given), &actual); err != nil {
		t.Fatalf("unmarshal of legacy bare-path form failed: %v", err)
	}
	if len(actual) != 2 {
		t.Fatalf("expected 2 marks, got %d", len(actual))
	}
	// legacy paths become records with an empty blob, flagged for backfill.
	for _, mark := range actual {
		if mark.Blob != "" {
			t.Errorf("expected legacy mark %q to have empty blob, got %q", mark.Path, mark.Blob)
		}
	}
	if actual[0].Path != "a.go" || actual[1].Path != "b.go" {
		t.Errorf("legacy paths not preserved: %+v", actual)
	}
}

func TestCommentCurrentLineNumber(t *testing.T) {
	good := Anchor{Blob: "b1", LineNumber: 10, Context: []string{"a", "b", "c"}}
	good2 := Anchor{Blob: "b2", LineNumber: 14, Context: []string{"a", "b", "c"}}
	adrift := Anchor{Blob: "b3"}

	cases := []struct {
		name     string
		given    []Anchor
		expected int
	}{
		{"good-only history reports its latest line", []Anchor{good, good2}, 14},
		{"trailing adrift reports 0", []Anchor{good, adrift}, 0},
		{"all adrift reports 0", []Anchor{adrift}, 0},
		{"no anchors reports 0", nil, 0},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			given := &Comment{Anchors: c.given}
			actual := given.CurrentLineNumber()
			if actual != c.expected {
				t.Errorf("CurrentLineNumber() = %d, want %d", actual, c.expected)
			}
		})
	}
}

func TestCommentIsOutdated(t *testing.T) {
	good := Anchor{Blob: "b1", LineNumber: 10, Context: []string{"a", "b", "c"}}
	adrift := Anchor{Blob: "b2"}

	cases := []struct {
		name     string
		given    []Anchor
		expected bool
	}{
		{"good-only history is not outdated", []Anchor{good}, false},
		{"trailing adrift is outdated", []Anchor{good, adrift}, true},
		{"all adrift is outdated", []Anchor{adrift}, true},
		{"no anchors is never outdated", nil, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			given := &Comment{Anchors: c.given}
			actual := given.IsOutdated()
			if actual != c.expected {
				t.Errorf("IsOutdated() = %v, want %v", actual, c.expected)
			}
		})
	}
}

func TestCommentLastGoodContext(t *testing.T) {
	good1 := Anchor{Blob: "b1", LineNumber: 10, Context: []string{"x", "y", "z"}}
	good2 := Anchor{Blob: "b2", LineNumber: 12, Context: []string{"p", "q", "r"}}
	adrift := Anchor{Blob: "b3"}

	t.Run("good-only history returns the most recent good context", func(t *testing.T) {
		given := &Comment{Anchors: []Anchor{good1, good2}}
		actual := given.LastGoodContext()
		expected := []string{"p", "q", "r"}
		if !equalStrings(actual, expected) {
			t.Errorf("LastGoodContext() = %v, want %v", actual, expected)
		}
	})

	t.Run("trailing adrift falls back to the last good context", func(t *testing.T) {
		given := &Comment{Anchors: []Anchor{good1, adrift}}
		actual := given.LastGoodContext()
		expected := []string{"x", "y", "z"}
		if !equalStrings(actual, expected) {
			t.Errorf("LastGoodContext() = %v, want %v", actual, expected)
		}
	})

	t.Run("all adrift returns nil", func(t *testing.T) {
		given := &Comment{Anchors: []Anchor{adrift}}
		if actual := given.LastGoodContext(); actual != nil {
			t.Errorf("LastGoodContext() = %v, want nil", actual)
		}
	})

	t.Run("no anchors returns nil", func(t *testing.T) {
		given := &Comment{}
		if actual := given.LastGoodContext(); actual != nil {
			t.Errorf("LastGoodContext() = %v, want nil", actual)
		}
	})
}

func TestCommentUnmarshalJSONLegacyUpgrade(t *testing.T) {
	given := `{
		"id": "c1",
		"author": "Test User",
		"content": "legacy note",
		"status": "active",
		"line_number": 7,
		"context_before": "before",
		"context_line": "the line",
		"context_after": "after"
	}`

	var actual Comment
	if err := json.Unmarshal([]byte(given), &actual); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(actual.Anchors) != 1 {
		t.Fatalf("expected legacy comment to upgrade to one anchor, got %d", len(actual.Anchors))
	}
	anchor := actual.Anchors[0]
	if anchor.Blob != "" {
		t.Errorf("expected upgraded anchor to carry an empty blob, got %q", anchor.Blob)
	}
	if anchor.LineNumber != 7 {
		t.Errorf("expected upgraded anchor line 7, got %d", anchor.LineNumber)
	}
	expectedContext := []string{"before", "the line", "after"}
	if !equalStrings(anchor.Context, expectedContext) {
		t.Errorf("expected upgraded context %v, got %v", expectedContext, anchor.Context)
	}
}

func TestCommentUnmarshalJSONAnchorFormUnchanged(t *testing.T) {
	given := `{
		"id": "c1",
		"author": "Test User",
		"content": "note",
		"status": "active",
		"anchors": [{"blob": "sha-a", "line_number": 12, "context": ["a", "b", "c"]}]
	}`

	var actual Comment
	if err := json.Unmarshal([]byte(given), &actual); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(actual.Anchors) != 1 {
		t.Fatalf("expected anchor-form comment to load one anchor, got %d", len(actual.Anchors))
	}
	anchor := actual.Anchors[0]
	if anchor.Blob != "sha-a" || anchor.LineNumber != 12 {
		t.Errorf("expected {sha-a 12}, got {%s %d}", anchor.Blob, anchor.LineNumber)
	}
	if !equalStrings(anchor.Context, []string{"a", "b", "c"}) {
		t.Errorf("expected context [a b c], got %v", anchor.Context)
	}
}

func TestCommentUnmarshalJSONReplyGetsNoAnchor(t *testing.T) {
	// a reply carries line_number 0 and no context: it must not be upgraded.
	given := `{
		"id": "r1",
		"parent_id": "c1",
		"author": "Test User",
		"content": "a reply",
		"status": "active",
		"line_number": 0
	}`

	var actual Comment
	if err := json.Unmarshal([]byte(given), &actual); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(actual.Anchors) != 0 {
		t.Errorf("expected a reply to load with no anchors, got %d", len(actual.Anchors))
	}
}

// report whether two string slices have equal length and equal elements.
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
