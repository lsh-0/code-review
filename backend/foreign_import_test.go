package main

import (
	"code-review/model"
	"strings"
	"testing"
)

// the paths the fixture comments on, with the line each of its threads states.
var fixturePaths = map[string]int{
	".claude/commands/opsx/apply.md":                            1,
	"config/server.conf":                                        69,
	"test/integration/www.example.com/featureFlagEvaluation.ts": 71,
	"test/integration/utilities/pathCaseTests.ts":               22,
	"config/conf.d/www.example.com.conf":                        419,
}

// diff lines for `path` covering `first` through `last`, each line's content naming
// its number so a captured window is identifiable in an assertion.
func diffLinesFor(first int, last int) []model.DiffLineRef {
	lines := make([]model.DiffLineRef, 0, last-first+1)
	for n := first; n <= last; n++ {
		lines = append(lines, model.DiffLineRef{Content: "line " + itoaTest(n), LineNumber: n})
	}
	return lines
}

func itoaTest(n int) string {
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	if digits == "" {
		return "0"
	}
	return digits
}

// {1 Mapping}

func TestMapFixtureProducesEveryThread(t *testing.T) {
	doc := decodeFixture(t)

	mapped, report := mapForeignDocument(doc, map[string][]model.DiffLineRef{}, map[string]string{})

	expected := 7
	actual := len(mapped)
	if actual != expected {
		t.Fatalf("mapped comments: got %d, want %d", actual, expected)
	}
	if report.Imported != expected {
		t.Errorf("report imported: got %d, want %d", report.Imported, expected)
	}
	if report.Skipped != 0 {
		t.Errorf("report skipped: got %d, want 0", report.Skipped)
	}
	if report.Replies != 0 {
		t.Errorf("report replies: got %d, want 0", report.Replies)
	}
}

func TestMapFixtureEveryCommentCarriesItsOrigin(t *testing.T) {
	doc := decodeFixture(t)

	mapped, _ := mapForeignDocument(doc, map[string][]model.DiffLineRef{}, map[string]string{})

	for _, entry := range mapped {
		if !entry.Comment.IsForeign() {
			t.Errorf("%s: expected an imported comment to be foreign", entry.FilePath)
		}
		if entry.Comment.SourceID() != "bb:1430" {
			t.Errorf("source id: got %q, want %q", entry.Comment.SourceID(), "bb:1430")
		}
		if entry.Comment.Origin.ForeignID == "" {
			t.Error("expected an imported comment to carry its foreign id")
		}
	}
}

func TestMapFixtureReviewLevelCommentHasNoAnchor(t *testing.T) {
	doc := decodeFixture(t)

	mapped, _ := mapForeignDocument(doc, map[string][]model.DiffLineRef{}, map[string]string{})

	reviewLevel := make([]mappedComment, 0)
	for _, entry := range mapped {
		if entry.FilePath == "" {
			reviewLevel = append(reviewLevel, entry)
		}
	}

	if len(reviewLevel) != 1 {
		t.Fatalf("review-level comments: got %d, want 1", len(reviewLevel))
	}
	if len(reviewLevel[0].Comment.Anchors) != 0 {
		t.Errorf("expected a review-level comment to carry no anchor, got %d", len(reviewLevel[0].Comment.Anchors))
	}
	if reviewLevel[0].Comment.IsOutdated() {
		t.Error("expected a review-level comment never to be outdated")
	}
}

// a line present in the diff yields a good anchor at that line, with a captured
// window, so the comment renders against its line and reconciles like a local one.
func TestMapPlacesCommentAtStatedLineWithCapturedWindow(t *testing.T) {
	doc := decodeFixture(t)
	lines := map[string][]model.DiffLineRef{"config/server.conf": diffLinesFor(60, 80)}
	blobs := map[string]string{"config/server.conf": "sha-server"}

	mapped, report := mapForeignDocument(doc, lines, blobs)

	var placed *model.Comment
	for _, entry := range mapped {
		if entry.FilePath == "config/server.conf" {
			placed = entry.Comment
			break
		}
	}
	if placed == nil {
		t.Fatal("expected a comment on config/server.conf")
	}

	if placed.IsOutdated() {
		t.Error("expected a comment on a present line not to be outdated")
	}
	if placed.CurrentLineNumber() != 69 {
		t.Errorf("line: got %d, want 69", placed.CurrentLineNumber())
	}

	anchor := placed.Anchors[0]
	if anchor.Blob != "sha-server" {
		t.Errorf("blob: got %q, want %q", anchor.Blob, "sha-server")
	}
	if len(anchor.Context) != 7 {
		t.Errorf("captured window: got %d lines, want 7", len(anchor.Context))
	}
	if anchor.Context[anchor.Offset] != "line 69" {
		t.Errorf("offset %d points at %q, want %q", anchor.Offset, anchor.Context[anchor.Offset], "line 69")
	}

	if report.Placed != 1 {
		t.Errorf("report placed: got %d, want 1", report.Placed)
	}
}

// a line absent from the diff yields an adrift anchor, so the comment renders
// untethered rather than being placed on code that is not there.
func TestMapLeavesAbsentLineUntethered(t *testing.T) {
	doc := decodeFixture(t)
	// the file is in the diff, but line 69 is not within the supplied range.
	lines := map[string][]model.DiffLineRef{"config/server.conf": diffLinesFor(1, 10)}
	blobs := map[string]string{"config/server.conf": "sha-server"}

	mapped, report := mapForeignDocument(doc, lines, blobs)

	for _, entry := range mapped {
		if entry.FilePath != "config/server.conf" {
			continue
		}
		if !entry.Comment.IsOutdated() {
			t.Error("expected a comment on an absent line to be outdated")
		}
		if entry.Comment.Anchors[0].Blob != "sha-server" {
			t.Errorf("expected the adrift anchor to carry the current blob, got %q", entry.Comment.Anchors[0].Blob)
		}
		if len(entry.Comment.Anchors[0].Context) != 0 {
			t.Error("expected an adrift anchor to carry no context")
		}
	}

	if report.Untethered != 6 {
		t.Errorf("report untethered: got %d, want 6", report.Untethered)
	}
	if report.Placed != 0 {
		t.Errorf("report placed: got %d, want 0", report.Placed)
	}
}

func TestMapMultiLineSpanPlacesOneCommentAtFirstLine(t *testing.T) {
	doc, err := decodeForeignDocument([]byte(documentWith(fileThread("a.go", positionUpdated, 12, 20))))
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	lines := map[string][]model.DiffLineRef{"a.go": diffLinesFor(1, 30)}

	mapped, _ := mapForeignDocument(doc, lines, map[string]string{"a.go": "sha"})

	if len(mapped) != 1 {
		t.Fatalf("expected one comment for a multi-line span, got %d", len(mapped))
	}
	if mapped[0].Comment.CurrentLineNumber() != 12 {
		t.Errorf("line: got %d, want 12", mapped[0].Comment.CurrentLineNumber())
	}
}

func TestMapUnchangedPositionUsesUpdatedSpan(t *testing.T) {
	given := fileThreadRaw("a.go",
		`{"kind": "unchanged", "original_lines": {"first": 4, "last": 4}, "updated_lines": {"first": 7, "last": 7}}`)
	doc, err := decodeForeignDocument([]byte(documentWith(given)))
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	lines := map[string][]model.DiffLineRef{"a.go": diffLinesFor(1, 20)}

	mapped, _ := mapForeignDocument(doc, lines, map[string]string{"a.go": "sha"})

	if mapped[0].Comment.CurrentLineNumber() != 7 {
		t.Errorf("expected the updated span's line, got %d", mapped[0].Comment.CurrentLineNumber())
	}
	if mapped[0].Comment.IsOutdated() {
		t.Error("expected an unchanged position on a present line to be placed")
	}
}

// an old-side or whole-file position addresses no new-side line, so it can never be
// placed. It imports untethered and is counted apart from a line that merely drifted.
func TestMapUnplaceablePositionsImportUntethered(t *testing.T) {
	given := []struct {
		name     string
		position string
	}{
		{"original", `{"kind": "original", "lines": {"first": 4, "last": 4}}`},
		{"whole file", `{"kind": "whole_file"}`},
	}

	for _, test := range given {
		t.Run(test.name, func(t *testing.T) {
			doc, err := decodeForeignDocument([]byte(documentWith(fileThreadRaw("a.go", test.position))))
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}
			lines := map[string][]model.DiffLineRef{"a.go": diffLinesFor(1, 20)}

			mapped, report := mapForeignDocument(doc, lines, map[string]string{"a.go": "sha"})

			if !mapped[0].Comment.IsOutdated() {
				t.Error("expected an unplaceable position to import untethered")
			}
			if report.Untethered != 1 {
				t.Errorf("report untethered: got %d, want 1", report.Untethered)
			}
			if len(report.UnplaceablePaths) != 1 {
				t.Fatalf("report unplaceable paths: got %d, want 1", len(report.UnplaceablePaths))
			}
			if report.UnplaceablePaths[0] != "a.go" {
				t.Errorf("unplaceable path: got %q, want %q", report.UnplaceablePaths[0], "a.go")
			}
			// the reviewer must be told which file to look in, not only how many.
			joined := strings.Join(report.Lines(), "\n")
			if !strings.Contains(joined, "a.go") {
				t.Errorf("report does not name the unplaceable file: %s", joined)
			}
		})
	}
}

func TestMapSkipsDeletedAndPendingComments(t *testing.T) {
	given := []struct {
		name string
		body string
		pend bool
	}{
		{"deleted", `{"kind": "deleted"}`, false},
		{"pending", markdownBody("a draft"), true},
	}

	for _, test := range given {
		t.Run(test.name, func(t *testing.T) {
			thread := threadJSON("a.go", `{"kind": "updated", "lines": {"first": 1, "last": 1}}`,
				commentJSON(1, test.body, test.pend, nil))
			doc, err := decodeForeignDocument([]byte(documentWith(thread)))
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}

			mapped, report := mapForeignDocument(doc, map[string][]model.DiffLineRef{}, map[string]string{})

			if len(mapped) != 0 {
				t.Errorf("expected no comment to be imported, got %d", len(mapped))
			}
			if report.Skipped != 1 {
				t.Errorf("report skipped: got %d, want 1", report.Skipped)
			}
		})
	}
}

// skipping a root skips its replies with it, and the count reflects every comment
// the thread cost rather than one.
func TestMapSkippingARootSkipsItsThread(t *testing.T) {
	reply := commentJSON(2, markdownBody("a reply"), false, nil)
	root := commentJSON(1, `{"kind": "deleted"}`, false, []string{reply})
	doc, err := decodeForeignDocument([]byte(documentWith(
		threadJSON("a.go", `{"kind": "updated", "lines": {"first": 1, "last": 1}}`, root))))
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	mapped, report := mapForeignDocument(doc, map[string][]model.DiffLineRef{}, map[string]string{})

	if len(mapped) != 0 {
		t.Errorf("expected the whole thread to be skipped, got %d comments", len(mapped))
	}
	if report.Skipped != 2 {
		t.Errorf("report skipped: got %d, want 2", report.Skipped)
	}
}

// a nested exchange flattens by re-rooting: every comment survives, in order, with
// its own author, and nothing is parented to a reply.
func TestMapFlattensNestedRepliesByReRooting(t *testing.T) {
	deepest := `{"id": 4, "author": {"display_name": "Dana", "kind": "human"},
		"body": ` + markdownBody("deepest") + `, "created_at": "2026-09-04T03:59:34.961609Z",
		"pending": false, "replies": []}`
	middle := `{"id": 3, "author": {"display_name": "Cass", "kind": "human"},
		"body": ` + markdownBody("middle") + `, "created_at": "2026-09-04T03:59:34.961609Z",
		"pending": false, "replies": [` + deepest + `]}`
	inner := `{"id": 2, "author": {"display_name": "Bo", "kind": "human"},
		"body": ` + markdownBody("inner") + `, "created_at": "2026-09-04T03:59:34.961609Z",
		"pending": false, "replies": [` + middle + `]}`
	root := `{"id": 1, "author": {"display_name": "Ari", "kind": "human"},
		"body": ` + markdownBody("root") + `, "created_at": "2026-09-04T03:59:34.961609Z",
		"pending": false, "replies": [` + inner + `]}`

	doc, err := decodeForeignDocument([]byte(documentWith(
		threadJSON("a.go", `{"kind": "updated", "lines": {"first": 1, "last": 1}}`, root))))
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	lines := map[string][]model.DiffLineRef{"a.go": diffLinesFor(1, 10)}

	mapped, report := mapForeignDocument(doc, lines, map[string]string{"a.go": "sha"})

	if len(mapped) != 4 {
		t.Fatalf("expected every comment to survive, got %d", len(mapped))
	}
	if report.Replies != 3 {
		t.Errorf("report replies: got %d, want 3", report.Replies)
	}

	rootID := mapped[0].Comment.ID
	if mapped[0].Comment.ParentID != "" {
		t.Error("expected the first mapped comment to be the root")
	}

	expectedOrder := []string{"root", "inner", "middle", "deepest"}
	expectedAuthors := []string{"Ari", "Bo", "Cass", "Dana"}
	for i, entry := range mapped {
		if entry.Comment.Content != expectedOrder[i] {
			t.Errorf("comment %d: content %q, want %q", i, entry.Comment.Content, expectedOrder[i])
		}
		if entry.Comment.Author != expectedAuthors[i] {
			t.Errorf("comment %d: author %q, want %q", i, entry.Comment.Author, expectedAuthors[i])
		}
		if i > 0 && entry.Comment.ParentID != rootID {
			t.Errorf("comment %d: expected re-rooting to the root, got parent %q", i, entry.Comment.ParentID)
		}
	}
}

// no imported comment may be parented to another reply: the model's threads are one
// level deep and the render layer shows exactly that.
func TestMapNeverParentsAReplyToAReply(t *testing.T) {
	inner := commentJSON(3, markdownBody("inner"), false, nil)
	outer := commentJSON(2, markdownBody("outer"), false, []string{inner})
	root := commentJSON(1, markdownBody("root"), false, []string{outer})

	doc, err := decodeForeignDocument([]byte(documentWith(
		threadJSON("a.go", `{"kind": "updated", "lines": {"first": 1, "last": 1}}`, root))))
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	mapped, _ := mapForeignDocument(doc, map[string][]model.DiffLineRef{}, map[string]string{})

	replyIDs := make(map[string]bool)
	for _, entry := range mapped {
		if entry.Comment.ParentID != "" {
			replyIDs[entry.Comment.ID] = true
		}
	}
	for _, entry := range mapped {
		if entry.Comment.ParentID != "" && replyIDs[entry.Comment.ParentID] {
			t.Errorf("comment %q is parented to a reply", entry.Comment.Content)
		}
	}
}

func TestMapMarksAutomatedAuthors(t *testing.T) {
	doc := decodeFixture(t)

	mapped, _ := mapForeignDocument(doc, map[string][]model.DiffLineRef{}, map[string]string{})

	if !strings.Contains(mapped[0].Comment.Author, "review-bot") {
		t.Errorf("author: got %q, want it to name review-bot", mapped[0].Comment.Author)
	}
	if !strings.Contains(mapped[0].Comment.Author, "(bot)") {
		t.Errorf("author %q does not mark the automated reviewer", mapped[0].Comment.Author)
	}
}

// a thread already resolved at the source starts resolved locally, so the reviewer
// is not asked to work through feedback its author has closed.
func TestMapResolvedThreadStartsResolved(t *testing.T) {
	given := `{
		"pull_request": ` + minimalPullRequest + `,
		"discussion": [{
			"location": {"kind": "pull_request"},
			"resolution": {"state": "resolved", "by": ` + humanAuthor + `, "at": "2026-09-04T03:59:34.961609Z"},
			"root": ` + commentJSON(1, markdownBody("overall"), false, nil) + `
		}],
		"file_feedback": []
	}`
	doc, err := decodeForeignDocument([]byte(given))
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	mapped, _ := mapForeignDocument(doc, map[string][]model.DiffLineRef{}, map[string]string{})

	if mapped[0].Comment.Status != model.CommentStatusResolved {
		t.Errorf("status: got %q, want %q", mapped[0].Comment.Status, model.CommentStatusResolved)
	}
}

func TestMapReportsUnmatchedPaths(t *testing.T) {
	doc := decodeFixture(t)
	lines := map[string][]model.DiffLineRef{"config/server.conf": diffLinesFor(60, 80)}

	_, report := mapForeignDocument(doc, lines, map[string]string{"config/server.conf": "sha"})

	if report.MatchedPaths != 1 {
		t.Errorf("matched paths: got %d, want 1", report.MatchedPaths)
	}
	if len(report.UnmatchedPaths) != 4 {
		t.Errorf("unmatched paths: got %d, want 4: %v", len(report.UnmatchedPaths), report.UnmatchedPaths)
	}
	// named, not merely counted: the reviewer needs the path to act on it.
	expectedUnmatched := []string{
		".claude/commands/opsx/apply.md",
		"config/conf.d/www.example.com.conf",
		"test/integration/utilities/pathCaseTests.ts",
		"test/integration/www.example.com/flagEvaluation.test.ts",
	}
	if !equalStringSlices(report.UnmatchedPaths, expectedUnmatched) {
		t.Errorf("unmatched paths: got %v, want %v", report.UnmatchedPaths, expectedUnmatched)
	}
	joined := strings.Join(report.Lines(), "\n")
	for _, path := range expectedUnmatched {
		if !strings.Contains(joined, path) {
			t.Errorf("report does not name %q: %s", path, joined)
		}
	}
	if report.whollyUnmatched() {
		t.Error("expected a partially matched document not to report as wholly unmatched")
	}
}

func TestMapReportsWhollyUnmatchedDocument(t *testing.T) {
	doc := decodeFixture(t)

	_, report := mapForeignDocument(doc, map[string][]model.DiffLineRef{}, map[string]string{})

	if !report.whollyUnmatched() {
		t.Error("expected a document with no matching path to report as wholly unmatched")
	}

	joined := strings.Join(report.Lines(), "\n")
	if !strings.Contains(joined, "another branch") {
		t.Errorf("report does not call out the wholly unmatched case: %s", joined)
	}
}

func TestForeignSourceIsDerivedFromTheDocument(t *testing.T) {
	doc := decodeFixture(t)

	actual := foreignSource(doc, "2026-09-14T12:00:00Z")

	if actual.ID != "bb:1430" {
		t.Errorf("id: got %q, want %q", actual.ID, "bb:1430")
	}
	if actual.PullRequest != 1430 {
		t.Errorf("pull request: got %d, want 1430", actual.PullRequest)
	}
	if actual.SourceBranch != "feature-1430--telemetry" {
		t.Errorf("source branch: got %q", actual.SourceBranch)
	}
	if actual.TargetBranch != "master" {
		t.Errorf("target branch: got %q", actual.TargetBranch)
	}
	if !strings.Contains(actual.Label, "1430") {
		t.Errorf("label %q does not name the pull request", actual.Label)
	}
	if actual.Title != "Telemetry flag wiring" {
		t.Errorf("title: got %q, want %q", actual.Title, "Telemetry flag wiring")
	}
	if actual.ImportedAt != "2026-09-14T12:00:00Z" {
		t.Errorf("imported at: got %q", actual.ImportedAt)
	}
}

// {1 Merging}

// import the fixture into a fresh review, as the CLI does.
func importFixture(t *testing.T, review *model.Review, lines map[string][]model.DiffLineRef) importReport {
	t.Helper()
	doc := decodeFixture(t)
	blobs := make(map[string]string, len(lines))
	for path := range lines {
		blobs[path] = "sha-" + path
	}
	mapped, report := mapForeignDocument(doc, lines, blobs)
	mergeForeignComments(review, foreignSource(doc, "t1"), mapped)
	return report
}

func TestMergeRegistersTheSource(t *testing.T) {
	given := model.NewReview("/repo", "feature", "master")

	importFixture(t, given, map[string][]model.DiffLineRef{})

	if len(given.Sources) != 1 {
		t.Fatalf("sources: got %d, want 1", len(given.Sources))
	}
	if given.GetSource("bb:1430") == nil {
		t.Error("expected the source to be registered under its id")
	}
}

func TestMergeIsIdempotent(t *testing.T) {
	given := model.NewReview("/repo", "feature", "master")
	lines := map[string][]model.DiffLineRef{"config/server.conf": diffLinesFor(60, 80)}

	importFixture(t, given, lines)
	first := len(given.GetAllComments())
	importFixture(t, given, lines)
	second := len(given.GetAllComments())

	if first != second {
		t.Errorf("comment count changed on re-import: %d then %d", first, second)
	}
	if len(given.Sources) != 1 {
		t.Errorf("sources: got %d, want 1", len(given.Sources))
	}
}

func TestMergePreservesLocalReplyAcrossReimport(t *testing.T) {
	given := model.NewReview("/repo", "feature", "master")
	lines := map[string][]model.DiffLineRef{"config/server.conf": diffLinesFor(60, 80)}
	importFixture(t, given, lines)

	file := given.GetFileDiff("config/server.conf")
	root := file.Comments[0]
	file.AddReply(root.ID, "I will fix this", "Test User")

	importFixture(t, given, lines)

	file = given.GetFileDiff("config/server.conf")
	replies := 0
	rootStillPresent := false
	for _, comment := range file.Comments {
		if comment.ID == root.ID {
			rootStillPresent = true
		}
		if comment.ParentID == root.ID && !comment.IsForeign() {
			replies++
		}
	}

	if !rootStillPresent {
		t.Error("expected the re-imported root to keep its local id")
	}
	if replies != 1 {
		t.Errorf("local replies after re-import: got %d, want 1", replies)
	}
}

func TestMergePreservesLocalResolutionAcrossReimport(t *testing.T) {
	given := model.NewReview("/repo", "feature", "master")
	lines := map[string][]model.DiffLineRef{"config/server.conf": diffLinesFor(60, 80)}
	importFixture(t, given, lines)

	file := given.GetFileDiff("config/server.conf")
	file.Comments[0].Resolve()
	resolvedID := file.Comments[0].ID

	importFixture(t, given, lines)

	file = given.GetFileDiff("config/server.conf")
	for _, comment := range file.Comments {
		if comment.ID == resolvedID {
			if comment.Status != model.CommentStatusResolved {
				t.Errorf("status after re-import: got %q, want resolved", comment.Status)
			}
			return
		}
	}
	t.Error("expected the resolved comment to survive the re-import")
}

// a comment gone from the source is removed locally, and its local replies go with
// it, because the thread no longer exists to answer.
func TestMergeRemovesCommentsAbsentFromTheDocument(t *testing.T) {
	given := model.NewReview("/repo", "feature", "master")
	doc := decodeFixture(t)
	lines := map[string][]model.DiffLineRef{"a.go": diffLinesFor(1, 10)}

	mapped, _ := mapForeignDocument(doc, lines, map[string]string{})
	source := foreignSource(doc, "t1")
	mergeForeignComments(given, source, mapped)

	file := given.GetFileDiff("config/server.conf")
	if file == nil {
		t.Fatal("expected the imported file to be present")
	}
	root := file.Comments[0]
	file.AddReply(root.ID, "local reply", "Test User")

	// a second import of the same source carrying nothing.
	mergeForeignComments(given, source, []mappedComment{})

	file = given.GetFileDiff("config/server.conf")
	if len(file.Comments) != 0 {
		t.Errorf("expected the comment and its reply to be removed, got %d comments", len(file.Comments))
	}
}

func TestMergeLeavesLocalCommentsUntouched(t *testing.T) {
	given := model.NewReview("/repo", "feature", "master")
	file := given.AddFileDiff("config/server.conf")
	local := file.AddComment("my own note", 4, "Test User")
	given.AddComment("my own overall note", "Test User")

	importFixture(t, given, map[string][]model.DiffLineRef{"config/server.conf": diffLinesFor(60, 80)})
	importFixture(t, given, map[string][]model.DiffLineRef{"config/server.conf": diffLinesFor(60, 80)})

	file = given.GetFileDiff("config/server.conf")
	found := false
	for _, comment := range file.Comments {
		if comment.ID == local.ID {
			found = true
			if comment.Content != "my own note" {
				t.Errorf("local content changed: %q", comment.Content)
			}
			if comment.IsForeign() {
				t.Error("expected the local comment to stay local")
			}
		}
	}
	if !found {
		t.Error("expected the local file comment to survive every import")
	}

	localReviewLevel := 0
	for _, comment := range given.Comments {
		if !comment.IsForeign() {
			localReviewLevel++
		}
	}
	if localReviewLevel != 1 {
		t.Errorf("local review-level comments: got %d, want 1", localReviewLevel)
	}
}

func TestMergeLeavesOtherSourcesUntouched(t *testing.T) {
	given := model.NewReview("/repo", "feature", "master")
	file := given.AddFileDiff("config/server.conf")
	other := &model.Comment{
		ID:      "other-1",
		Author:  "someone",
		Content: "from another source",
		Status:  model.CommentStatusActive,
		Origin:  &model.Origin{Source: "gh:88", ForeignID: "99"},
	}
	file.Comments = append(file.Comments, other)

	importFixture(t, given, map[string][]model.DiffLineRef{"config/server.conf": diffLinesFor(60, 80)})

	file = given.GetFileDiff("config/server.conf")
	for _, comment := range file.Comments {
		if comment.ID == "other-1" {
			if comment.Content != "from another source" {
				t.Errorf("other source's content changed: %q", comment.Content)
			}
			return
		}
	}
	t.Error("expected another source's comment to survive the import")
}

// a local reply to a local root must not be removed when an unrelated source's
// threads are replaced.
func TestMergeKeepsLocalReplyToLocalRoot(t *testing.T) {
	given := model.NewReview("/repo", "feature", "master")
	file := given.AddFileDiff("config/server.conf")
	root := file.AddComment("my own note", 4, "Test User")
	file.AddReply(root.ID, "and a follow-up", "Test User")

	importFixture(t, given, map[string][]model.DiffLineRef{"config/server.conf": diffLinesFor(60, 80)})

	file = given.GetFileDiff("config/server.conf")
	replies := 0
	for _, comment := range file.Comments {
		if comment.ParentID == root.ID {
			replies++
		}
	}
	if replies != 1 {
		t.Errorf("local replies to a local root: got %d, want 1", replies)
	}
}

// a file the document comments on that the diff does not carry still gets its
// comments, so the CLI can reach them even though no GUI surface lists the file.
func TestMergeCreatesFileDiffForUnmatchedPath(t *testing.T) {
	given := model.NewReview("/repo", "feature", "master")

	importFixture(t, given, map[string][]model.DiffLineRef{})

	if given.GetFileDiff("config/server.conf") == nil {
		t.Error("expected a file diff to be created for an unmatched path")
	}
	if len(given.GetAllComments()) != 7 {
		t.Errorf("comments: got %d, want 7", len(given.GetAllComments()))
	}
}

func TestMergeReviewLevelCommentGoesToTheReviewSurface(t *testing.T) {
	given := model.NewReview("/repo", "feature", "master")

	importFixture(t, given, map[string][]model.DiffLineRef{})

	if len(given.Comments) != 1 {
		t.Fatalf("review-level comments: got %d, want 1", len(given.Comments))
	}
	if !given.Comments[0].IsForeign() {
		t.Error("expected the review-level comment to be foreign")
	}
}

// a re-imported root whose id is adopted must carry its incoming replies with it,
// rather than leaving them parented to the id the mapping generated.
func TestMergeReparentsIncomingRepliesWhenARootAdoptsItsStoredID(t *testing.T) {
	reply := commentJSON(2, markdownBody("a reply"), false, nil)
	root := commentJSON(1, markdownBody("root"), false, []string{reply})
	raw := documentWith(threadJSON("a.go", `{"kind": "updated", "lines": {"first": 1, "last": 1}}`, root))

	doc, err := decodeForeignDocument([]byte(raw))
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	lines := map[string][]model.DiffLineRef{"a.go": diffLinesFor(1, 10)}
	source := foreignSource(doc, "t1")

	given := model.NewReview("/repo", "feature", "master")
	first, _ := mapForeignDocument(doc, lines, map[string]string{"a.go": "sha"})
	mergeForeignComments(given, source, first)

	storedRootID := ""
	for _, comment := range given.GetFileDiff("a.go").Comments {
		if comment.ParentID == "" {
			storedRootID = comment.ID
		}
	}

	second, _ := mapForeignDocument(doc, lines, map[string]string{"a.go": "sha"})
	mergeForeignComments(given, source, second)

	comments := given.GetFileDiff("a.go").Comments
	if len(comments) != 2 {
		t.Fatalf("comments after re-import: got %d, want 2", len(comments))
	}
	for _, comment := range comments {
		if comment.ParentID == "" {
			if comment.ID != storedRootID {
				t.Errorf("root id changed on re-import: got %q, want %q", comment.ID, storedRootID)
			}
			continue
		}
		if comment.ParentID != storedRootID {
			t.Errorf("reply parent: got %q, want the stored root id %q", comment.ParentID, storedRootID)
		}
	}
}

// report whether two string slices are equal element for element.
func equalStringSlices(a, b []string) bool {
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

// a deleted or pending comment nested as a reply must be skipped too, leaving the
// rest of its thread intact. The root-level skip is the easy case; this is the branch
// inside the reply walk.
func TestMapSkipsADeletedReplyAndKeepsItsSiblings(t *testing.T) {
	first := commentJSON(2, markdownBody("first reply"), false, nil)
	deleted := commentJSON(3, `{"kind": "deleted"}`, false, nil)
	third := commentJSON(4, markdownBody("third reply"), false, nil)
	root := commentJSON(1, markdownBody("root"), false, []string{first, deleted, third})

	doc, err := decodeForeignDocument([]byte(documentWith(
		threadJSON("a.go", `{"kind": "updated", "lines": {"first": 1, "last": 1}}`, root))))
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	lines := map[string][]model.DiffLineRef{"a.go": diffLinesFor(1, 10)}

	mapped, report := mapForeignDocument(doc, lines, map[string]string{"a.go": "sha"})

	// the root plus the two surviving replies.
	if len(mapped) != 3 {
		t.Fatalf("mapped comments: got %d, want 3", len(mapped))
	}
	if report.Skipped != 1 {
		t.Errorf("report skipped: got %d, want 1", report.Skipped)
	}
	if report.Replies != 2 {
		t.Errorf("report replies: got %d, want 2", report.Replies)
	}

	contents := make([]string, 0, len(mapped))
	for _, entry := range mapped {
		contents = append(contents, entry.Comment.Content)
	}
	expected := []string{"root", "first reply", "third reply"}
	if !equalStringSlices(contents, expected) {
		t.Errorf("surviving thread: got %v, want %v", contents, expected)
	}
}

func TestMapSkipsAPendingReply(t *testing.T) {
	pending := commentJSON(2, markdownBody("a draft"), true, nil)
	kept := commentJSON(3, markdownBody("a real reply"), false, nil)
	root := commentJSON(1, markdownBody("root"), false, []string{pending, kept})

	doc, err := decodeForeignDocument([]byte(documentWith(
		threadJSON("a.go", `{"kind": "updated", "lines": {"first": 1, "last": 1}}`, root))))
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	mapped, report := mapForeignDocument(doc, map[string][]model.DiffLineRef{}, map[string]string{})

	if len(mapped) != 2 {
		t.Fatalf("mapped comments: got %d, want 2", len(mapped))
	}
	if report.Skipped != 1 {
		t.Errorf("report skipped: got %d, want 1", report.Skipped)
	}
	if mapped[1].Comment.Content != "a real reply" {
		t.Errorf("surviving reply: got %q, want %q", mapped[1].Comment.Content, "a real reply")
	}
}

// the report must state the skipped count in its rendered lines, not only hold it as
// a field.
func TestReportLinesStateTheSkippedCount(t *testing.T) {
	root := commentJSON(1, `{"kind": "deleted"}`, false, nil)
	doc, err := decodeForeignDocument([]byte(documentWith(
		threadJSON("a.go", `{"kind": "updated", "lines": {"first": 1, "last": 1}}`, root))))
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	_, report := mapForeignDocument(doc, map[string][]model.DiffLineRef{}, map[string]string{})

	joined := strings.Join(report.Lines(), "\n")
	if !strings.Contains(joined, "skipped 1") {
		t.Errorf("report does not state the skipped count: %s", joined)
	}
}

// a re-import must stamp the registry entry with the later time, so the reviewer can
// see when the feedback was last refreshed.
func TestReimportRecordsTheLaterImportTime(t *testing.T) {
	doc := decodeFixture(t)
	lines := map[string][]model.DiffLineRef{"config/server.conf": diffLinesFor(60, 80)}
	given := model.NewReview("/repo", "feature", "master")

	first, _ := mapForeignDocument(doc, lines, map[string]string{})
	mergeForeignComments(given, foreignSource(doc, "2026-09-14T10:00:00Z"), first)

	second, _ := mapForeignDocument(doc, lines, map[string]string{})
	mergeForeignComments(given, foreignSource(doc, "2026-09-14T11:00:00Z"), second)

	if len(given.Sources) != 1 {
		t.Fatalf("sources: got %d, want 1", len(given.Sources))
	}
	actual := given.GetSource("bb:1430")
	if actual.ImportedAt != "2026-09-14T11:00:00Z" {
		t.Errorf("imported at: got %q, want the later time", actual.ImportedAt)
	}
}
