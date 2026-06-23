package main

import (
	"bytes"
	"code-review/model"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// a review with one active file comment (id "c-active"), one resolved file
// comment (id "c-done"), a review-level comment (id "c-top"), and a reply to the
// active comment (id "r-1"), plus one marked file. The active file comment
// carries a good anchor at line 7; a second review fixture exercises an adrift
// anchor where needed.
func fixtureReview() *model.Review {
	review := model.NewReview("/repo", "feature", "main")

	file := review.AddFileDiff("a.go")
	active := model.NewCommentWithContext("fix this", 7, "Reviewer", "blob-a", []string{"x", "y", "z"}, 1)
	active.ID = "c-active"
	done := model.NewCommentWithContext("done already", 12, "Reviewer", "blob-a", []string{"p", "q", "r"}, 1)
	done.ID = "c-done"
	done.Resolve()
	file.Comments = []*model.Comment{active, done}
	file.AddReply("c-active", "a reply", "Agent")
	file.Comments[len(file.Comments)-1].ID = "r-1"

	top := review.AddComment("overall note", "Reviewer")
	top.ID = "c-top"

	review.MarkFile("a.go", "blob-a")
	return review
}

// a reviewContext over `review` writing to a temp state file, for exercising the
// command handlers directly.
func fixtureContext(t *testing.T, review *model.Review) *reviewContext {
	t.Helper()
	return &reviewContext{
		review:    review,
		statePath: filepath.Join(t.TempDir(), "state.json"),
		userName:  "Agent",
	}
}

func TestFindCommentAcrossSurfaces(t *testing.T) {
	review := fixtureReview()

	tests := []struct {
		given        string
		expectedFile string
		expectFound  bool
	}{
		{"c-active", "a.go", true},
		{"c-done", "a.go", true},
		{"r-1", "a.go", true},
		{"c-top", "", true},
		{"missing", "", false},
	}

	for _, test := range tests {
		comment, file, _, found := findComment(review, test.given)
		if found != test.expectFound {
			t.Errorf("findComment(%q): found = %v, expected %v", test.given, found, test.expectFound)
			continue
		}
		if !found {
			continue
		}
		if file != test.expectedFile {
			t.Errorf("findComment(%q): file = %q, expected %q", test.given, file, test.expectedFile)
		}
		if comment.ID != test.given {
			t.Errorf("findComment(%q): returned comment id %q", test.given, comment.ID)
		}
	}
}

func TestListEmitsActiveRootsOnly(t *testing.T) {
	ctx := fixtureContext(t, fixtureReview())
	var out bytes.Buffer

	if err := cmdList(ctx, &out, nil); err != nil {
		t.Fatalf("cmdList: %v", err)
	}

	var actual []commentView
	if err := json.Unmarshal(out.Bytes(), &actual); err != nil {
		t.Fatalf("unmarshal list output: %v", err)
	}

	// only the active root comments: the file comment and the review-level one.
	// The resolved comment and the reply are excluded.
	expectedIDs := map[string]bool{"c-active": true, "c-top": true}
	if len(actual) != len(expectedIDs) {
		t.Fatalf("expected %d active roots, got %d: %+v", len(expectedIDs), len(actual), actual)
	}
	for _, view := range actual {
		if !expectedIDs[view.ID] {
			t.Errorf("unexpected comment in list: %q", view.ID)
		}
	}
}

func TestListEmptyIsEmptyArray(t *testing.T) {
	ctx := fixtureContext(t, model.NewReview("/repo", "feature", "main"))
	var out bytes.Buffer

	if err := cmdList(ctx, &out, nil); err != nil {
		t.Fatalf("cmdList: %v", err)
	}

	if got := strings.TrimSpace(out.String()); got != "[]" {
		t.Errorf("expected empty JSON array, got %q", got)
	}
}

func TestShowIncludesThreadAndOmitsInternalFields(t *testing.T) {
	ctx := fixtureContext(t, fixtureReview())
	var out bytes.Buffer

	if err := cmdShow(ctx, &out, []string{"c-active"}); err != nil {
		t.Fatalf("cmdShow: %v", err)
	}

	// assert on the raw JSON keys: internal anchor/blob fields must be absent.
	for _, forbidden := range []string{"anchors", "blob", "context"} {
		if strings.Contains(out.String(), `"`+forbidden+`"`) {
			t.Errorf("show output leaks internal field %q: %s", forbidden, out.String())
		}
	}

	var view commentView
	if err := json.Unmarshal(out.Bytes(), &view); err != nil {
		t.Fatalf("unmarshal show output: %v", err)
	}
	if view.Line != 7 {
		t.Errorf("expected line 7, got %d", view.Line)
	}
	if view.Outdated {
		t.Error("expected a good anchor to report outdated = false")
	}
	if len(view.Replies) != 1 || view.Replies[0].ID != "r-1" {
		t.Errorf("expected one reply r-1, got %+v", view.Replies)
	}
}

func TestShowOutdatedFlag(t *testing.T) {
	review := fixtureReview()
	// drive the active comment adrift: append an anchor with no context against a
	// new blob, which is how `reanchorComment` records a failed re-anchor.
	comment, _, _, _ := findComment(review, "c-active")
	comment.Anchors = append(comment.Anchors, model.Anchor{Blob: "blob-b"})

	ctx := fixtureContext(t, review)
	var out bytes.Buffer
	if err := cmdShow(ctx, &out, []string{"c-active"}); err != nil {
		t.Fatalf("cmdShow: %v", err)
	}

	var view commentView
	if err := json.Unmarshal(out.Bytes(), &view); err != nil {
		t.Fatalf("unmarshal show output: %v", err)
	}
	if !view.Outdated {
		t.Error("expected an adrift anchor to report outdated = true")
	}
}

func TestShowUnknownID(t *testing.T) {
	ctx := fixtureContext(t, fixtureReview())
	if err := cmdShow(ctx, &bytes.Buffer{}, []string{"missing"}); err == nil {
		t.Error("expected an error for an unknown comment id")
	}
}

func TestStatusCounts(t *testing.T) {
	ctx := fixtureContext(t, fixtureReview())
	var out bytes.Buffer

	if err := cmdStatus(ctx, &out, nil); err != nil {
		t.Fatalf("cmdStatus: %v", err)
	}

	var view statusView
	if err := json.Unmarshal(out.Bytes(), &view); err != nil {
		t.Fatalf("unmarshal status output: %v", err)
	}

	// active: c-active + c-top; resolved: c-done; ignored: none; marks: a.go.
	expected := statusView{SourceBranch: "feature", TargetBranch: "main", Active: 2, Resolved: 1, Ignored: 0, MarkedFiles: 1}
	if view != expected {
		t.Errorf("status = %+v, expected %+v", view, expected)
	}
}

func TestResolveAndReactivate(t *testing.T) {
	ctx := fixtureContext(t, fixtureReview())

	if err := cmdResolve(ctx, &bytes.Buffer{}, []string{"c-active"}); err != nil {
		t.Fatalf("cmdResolve: %v", err)
	}
	comment, _, _, _ := findComment(ctx.review, "c-active")
	if comment.Status != model.CommentStatusResolved {
		t.Errorf("expected resolved, got %q", comment.Status)
	}

	if err := cmdReactivate(ctx, &bytes.Buffer{}, []string{"c-active"}); err != nil {
		t.Fatalf("cmdReactivate: %v", err)
	}
	if comment.Status != model.CommentStatusActive {
		t.Errorf("expected active after reactivate, got %q", comment.Status)
	}

	// the change is persisted, not just in memory.
	loaded, err := LoadReview(ctx.statePath)
	if err != nil {
		t.Fatalf("LoadReview: %v", err)
	}
	if reloaded := loaded.GetFileDiff("a.go").GetComment("c-active"); reloaded.Status != model.CommentStatusActive {
		t.Errorf("persisted status = %q, expected active", reloaded.Status)
	}
}

func TestResolveRejectsReplyAndUnknown(t *testing.T) {
	for _, given := range []string{"r-1", "missing"} {
		ctx := fixtureContext(t, fixtureReview())
		if err := cmdResolve(ctx, &bytes.Buffer{}, []string{given}); err == nil {
			t.Errorf("cmdResolve(%q): expected an error", given)
		}
		// no write: the state file must not have been created.
		if _, err := os.Stat(ctx.statePath); !os.IsNotExist(err) {
			t.Errorf("cmdResolve(%q): state file was written despite the error", given)
		}
	}
}

func TestReplyAppendsToThread(t *testing.T) {
	ctx := fixtureContext(t, fixtureReview())

	if err := cmdReply(ctx, &bytes.Buffer{}, []string{"c-active", "another reply"}); err != nil {
		t.Fatalf("cmdReply: %v", err)
	}

	replies := threadReplies(ctx.review.GetFileDiff("a.go").Comments, "c-active")
	if len(replies) != 2 {
		t.Fatalf("expected 2 replies after appending, got %d", len(replies))
	}
	last := replies[len(replies)-1]
	if last.Content != "another reply" || last.Author != "Agent" {
		t.Errorf("appended reply = %+v, expected content 'another reply' by Agent", last)
	}

	// the parent's status is unchanged by a reply.
	parent, _, _, _ := findComment(ctx.review, "c-active")
	if parent.Status != model.CommentStatusActive {
		t.Errorf("reply changed parent status to %q", parent.Status)
	}
}

func TestReplyToReviewLevelComment(t *testing.T) {
	ctx := fixtureContext(t, fixtureReview())
	if err := cmdReply(ctx, &bytes.Buffer{}, []string{"c-top", "noted"}); err != nil {
		t.Fatalf("cmdReply on review-level comment: %v", err)
	}
	if replies := threadReplies(ctx.review.Comments, "c-top"); len(replies) != 1 {
		t.Errorf("expected one reply on the review-level comment, got %d", len(replies))
	}
}

func TestReplyUnknownID(t *testing.T) {
	ctx := fixtureContext(t, fixtureReview())
	if err := cmdReply(ctx, &bytes.Buffer{}, []string{"missing", "text"}); err == nil {
		t.Error("expected an error replying to an unknown id")
	}
	if _, err := os.Stat(ctx.statePath); !os.IsNotExist(err) {
		t.Error("state file was written despite the error")
	}
}

func TestCommentAddsReviewLevel(t *testing.T) {
	ctx := fixtureContext(t, fixtureReview())
	before := len(ctx.review.Comments)

	if err := cmdComment(ctx, &bytes.Buffer{}, []string{"a new top-level note"}); err != nil {
		t.Fatalf("cmdComment: %v", err)
	}

	if len(ctx.review.Comments) != before+1 {
		t.Fatalf("expected one new review-level comment, got %d", len(ctx.review.Comments))
	}
	added := ctx.review.Comments[len(ctx.review.Comments)-1]
	if added.Content != "a new top-level note" || added.Author != "Agent" || added.ParentID != "" {
		t.Errorf("added comment = %+v, expected an unattached note by Agent", added)
	}
}

func TestCommentRejectsEmptyText(t *testing.T) {
	ctx := fixtureContext(t, fixtureReview())
	if err := cmdComment(ctx, &bytes.Buffer{}, []string{"   "}); err == nil {
		t.Error("expected an error for empty comment text")
	}
	if _, err := os.Stat(ctx.statePath); !os.IsNotExist(err) {
		t.Error("state file was written despite the error")
	}
}

func TestUnmark(t *testing.T) {
	ctx := fixtureContext(t, fixtureReview())

	if err := cmdUnmark(ctx, &bytes.Buffer{}, []string{"a.go"}); err != nil {
		t.Fatalf("cmdUnmark: %v", err)
	}
	if ctx.review.IsFileMarked("a.go") {
		t.Error("a.go is still marked after unmark")
	}
}

func TestUnmarkUnmarkedFileIsNoOp(t *testing.T) {
	ctx := fixtureContext(t, fixtureReview())
	before := len(ctx.review.MarkedFiles)

	if err := cmdUnmark(ctx, &bytes.Buffer{}, []string{"never-marked.go"}); err != nil {
		t.Fatalf("cmdUnmark on an unmarked file: %v", err)
	}
	if len(ctx.review.MarkedFiles) != before {
		t.Errorf("marked set changed: was %d, now %d", before, len(ctx.review.MarkedFiles))
	}
}

func TestInstructionsPrintsContract(t *testing.T) {
	var out bytes.Buffer
	if err := cmdInstructions(nil, &out, nil); err != nil {
		t.Fatalf("cmdInstructions: %v", err)
	}
	if !strings.Contains(out.String(), "code-review list") {
		t.Errorf("instructions output does not mention the CLI commands: %q", out.String())
	}
}

func TestUnknownCommandExitsTwo(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := runCLI([]string{"frobnicate"}, &out, &errOut); code != 2 {
		t.Errorf("unknown command exit code = %d, expected 2", code)
	}
	if !strings.Contains(errOut.String(), "unknown command") {
		t.Errorf("expected an 'unknown command' message, got %q", errOut.String())
	}
}

func TestHelpListsCommands(t *testing.T) {
	for _, given := range []string{"-h", "--help"} {
		var out bytes.Buffer
		if code := runCLI([]string{given}, &out, &bytes.Buffer{}); code != 0 {
			t.Errorf("%s exit code = %d, expected 0", given, code)
		}
		for _, name := range []string{"list", "resolve", "instructions"} {
			if !strings.Contains(out.String(), name) {
				t.Errorf("%s usage omits command %q", given, name)
			}
		}
	}
}

func TestIsCLIInvocation(t *testing.T) {
	tests := []struct {
		given    []string
		expected bool
	}{
		{nil, false},
		{[]string{"--version"}, false},
		{[]string{"list"}, true},
		{[]string{"-h"}, true},
		{[]string{"--help"}, true},
		{[]string{"frobnicate"}, true}, // recognised as a CLI attempt, errors later
	}
	for _, test := range tests {
		if got := isCLIInvocation(test.given); got != test.expected {
			t.Errorf("isCLIInvocation(%v) = %v, expected %v", test.given, got, test.expected)
		}
	}
}

// initGitRepo creates a throwaway git repository with one commit on `main` and a
// checked-out `feature` branch, returning its path. Used for the resolution and
// read-only tests, which need real git plumbing.
func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "HOME="+dir)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.name", "Test User")
	run("config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	run("add", ".")
	run("commit", "-q", "-m", "initial")
	run("checkout", "-q", "-b", "feature")
	return dir
}

// run the CLI with the working directory set to `dir`, restoring it afterwards.
// The CLI resolves the review from the current directory, so the test must enter
// the repo. `t.Chdir` (Go 1.24+) restores the directory at cleanup.
func runCLIInDir(t *testing.T, dir string, args ...string) (int, string, string) {
	t.Helper()
	t.Chdir(dir)
	t.Setenv("HOME", dir)
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "xdg"))
	var out, errOut bytes.Buffer
	code := runCLI(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestResolveReviewErrors(t *testing.T) {
	// not a git repository: a bare temp dir.
	t.Run("not a repo", func(t *testing.T) {
		code, _, errOut := runCLIInDir(t, t.TempDir(), "list")
		if code != 1 {
			t.Errorf("exit code = %d, expected 1", code)
		}
		if !strings.Contains(errOut, "not a git repository") {
			t.Errorf("expected a 'not a git repository' message, got %q", errOut)
		}
	})

	// a repo on a branch with no review state yet.
	t.Run("no review", func(t *testing.T) {
		code, _, errOut := runCLIInDir(t, initGitRepo(t), "list")
		if code != 1 {
			t.Errorf("exit code = %d, expected 1", code)
		}
		if !strings.Contains(errOut, "no review found") {
			t.Errorf("expected a 'no review found' message, got %q", errOut)
		}
	})
}

// a mutating command must write only the state file and leave the repository
// untouched (no branch switch, no working-tree or index change).
func TestMutationLeavesRepositoryUnchanged(t *testing.T) {
	dir := initGitRepo(t)

	// seed a review at the path the CLI will resolve, with one active comment.
	branch, _ := GetCurrentBranch(dir)
	def, _ := GetDefaultBranch(dir)
	xdg := filepath.Join(dir, "xdg", "code-review")
	statePath := GetReviewStatePath(xdg, dir, branch, def)
	if err := os.MkdirAll(xdg, 0o755); err != nil {
		t.Fatalf("mkdir xdg: %v", err)
	}
	review := model.NewReview(dir, branch, def)
	file := review.AddFileDiff("a.go")
	seed := model.NewCommentWithContext("note", 1, "Reviewer", "blob", []string{"package a"}, 0)
	seed.ID = "c-1"
	file.Comments = []*model.Comment{seed}
	if err := SaveReview(statePath, review); err != nil {
		t.Fatalf("seed review: %v", err)
	}

	statusBefore := gitStatus(t, dir)
	branchBefore := gitHead(t, dir)

	code, _, errOut := runCLIInDir(t, dir, "resolve", "c-1")
	if code != 0 {
		t.Fatalf("resolve failed: code %d, %s", code, errOut)
	}

	if after := gitStatus(t, dir); after != statusBefore {
		t.Errorf("working tree changed: before %q, after %q", statusBefore, after)
	}
	if after := gitHead(t, dir); after != branchBefore {
		t.Errorf("branch changed: before %q, after %q", branchBefore, after)
	}
}

func gitStatus(t *testing.T, dir string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "status", "--porcelain").CombinedOutput()
	if err != nil {
		t.Fatalf("git status: %v\n%s", err, out)
	}
	return string(out)
}

func gitHead(t *testing.T, dir string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD").CombinedOutput()
	if err != nil {
		t.Fatalf("git rev-parse: %v\n%s", err, out)
	}
	return string(out)
}
