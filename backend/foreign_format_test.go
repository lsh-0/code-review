package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// the committed sample of the upstream encoding. Decoding it pins the format's tag
// names, so a change to the upstream encoder fails here rather than at runtime.
const foreignFixture = "testdata/foreign_review_1430.json"

func readForeignFixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(foreignFixture)
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}
	return data
}

func decodeFixture(t *testing.T) *foreignDocument {
	t.Helper()
	doc, err := decodeForeignDocument(readForeignFixture(t))
	if err != nil {
		t.Fatalf("failed to decode fixture: %v", err)
	}
	return doc
}

func TestDecodeFixturePullRequest(t *testing.T) {
	given := decodeFixture(t)

	if given.PullRequest.ID != 1430 {
		t.Errorf("pull request id: got %d, want %d", given.PullRequest.ID, 1430)
	}
	if given.PullRequest.Source.Branch != "feature-1430--telemetry" {
		t.Errorf("source branch: got %q", given.PullRequest.Source.Branch)
	}
	if given.PullRequest.Destination.Branch != "master" {
		t.Errorf("destination branch: got %q", given.PullRequest.Destination.Branch)
	}
	if given.PullRequest.Source.Commit != "8ebf8c861e7f" {
		t.Errorf("source commit: got %q", given.PullRequest.Source.Commit)
	}
}

func TestDecodeFixtureThreadCounts(t *testing.T) {
	given := decodeFixture(t)

	if len(given.Discussion) != 1 {
		t.Errorf("discussion threads: got %d, want 1", len(given.Discussion))
	}
	if len(given.FileFeedback) != 6 {
		t.Errorf("file feedback threads: got %d, want 6", len(given.FileFeedback))
	}

	expected := 7
	actual := len(given.threads())
	if actual != expected {
		t.Errorf("total threads: got %d, want %d", actual, expected)
	}
}

func TestDecodeFixtureLocationsAndPositions(t *testing.T) {
	given := decodeFixture(t)

	if given.Discussion[0].Location.Kind != locationPullRequest {
		t.Errorf("discussion location kind: got %q, want %q", given.Discussion[0].Location.Kind, locationPullRequest)
	}

	for _, thread := range given.FileFeedback {
		if thread.Location.Kind != locationFileRevision {
			t.Errorf("file feedback location kind: got %q, want %q", thread.Location.Kind, locationFileRevision)
		}
		if thread.Location.Path == "" {
			t.Error("expected every file feedback thread to carry a path")
		}
		if thread.Location.Position.Kind != positionUpdated {
			t.Errorf("%s: position kind: got %q, want %q", thread.Location.Path, thread.Location.Position.Kind, positionUpdated)
		}
		if thread.Location.Position.Lines.First != thread.Location.Position.Lines.Last {
			t.Errorf("%s: expected a single-line span, got %d-%d",
				thread.Location.Path, thread.Location.Position.Lines.First, thread.Location.Position.Lines.Last)
		}
	}
}

// the fixture's feedback comes from an automated reviewer, which the format
// reports through `author.kind` rather than by matching the display name.
func TestDecodeFixtureApplicationAuthor(t *testing.T) {
	given := decodeFixture(t)
	author := given.Discussion[0].Root.Author

	if author.Kind != authorApplication {
		t.Errorf("author kind: got %q, want %q", author.Kind, authorApplication)
	}
	if author.ApplicationKind != "workspace_access_token" {
		t.Errorf("application kind: got %q", author.ApplicationKind)
	}
	if author.DisplayName != "review-bot" {
		t.Errorf("display name: got %q", author.DisplayName)
	}
}

func TestDecodeFixtureBodiesAndResolution(t *testing.T) {
	given := decodeFixture(t)

	for _, thread := range given.threads() {
		if thread.Root.Body.Kind != bodyMarkdown {
			t.Errorf("body kind: got %q, want %q", thread.Root.Body.Kind, bodyMarkdown)
		}
		if thread.Root.Body.Text == "" {
			t.Error("expected a markdown body to carry text")
		}
		if thread.Resolution.State != resolutionUnresolved {
			t.Errorf("resolution state: got %q, want %q", thread.Resolution.State, resolutionUnresolved)
		}
		if thread.Root.Pending {
			t.Error("expected no fixture comment to be pending")
		}
		if len(thread.Root.Replies) != 0 {
			t.Errorf("expected no fixture replies, got %d", len(thread.Root.Replies))
		}
	}
}

// a document whose collections contradict their threads' own locations is
// repartitioned rather than trusted, matching the upstream decoder.
func TestDecodeClassifiesThreadByItsOwnLocation(t *testing.T) {
	given := `{
		"pull_request": ` + minimalPullRequest + `,
		"discussion": [` + fileThread("main.go", positionUpdated, 10, 10) + `],
		"file_feedback": []
	}`

	doc, err := decodeForeignDocument([]byte(given))
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	threads := doc.threads()
	if len(threads) != 1 {
		t.Fatalf("threads: got %d, want 1", len(threads))
	}
	if threads[0].Location.Kind != locationFileRevision {
		t.Errorf("expected the thread's own location to classify it, got %q", threads[0].Location.Kind)
	}
	if threads[0].Location.Path != "main.go" {
		t.Errorf("path: got %q, want %q", threads[0].Location.Path, "main.go")
	}
}

func TestDecodeAcceptsEveryPositionKind(t *testing.T) {
	given := []struct {
		name string
		json string
	}{
		{"whole file", fileThreadRaw("a.go", `{"kind": "whole_file"}`)},
		{"original", fileThreadRaw("a.go", `{"kind": "original", "lines": {"first": 4, "last": 4}}`)},
		{"updated", fileThreadRaw("a.go", `{"kind": "updated", "lines": {"first": 4, "last": 9}}`)},
		{"unchanged", fileThreadRaw("a.go",
			`{"kind": "unchanged", "original_lines": {"first": 4, "last": 4}, "updated_lines": {"first": 7, "last": 7}}`)},
	}

	for _, test := range given {
		t.Run(test.name, func(t *testing.T) {
			doc, err := decodeForeignDocument([]byte(documentWith(test.json)))
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}
			if len(doc.FileFeedback) != 1 {
				t.Fatalf("file feedback: got %d, want 1", len(doc.FileFeedback))
			}
		})
	}
}

func TestDecodeMultiLineSpan(t *testing.T) {
	doc, err := decodeForeignDocument([]byte(documentWith(fileThread("a.go", positionUpdated, 12, 20))))
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	position := doc.FileFeedback[0].Location.Position
	if position.Lines.First != 12 || position.Lines.Last != 20 {
		t.Errorf("span: got %d-%d, want 12-20", position.Lines.First, position.Lines.Last)
	}
}

func TestDecodeUnchangedPositionKeepsBothSpans(t *testing.T) {
	given := fileThreadRaw("a.go",
		`{"kind": "unchanged", "original_lines": {"first": 4, "last": 5}, "updated_lines": {"first": 7, "last": 8}}`)

	doc, err := decodeForeignDocument([]byte(documentWith(given)))
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	position := doc.FileFeedback[0].Location.Position
	if position.OriginalLines.First != 4 || position.OriginalLines.Last != 5 {
		t.Errorf("original span: got %d-%d, want 4-5", position.OriginalLines.First, position.OriginalLines.Last)
	}
	if position.UpdatedLines.First != 7 || position.UpdatedLines.Last != 8 {
		t.Errorf("updated span: got %d-%d, want 7-8", position.UpdatedLines.First, position.UpdatedLines.Last)
	}
}

func TestDecodeDeletedBodyAndPendingComment(t *testing.T) {
	given := documentWith(
		threadJSON("main.go", `{"kind": "updated", "lines": {"first": 1, "last": 1}}`,
			commentJSON(1, `{"kind": "deleted"}`, true, nil)))

	doc, err := decodeForeignDocument([]byte(given))
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	root := doc.FileFeedback[0].Root
	if root.Body.Kind != bodyDeleted {
		t.Errorf("body kind: got %q, want %q", root.Body.Kind, bodyDeleted)
	}
	if !root.Pending {
		t.Error("expected the comment to be pending")
	}
}

func TestDecodeNestedRepliesToDepthThree(t *testing.T) {
	deepest := commentJSON(4, markdownBody("deepest"), false, nil)
	middle := commentJSON(3, markdownBody("middle"), false, []string{deepest})
	inner := commentJSON(2, markdownBody("inner"), false, []string{middle})
	root := commentJSON(1, markdownBody("root"), false, []string{inner})

	given := documentWith(threadJSON("main.go", `{"kind": "updated", "lines": {"first": 1, "last": 1}}`, root))

	doc, err := decodeForeignDocument([]byte(given))
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	actual := doc.FileFeedback[0].Root
	if len(actual.Replies) != 1 {
		t.Fatalf("root replies: got %d, want 1", len(actual.Replies))
	}
	if len(actual.Replies[0].Replies) != 1 {
		t.Fatalf("second-level replies: got %d, want 1", len(actual.Replies[0].Replies))
	}
	if len(actual.Replies[0].Replies[0].Replies) != 1 {
		t.Fatalf("third-level replies: got %d, want 1", len(actual.Replies[0].Replies[0].Replies))
	}
	if actual.Replies[0].Replies[0].Replies[0].ID != 4 {
		t.Errorf("deepest reply id: got %d, want 4", actual.Replies[0].Replies[0].Replies[0].ID)
	}
}

func TestDecodeResolvedResolution(t *testing.T) {
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

	resolution := doc.Discussion[0].Resolution
	if resolution.State != resolutionResolved {
		t.Errorf("state: got %q, want %q", resolution.State, resolutionResolved)
	}
	if resolution.By.DisplayName != "Luke" {
		t.Errorf("resolved by: got %q, want %q", resolution.By.DisplayName, "Luke")
	}
}

func TestDecodeResolvedWithoutDetail(t *testing.T) {
	given := `{
		"pull_request": ` + minimalPullRequest + `,
		"discussion": [{
			"location": {"kind": "pull_request"},
			"resolution": {"state": "resolved_without_detail"},
			"root": ` + commentJSON(1, markdownBody("overall"), false, nil) + `
		}],
		"file_feedback": []
	}`

	doc, err := decodeForeignDocument([]byte(given))
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if doc.Discussion[0].Resolution.State != resolutionResolvedWithoutInfo {
		t.Errorf("state: got %q, want %q", doc.Discussion[0].Resolution.State, resolutionResolvedWithoutInfo)
	}
}

// every union tag must be pinned: an unrecognised value is an error naming the
// tag and its field, never a silent default to a known case.
func TestDecodeRejectsUnrecognisedTags(t *testing.T) {
	given := []struct {
		name     string
		document string
		wantIn   string
	}{
		{
			name: "location kind",
			document: `{"pull_request": ` + minimalPullRequest + `, "discussion": [{
				"location": {"kind": "somewhere_else"},
				"resolution": {"state": "unresolved"},
				"root": ` + commentJSON(1, markdownBody("x"), false, nil) + `}], "file_feedback": []}`,
			wantIn: "location.kind",
		},
		{
			name:     "position kind",
			document: documentWith(fileThreadRaw("a.go", `{"kind": "sideways", "lines": {"first": 1, "last": 1}}`)),
			wantIn:   "position.kind",
		},
		{
			name: "body kind",
			document: documentWith(threadJSON("a.go", `{"kind": "updated", "lines": {"first": 1, "last": 1}}`,
				commentJSON(1, `{"kind": "rich_text", "text": "x"}`, false, nil))),
			wantIn: "body.kind",
		},
		{
			name: "author kind",
			document: documentWith(threadJSON("a.go", `{"kind": "updated", "lines": {"first": 1, "last": 1}}`,
				`{"id": 1, "author": {"display_name": "x", "kind": "robot"},
				  "body": `+markdownBody("x")+`, "created_at": "2026-09-04T03:59:34.961609Z",
				  "pending": false, "replies": []}`)),
			wantIn: "author.kind",
		},
		{
			name: "resolution state",
			document: `{"pull_request": ` + minimalPullRequest + `, "discussion": [{
				"location": {"kind": "pull_request"},
				"resolution": {"state": "half_done"},
				"root": ` + commentJSON(1, markdownBody("x"), false, nil) + `}], "file_feedback": []}`,
			wantIn: "resolution.state",
		},
	}

	for _, test := range given {
		t.Run(test.name, func(t *testing.T) {
			_, err := decodeForeignDocument([]byte(test.document))
			if err == nil {
				t.Fatal("expected an error for an unrecognised tag")
			}
			if !strings.Contains(err.Error(), test.wantIn) {
				t.Errorf("error %q does not name %q", err.Error(), test.wantIn)
			}
		})
	}
}

// an unrecognised tag nested inside a reply must be rejected too, not only one on
// a root comment.
func TestDecodeRejectsUnrecognisedTagInAReply(t *testing.T) {
	reply := `{"id": 2, "author": ` + humanAuthor + `, "body": {"kind": "rich_text", "text": "x"},
		"created_at": "2026-09-04T03:59:34.961609Z", "pending": false, "replies": []}`
	root := commentJSON(1, markdownBody("root"), false, []string{reply})
	given := documentWith(threadJSON("a.go", `{"kind": "updated", "lines": {"first": 1, "last": 1}}`, root))

	_, err := decodeForeignDocument([]byte(given))
	if err == nil {
		t.Fatal("expected an error for an unrecognised tag in a reply")
	}
	if !strings.Contains(err.Error(), "body.kind") {
		t.Errorf("error %q does not name body.kind", err.Error())
	}
}

func TestDecodeRejectsMalformedDocuments(t *testing.T) {
	given := []struct {
		name     string
		document string
	}{
		{"invalid json", `{"pull_request": `},
		{"not an object", `[]`},
		{"no pull request id", `{"pull_request": {"title": "x"}, "discussion": [], "file_feedback": []}`},
		{
			name:     "span first beyond last",
			document: documentWith(fileThread("a.go", positionUpdated, 20, 12)),
		},
		{
			name:     "span below the first line",
			document: documentWith(fileThread("a.go", positionUpdated, 0, 0)),
		},
		{
			name: "unchanged position with a malformed updated span",
			document: documentWith(fileThreadRaw("a.go",
				`{"kind": "unchanged", "original_lines": {"first": 4, "last": 4}, "updated_lines": {"first": 9, "last": 2}}`)),
		},
		{
			name:     "file revision thread with no path",
			document: documentWith(fileThread("", positionUpdated, 1, 1)),
		},
		{
			name: "wrong type for a line number",
			document: documentWith(fileThreadRaw("a.go",
				`{"kind": "updated", "lines": {"first": "one", "last": "one"}}`)),
		},
		{
			// upstream declares a markdown body's text never empty, so a body with
			// no text is malformed. Accepting it would map to a comment the
			// reviewer cannot read or act on.
			name: "markdown body with no text",
			document: documentWith(threadJSON("a.go", `{"kind": "updated", "lines": {"first": 1, "last": 1}}`,
				`{"id": 1, "author": `+humanAuthor+`, "body": {"kind": "markdown"},
				  "created_at": "2026-09-04T03:59:34.961609Z", "pending": false, "replies": []}`)),
		},
		{
			name: "markdown body with blank text",
			document: documentWith(threadJSON("a.go", `{"kind": "updated", "lines": {"first": 1, "last": 1}}`,
				`{"id": 1, "author": `+humanAuthor+`, "body": {"kind": "markdown", "text": "   "},
				  "created_at": "2026-09-04T03:59:34.961609Z", "pending": false, "replies": []}`)),
		},
		{
			// upstream declares a display name never empty.
			name: "author with no display name",
			document: documentWith(threadJSON("a.go", `{"kind": "updated", "lines": {"first": 1, "last": 1}}`,
				`{"id": 1, "author": {"display_name": "", "kind": "human"}, "body": `+markdownBody("x")+`,
				  "created_at": "2026-09-04T03:59:34.961609Z", "pending": false, "replies": []}`)),
		},
		{
			// the same invariants must hold for a nested reply, not only a root.
			name: "reply with an empty markdown body",
			document: documentWith(threadJSON("a.go", `{"kind": "updated", "lines": {"first": 1, "last": 1}}`,
				commentJSON(1, markdownBody("root"), false, []string{
					`{"id": 2, "author": ` + humanAuthor + `, "body": {"kind": "markdown", "text": ""},
					  "created_at": "2026-09-04T03:59:34.961609Z", "pending": false, "replies": []}`,
				}))),
		},
	}

	for _, test := range given {
		t.Run(test.name, func(t *testing.T) {
			if _, err := decodeForeignDocument([]byte(test.document)); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

// the fixture must stay in step with the upstream encoder. A missing file is a
// failure rather than a skip: the pinning is the point of the test.
func TestForeignFixtureIsPresent(t *testing.T) {
	if _, err := os.Stat(filepath.Clean(foreignFixture)); err != nil {
		t.Fatalf("fixture missing: %v", err)
	}
}

// {1 Document builders for the cases the fixture does not exercise}

const minimalPullRequest = `{
	"id": 1430,
	"title": "A title",
	"description": null,
	"state": "open",
	"draft": false,
	"author": {"display_name": "Luke", "account_id": "a", "uuid": "u", "nickname": "Luke", "kind": "human"},
	"created_at": "2026-09-04T03:54:57.776968Z",
	"updated_at": null,
	"source": {"branch": "feature", "commit": "abc123"},
	"destination": {"branch": "master", "commit": "def456"},
	"participants": [],
	"reported_comment_count": 0,
	"reported_task_count": 0
}`

const humanAuthor = `{"display_name": "Luke", "account_id": "a", "uuid": "u", "nickname": "Luke", "kind": "human"}`

func markdownBody(text string) string {
	return `{"kind": "markdown", "text": "` + text + `"}`
}

func commentJSON(id int, body string, pending bool, replies []string) string {
	pendingText := "false"
	if pending {
		pendingText = "true"
	}
	return `{
		"id": ` + strconv.Itoa(id) + `,
		"author": ` + humanAuthor + `,
		"body": ` + body + `,
		"created_at": "2026-09-04T03:59:34.961609Z",
		"updated_at": null,
		"pending": ` + pendingText + `,
		"replies": [` + strings.Join(replies, ",") + `]
	}`
}

func threadJSON(path string, position string, root string) string {
	return `{
		"location": {"kind": "file_revision", "path": "` + path + `", "position": ` + position + `},
		"resolution": {"state": "unresolved"},
		"root": ` + root + `
	}`
}

func fileThreadRaw(path string, position string) string {
	return threadJSON(path, position, commentJSON(1, markdownBody("a finding"), false, nil))
}

func fileThread(path string, kind string, first int, last int) string {
	position := `{"kind": "` + kind + `", "lines": {"first": ` + strconv.Itoa(first) + `, "last": ` + strconv.Itoa(last) + `}}`
	return fileThreadRaw(path, position)
}

// wrap one file-feedback thread in an otherwise minimal document.
func documentWith(thread string) string {
	return `{
		"pull_request": ` + minimalPullRequest + `,
		"discussion": [],
		"file_feedback": [` + thread + `]
	}`
}
