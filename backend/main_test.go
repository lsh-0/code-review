package main

import (
	"code-review/model"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestApp_CommentStatusChanges(t *testing.T) {
	tmpDir := t.TempDir()

	app := &App{
		review:    model.NewReview("/some/repo/path", "feature", "main"),
		repoPath:  "/some/repo/path",
		dataDir:   tmpDir,
		statePath: filepath.Join(tmpDir, "test.json"),
	}

	filePath := "test.go"
	content := "test comment"
	lineNumber := 10

	_, err := app.AddComment(filePath, content, lineNumber, []string{"before", "line", "after"}, 1)
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

	_, err = app.ResolveComment(filePath, commentID)
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

	_, err = app.ReactivateComment(filePath, commentID)
	if err != nil {
		t.Fatalf("ReactivateComment failed: %v", err)
	}

	if comment.Status != model.CommentStatusActive {
		t.Errorf("expected status Active after reactivate, got %v", comment.Status)
	}

	_, err = app.IgnoreComment(filePath, commentID)
	if err != nil {
		t.Fatalf("IgnoreComment failed: %v", err)
	}

	if comment.Status != model.CommentStatusIgnored {
		t.Errorf("expected status Ignored, got %v", comment.Status)
	}

	_, err = app.DeleteComment(filePath, commentID)
	if err != nil {
		t.Fatalf("DeleteComment failed: %v", err)
	}

	if len(fileDiff.Comments) != 0 {
		t.Errorf("expected 0 comments after delete, got %d", len(fileDiff.Comments))
	}
}

// `GetDiffFiles` returns only path and binary metadata, never hunks, so the
// large per-line content is not shipped in the file-list payload.
func TestApp_GetDiffFiles_MetadataOnly(t *testing.T) {
	app := &App{
		diffFiles: []DiffFile{
			{Path: "a.go", Binary: false, Hunks: []DiffHunk{{NewStart: 1, Lines: []DiffLine{{Content: "x"}}}}},
			{Path: "img.png", Binary: true},
		},
	}

	raw, err := app.GetDiffFiles()
	if err != nil {
		t.Fatalf("GetDiffFiles failed: %v", err)
	}

	var actual []map[string]any
	if err := json.Unmarshal([]byte(raw), &actual); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(actual) != 2 {
		t.Fatalf("expected 2 files, got %d", len(actual))
	}
	if _, present := actual[0]["Hunks"]; present {
		t.Errorf("expected no Hunks key in metadata, got %v", actual[0])
	}
	if actual[0]["Path"] != "a.go" || actual[1]["Binary"] != true {
		t.Errorf("unexpected metadata: %v", actual)
	}
}

// `GetFileDiff` returns one file's full hunks by path, and a null result for an
// unknown path.
func TestApp_GetFileDiff_ByPath(t *testing.T) {
	given := DiffFile{Path: "a.go", Hunks: []DiffHunk{{NewStart: 1, Lines: []DiffLine{{Content: "x"}}}}}
	app := &App{diffFiles: []DiffFile{given}}

	raw, err := app.GetFileDiff("a.go")
	if err != nil {
		t.Fatalf("GetFileDiff failed: %v", err)
	}

	var actual DiffFile
	if err := json.Unmarshal([]byte(raw), &actual); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(actual.Hunks) != 1 || actual.Hunks[0].Lines[0].Content != "x" {
		t.Errorf("expected the file's hunks, got %+v", actual)
	}

	missing, err := app.GetFileDiff("nope.go")
	if err != nil {
		t.Fatalf("GetFileDiff(missing) failed: %v", err)
	}
	if missing != "null" {
		t.Errorf("expected null for unknown path, got %q", missing)
	}
}

func TestApp_CommentStatusErrors(t *testing.T) {
	tmpDir := t.TempDir()

	app := &App{
		review:    model.NewReview("/some/repo/path", "feature", "main"),
		repoPath:  "/some/repo/path",
		dataDir:   tmpDir,
		statePath: filepath.Join(tmpDir, "test.json"),
	}

	_, err := app.ResolveComment("nonexistent.go", "fake-id")
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}

	app.review.AddFileDiff("test.go")

	_, err = app.ResolveComment("test.go", "fake-id")
	if err == nil {
		t.Error("expected error for nonexistent comment, got nil")
	}

	_, err = app.IgnoreComment("test.go", "fake-id")
	if err == nil {
		t.Error("expected error for nonexistent comment, got nil")
	}

	_, err = app.ReactivateComment("test.go", "fake-id")
	if err == nil {
		t.Error("expected error for nonexistent comment, got nil")
	}

	_, err = app.UpdateComment("test.go", "fake-id", "new content")
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
		review:    model.NewReview("/some/repo/path", "feature", "main"),
		repoPath:  "/some/repo/path",
		dataDir:   tmpDir,
		statePath: filepath.Join(tmpDir, "test.json"),
	}

	filePath := "test.go"
	originalContent := "original comment"
	lineNumber := 10

	_, err := app.AddComment(filePath, originalContent, lineNumber, nil, 0)
	if err != nil {
		t.Fatalf("AddComment failed: %v", err)
	}

	fileDiff := app.review.GetFileDiff(filePath)
	commentID := fileDiff.Comments[0].ID

	newContent := "updated comment"
	_, err = app.UpdateComment(filePath, commentID, newContent)
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
		review:    model.NewReview("/some/repo/path", "feature", "main"),
		repoPath:  "/some/repo/path",
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
	if _, err := app.AddComment("c.go", "third", 3, nil, 0); err != nil {
		t.Fatalf("AddComment failed: %v", err)
	}
	if _, err := app.AddComment("a.go", "first", 1, nil, 0); err != nil {
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
		review:    model.NewReview("/some/repo/path", "feature", "main"),
		repoPath:  "/some/repo/path",
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
		review:    model.NewReview("/some/repo/path", "feature", "main"),
		repoPath:  "/some/repo/path",
		dataDir:   tmpDir,
		statePath: filepath.Join(tmpDir, "test.json"),
		userName:  "Test User",
	}

	// add a review-level comment (no file anchor).
	if _, err := app.AddReviewComment("overall feedback"); err != nil {
		t.Fatalf("AddReviewComment failed: %v", err)
	}
	if len(app.review.Comments) != 1 {
		t.Fatalf("expected 1 review comment, got %d", len(app.review.Comments))
	}
	commentID := app.review.Comments[0].ID

	// the empty filePath routes status/reply/delete to the review level.
	if _, err := app.AddReply("", commentID, "a reply"); err != nil {
		t.Fatalf("AddReply (review-level) failed: %v", err)
	}
	if _, err := app.ResolveComment("", commentID); err != nil {
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
	if _, err := app.DeleteComment("", commentID); err != nil {
		t.Fatalf("DeleteComment (review-level) failed: %v", err)
	}
	if len(app.review.Comments) != 0 {
		t.Errorf("expected cascade to empty review comments, got %d", len(app.review.Comments))
	}
}

func TestApp_GetReviewComments(t *testing.T) {
	tmpDir := t.TempDir()
	app := &App{
		review:    model.NewReview("/some/repo/path", "feature", "main"),
		repoPath:  "/some/repo/path",
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

// RecomputeDiff is the only path that shells out to git: it must populate
// diffFiles from the branch diff.
func TestApp_RecomputeDiff_PopulatesFromGit(t *testing.T) {
	tmpDir := setupTestRepo(t)

	baseBranch, err := GetCurrentBranch(tmpDir)
	if err != nil {
		t.Fatalf("failed to get base branch: %v", err)
	}

	checkout := exec.Command("git", "checkout", "-b", "feature")
	checkout.Dir = tmpDir
	if err := checkout.Run(); err != nil {
		t.Fatalf("failed to create feature branch: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("initial content\nadded line\n"), 0644); err != nil {
		t.Fatalf("failed to modify file: %v", err)
	}
	add := exec.Command("git", "add", "test.txt")
	add.Dir = tmpDir
	if err := add.Run(); err != nil {
		t.Fatalf("failed to add file: %v", err)
	}
	commit := exec.Command("git", "commit", "-m", "change")
	commit.Dir = tmpDir
	if err := commit.Run(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	headBranch, _ := GetCurrentBranch(tmpDir)

	app := &App{
		review:    model.NewReview(tmpDir, headBranch, baseBranch),
		repoPath:  tmpDir,
		dataDir:   t.TempDir(),
		statePath: filepath.Join(t.TempDir(), "state.json"),
	}

	if err := app.RecomputeDiff(); err != nil {
		t.Fatalf("RecomputeDiff failed: %v", err)
	}

	if len(app.diffFiles) == 0 {
		t.Fatal("expected RecomputeDiff to populate diffFiles from the git diff")
	}
	if app.diffFiles[0].Path != "test.txt" {
		t.Errorf("expected diff for test.txt, got %q", app.diffFiles[0].Path)
	}
}

// ReloadReview is the cheap reload: it re-reads the state JSON into a.review and
// does no git work, leaving the already-computed diff untouched. A path that is
// not a git repository proves no git subprocess ran.
func TestApp_ReloadReview_NoGit(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	// persist a review with one marked file to disk.
	onDisk := model.NewReview("/not/a/repo", "feature", "main")
	onDisk.MarkFile("a.go", "sha-a")
	if err := SaveReview(statePath, onDisk); err != nil {
		t.Fatalf("failed to seed state file: %v", err)
	}

	// the app starts with a different in-memory review and a sentinel diff that
	// ReloadReview must not disturb.
	sentinel := []DiffFile{{Path: "sentinel.go"}}
	app := &App{
		review:    model.NewReview("/not/a/repo", "feature", "main"),
		repoPath:  "/not/a/repo", // not a git repo: any git call would fail
		dataDir:   tmpDir,
		statePath: statePath,
		diffFiles: sentinel,
	}

	if err := app.ReloadReview(); err != nil {
		t.Fatalf("ReloadReview failed: %v", err)
	}

	if !app.review.IsFileMarked("a.go") {
		t.Error("expected ReloadReview to load the marked file from disk")
	}
	if len(app.diffFiles) != 1 || app.diffFiles[0].Path != "sentinel.go" {
		t.Errorf("expected ReloadReview to leave diffFiles untouched, got %+v", app.diffFiles)
	}
}

func TestApp_MutationResultShape(t *testing.T) {
	tmpDir := t.TempDir()
	app := &App{
		review:    model.NewReview("/some/repo/path", "feature", "main"),
		repoPath:  "/some/repo/path",
		dataDir:   tmpDir,
		statePath: filepath.Join(tmpDir, "test.json"),
		userName:  "Test User",
	}

	// adding a comment returns a result anchored at its line, carrying the file's
	// comments and an active status.
	raw, err := app.AddComment("a.go", "note", 7, nil, 0)
	if err != nil {
		t.Fatalf("AddComment failed: %v", err)
	}
	var added CommentMutationResult
	if err := json.Unmarshal([]byte(raw), &added); err != nil {
		t.Fatalf("unmarshal result failed: %v", err)
	}
	if added.FilePath != "a.go" || added.LineNumber != 7 {
		t.Errorf("expected a.go:7, got %s:%d", added.FilePath, added.LineNumber)
	}
	if added.FileStatus != "active" {
		t.Errorf("expected active status, got %q", added.FileStatus)
	}
	if len(added.Comments) != 1 {
		t.Fatalf("expected 1 comment in result, got %d", len(added.Comments))
	}
	commentID := added.Comments[0].ID

	// resolving anchors at the same root line and reports resolved.
	raw, err = app.ResolveComment("a.go", commentID)
	if err != nil {
		t.Fatalf("ResolveComment failed: %v", err)
	}
	var resolved CommentMutationResult
	json.Unmarshal([]byte(raw), &resolved)
	if resolved.LineNumber != 7 {
		t.Errorf("expected resolve anchored at line 7, got %d", resolved.LineNumber)
	}
	if resolved.FileStatus != "resolved" {
		t.Errorf("expected resolved status, got %q", resolved.FileStatus)
	}

	// a review-level comment reports an empty file path and line -1.
	raw, err = app.AddReviewComment("overall")
	if err != nil {
		t.Fatalf("AddReviewComment failed: %v", err)
	}
	var review CommentMutationResult
	json.Unmarshal([]byte(raw), &review)
	if review.FilePath != "" || review.LineNumber != -1 {
		t.Errorf("expected review-level result (\"\":-1), got %q:%d", review.FilePath, review.LineNumber)
	}

	// deleting reports the line of the now-removed thread and clears the file.
	raw, err = app.DeleteComment("a.go", commentID)
	if err != nil {
		t.Fatalf("DeleteComment failed: %v", err)
	}
	var deleted CommentMutationResult
	json.Unmarshal([]byte(raw), &deleted)
	if deleted.LineNumber != 7 {
		t.Errorf("expected delete anchored at line 7, got %d", deleted.LineNumber)
	}
	if deleted.FileStatus != "none" {
		t.Errorf("expected none status after delete, got %q", deleted.FileStatus)
	}
}

// SetFileMarked stores the file's blob SHA at mark-time, so the mark is durable.
func TestApp_SetFileMarked_StoresBlob(t *testing.T) {
	tmpDir := setupTestRepo(t)
	branch, err := GetCurrentBranch(tmpDir)
	if err != nil {
		t.Fatalf("failed to get branch: %v", err)
	}

	app := &App{
		review:    model.NewReview(tmpDir, branch, branch),
		repoPath:  tmpDir,
		dataDir:   t.TempDir(),
		statePath: filepath.Join(t.TempDir(), "state.json"),
	}

	if err := app.SetFileMarked("test.txt", true); err != nil {
		t.Fatalf("SetFileMarked failed: %v", err)
	}

	if len(app.review.MarkedFiles) != 1 {
		t.Fatalf("expected 1 mark, got %d", len(app.review.MarkedFiles))
	}
	if app.review.MarkedFiles[0].Blob == "" {
		t.Error("expected the stored mark to carry a non-empty blob SHA")
	}
}

// a committed change to a marked file evicts the mark on RecomputeDiff, and this
// holds even when the diff is recomputed fresh (the restart case).
func TestApp_RecomputeDiff_EvictsChangedMark(t *testing.T) {
	tmpDir := setupTestRepo(t)
	base, err := GetCurrentBranch(tmpDir)
	if err != nil {
		t.Fatalf("failed to get base branch: %v", err)
	}

	// branch and add a committed change so there is a diff to compute.
	checkout := exec.Command("git", "checkout", "-b", "feature")
	checkout.Dir = tmpDir
	checkout.Run()
	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("v2\n"), 0644)
	add := exec.Command("git", "add", "test.txt")
	add.Dir = tmpDir
	add.Run()
	commit := exec.Command("git", "commit", "-m", "v2")
	commit.Dir = tmpDir
	commit.Run()
	head, _ := GetCurrentBranch(tmpDir)

	app := &App{
		review:    model.NewReview(tmpDir, head, base),
		repoPath:  tmpDir,
		dataDir:   t.TempDir(),
		statePath: filepath.Join(t.TempDir(), "state.json"),
	}

	// mark the file at its current content.
	if err := app.SetFileMarked("test.txt", true); err != nil {
		t.Fatalf("SetFileMarked failed: %v", err)
	}

	// a recompute with no change keeps the mark.
	if err := app.RecomputeDiff(); err != nil {
		t.Fatalf("RecomputeDiff failed: %v", err)
	}
	if !app.review.IsFileMarked("test.txt") {
		t.Fatal("expected an unchanged marked file to stay marked")
	}

	// now commit a further change to the file, then recompute: the mark evicts.
	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("v3\n"), 0644)
	add2 := exec.Command("git", "add", "test.txt")
	add2.Dir = tmpDir
	add2.Run()
	commit2 := exec.Command("git", "commit", "-m", "v3")
	commit2.Dir = tmpDir
	commit2.Run()

	if err := app.RecomputeDiff(); err != nil {
		t.Fatalf("RecomputeDiff after change failed: %v", err)
	}
	if app.review.IsFileMarked("test.txt") {
		t.Error("expected a marked file changed by a new commit to be evicted")
	}
}

// commit `content` to `path` in the repo at `dir` with message `msg`. Used by the
// reconciliation integration tests to advance the source branch.
func commitFile(t *testing.T, dir, path, content, msg string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, path), []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	add := exec.Command("git", "add", path)
	add.Dir = dir
	if err := add.Run(); err != nil {
		t.Fatalf("git add %s: %v", path, err)
	}
	commit := exec.Command("git", "commit", "-m", msg)
	commit.Dir = dir
	if err := commit.Run(); err != nil {
		t.Fatalf("git commit: %v", err)
	}
}

// a comment placed on a line that later shifts down re-anchors to its new
// position when the diff is recomputed; a comment whose line is deleted becomes
// outdated. This drives the whole RecomputeDiff -> reanchorComments path through
// real git blobs and a real parsed diff, not the pure model in isolation.
func TestApp_RecomputeDiff_ReanchorsCommentOnShift(t *testing.T) {
	tmpDir := setupTestRepo(t)
	base, err := GetCurrentBranch(tmpDir)
	if err != nil {
		t.Fatalf("get base branch: %v", err)
	}

	// branch off and add a fresh file whose every line is added against base, so
	// the comment's context lines are present in the recomputed diff.
	checkout := exec.Command("git", "checkout", "-b", "feature")
	checkout.Dir = tmpDir
	if err := checkout.Run(); err != nil {
		t.Fatalf("create feature branch: %v", err)
	}
	commitFile(t, tmpDir, "app.go", "alpha\nbravo\ncharlie\ndelta\n", "add app.go")
	head, _ := GetCurrentBranch(tmpDir)

	app := &App{
		review:    model.NewReview(tmpDir, head, base),
		repoPath:  tmpDir,
		dataDir:   t.TempDir(),
		statePath: filepath.Join(t.TempDir(), "state.json"),
		userName:  "Test User",
	}
	if err := app.RecomputeDiff(); err != nil {
		t.Fatalf("initial RecomputeDiff: %v", err)
	}

	// comment on "charlie" (new-side line 3), capturing its context window. The
	// added lines run alpha(1) bravo(2) charlie(3) delta(4); the centre of the
	// 3-line window is charlie.
	if _, err := app.AddComment("app.go", "look at charlie", 3, []string{"bravo", "charlie", "delta"}, 1); err != nil {
		t.Fatalf("AddComment: %v", err)
	}

	// prepend two lines and commit: charlie shifts from line 3 to line 5.
	commitFile(t, tmpDir, "app.go", "header-1\nheader-2\nalpha\nbravo\ncharlie\ndelta\n", "prepend headers")

	if err := app.RecomputeDiff(); err != nil {
		t.Fatalf("RecomputeDiff after shift: %v", err)
	}

	comment := app.review.GetFileDiff("app.go").Comments[0]
	if comment.IsOutdated() {
		t.Fatal("expected the shifted comment to re-anchor, not go outdated")
	}
	if actual := comment.CurrentLineNumber(); actual != 5 {
		t.Errorf("expected the comment to re-anchor at line 5, got %d", actual)
	}
}

func TestApp_RecomputeDiff_OutdatesCommentOnDeletedLine(t *testing.T) {
	tmpDir := setupTestRepo(t)
	base, err := GetCurrentBranch(tmpDir)
	if err != nil {
		t.Fatalf("get base branch: %v", err)
	}

	checkout := exec.Command("git", "checkout", "-b", "feature")
	checkout.Dir = tmpDir
	if err := checkout.Run(); err != nil {
		t.Fatalf("create feature branch: %v", err)
	}
	commitFile(t, tmpDir, "app.go", "alpha\nbravo\ncharlie\ndelta\n", "add app.go")
	head, _ := GetCurrentBranch(tmpDir)

	app := &App{
		review:    model.NewReview(tmpDir, head, base),
		repoPath:  tmpDir,
		dataDir:   t.TempDir(),
		statePath: filepath.Join(t.TempDir(), "state.json"),
		userName:  "Test User",
	}
	if err := app.RecomputeDiff(); err != nil {
		t.Fatalf("initial RecomputeDiff: %v", err)
	}

	if _, err := app.AddComment("app.go", "look at charlie", 3, []string{"bravo", "charlie", "delta"}, 1); err != nil {
		t.Fatalf("AddComment: %v", err)
	}

	// rewrite the file so the commented context ("charlie") is gone entirely.
	commitFile(t, tmpDir, "app.go", "one\ntwo\nthree\nfour\nfive\n", "rewrite app.go")

	if err := app.RecomputeDiff(); err != nil {
		t.Fatalf("RecomputeDiff after delete: %v", err)
	}

	comment := app.review.GetFileDiff("app.go").Comments[0]
	if !comment.IsOutdated() {
		t.Error("expected a comment whose line was deleted to become outdated")
	}
}
