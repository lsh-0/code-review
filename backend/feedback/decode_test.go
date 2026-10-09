package feedback

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	ft "code-review/feedback/feedbacktest"
)

// decoding the committed sample of upstream output pins the format's tag names, so a
// change to the upstream encoder fails here and not at runtime.

func readForeignFixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(ft.Fixture1430)
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}
	return data
}

func decodeFixture(t *testing.T) *Document {
	t.Helper()
	doc, err := Decode(readForeignFixture(t))
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
	if given.PullRequest.Source.Commit == nil || *given.PullRequest.Source.Commit != "8ebf8c861e7f" {
		t.Errorf("source commit: got %v", given.PullRequest.Source.Commit)
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
	actual := len(given.Threads())
	if actual != expected {
		t.Errorf("total threads: got %d, want %d", actual, expected)
	}
}

func TestDecodeFixtureLocationsAndPositions(t *testing.T) {
	given := decodeFixture(t)

	if given.Discussion[0].Location.Kind != LocationPullRequest {
		t.Errorf("discussion location kind: got %q, want %q", given.Discussion[0].Location.Kind, LocationPullRequest)
	}

	for _, thread := range given.FileFeedback {
		if thread.Location.Kind != LocationFileRevision {
			t.Errorf("file feedback location kind: got %q, want %q", thread.Location.Kind, LocationFileRevision)
		}
		if thread.Location.Path == "" {
			t.Error("expected every file feedback thread to carry a path")
		}
		if thread.Location.Position.Kind != PositionUpdated {
			t.Errorf("%s: position kind: got %q, want %q", thread.Location.Path, thread.Location.Position.Kind, PositionUpdated)
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

	if author.Kind != AuthorApplication {
		t.Errorf("author kind: got %q, want %q", author.Kind, AuthorApplication)
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

	for _, thread := range given.Threads() {
		if thread.Root.Body.Kind != BodyMarkdown {
			t.Errorf("body kind: got %q, want %q", thread.Root.Body.Kind, BodyMarkdown)
		}
		if thread.Root.Body.Text == "" {
			t.Error("expected a markdown body to carry text")
		}
		if thread.Resolution.State != ResolutionUnresolved {
			t.Errorf("resolution state: got %q, want %q", thread.Resolution.State, ResolutionUnresolved)
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
		"pull_request": ` + ft.MinimalPullRequest + `,
		"discussion": [` + ft.FileThread("main.go", PositionUpdated, 10, 10) + `],
		"file_feedback": []
	}`

	doc, err := Decode([]byte(given))
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	threads := doc.Threads()
	if len(threads) != 1 {
		t.Fatalf("threads: got %d, want 1", len(threads))
	}
	if threads[0].Location.Kind != LocationFileRevision {
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
		{"whole file", ft.FileThreadRaw("a.go", `{"kind": "whole_file"}`)},
		{"original", ft.FileThreadRaw("a.go", `{"kind": "original", "lines": {"first": 4, "last": 4}}`)},
		{"updated", ft.FileThreadRaw("a.go", `{"kind": "updated", "lines": {"first": 4, "last": 9}}`)},
		{"unchanged", ft.FileThreadRaw("a.go",
			`{"kind": "unchanged", "original_lines": {"first": 4, "last": 4}, "updated_lines": {"first": 7, "last": 7}}`)},
	}

	for _, test := range given {
		t.Run(test.name, func(t *testing.T) {
			doc, err := Decode([]byte(ft.DocumentWith(test.json)))
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
	doc, err := Decode([]byte(ft.DocumentWith(ft.FileThread("a.go", PositionUpdated, 12, 20))))
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	position := doc.FileFeedback[0].Location.Position
	if position.Lines.First != 12 || position.Lines.Last != 20 {
		t.Errorf("span: got %d-%d, want 12-20", position.Lines.First, position.Lines.Last)
	}
}

func TestDecodeUnchangedPositionKeepsBothSpans(t *testing.T) {
	given := ft.FileThreadRaw("a.go",
		`{"kind": "unchanged", "original_lines": {"first": 4, "last": 5}, "updated_lines": {"first": 7, "last": 8}}`)

	doc, err := Decode([]byte(ft.DocumentWith(given)))
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
	given := ft.DocumentWith(
		ft.ThreadJSON("main.go", `{"kind": "updated", "lines": {"first": 1, "last": 1}}`,
			ft.CommentJSON(1, `{"kind": "deleted"}`, true, nil)))

	doc, err := Decode([]byte(given))
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	root := doc.FileFeedback[0].Root
	if root.Body.Kind != BodyDeleted {
		t.Errorf("body kind: got %q, want %q", root.Body.Kind, BodyDeleted)
	}
	if !root.Pending {
		t.Error("expected the comment to be pending")
	}
}

func TestDecodeNestedRepliesToDepthThree(t *testing.T) {
	deepest := ft.CommentJSON(4, ft.MarkdownBody("deepest"), false, nil)
	middle := ft.CommentJSON(3, ft.MarkdownBody("middle"), false, []string{deepest})
	inner := ft.CommentJSON(2, ft.MarkdownBody("inner"), false, []string{middle})
	root := ft.CommentJSON(1, ft.MarkdownBody("root"), false, []string{inner})

	given := ft.DocumentWith(ft.ThreadJSON("main.go", `{"kind": "updated", "lines": {"first": 1, "last": 1}}`, root))

	doc, err := Decode([]byte(given))
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
		"pull_request": ` + ft.MinimalPullRequest + `,
		"discussion": [{
			"location": {"kind": "pull_request"},
			"resolution": {"state": "resolved", "by": ` + ft.HumanAuthor + `, "at": "2026-09-04T03:59:34.961609Z"},
			"root": ` + ft.CommentJSON(1, ft.MarkdownBody("overall"), false, nil) + `
		}],
		"file_feedback": []
	}`

	doc, err := Decode([]byte(given))
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	resolution := doc.Discussion[0].Resolution
	if resolution.State != ResolutionResolved {
		t.Errorf("state: got %q, want %q", resolution.State, ResolutionResolved)
	}
	if resolution.By == nil || resolution.By.DisplayName != "Luke" {
		t.Errorf("resolved by: got %q, want %q", resolution.By.DisplayName, "Luke")
	}
}

func TestDecodeResolvedWithoutDetail(t *testing.T) {
	given := `{
		"pull_request": ` + ft.MinimalPullRequest + `,
		"discussion": [{
			"location": {"kind": "pull_request"},
			"resolution": {"state": "resolved_without_detail"},
			"root": ` + ft.CommentJSON(1, ft.MarkdownBody("overall"), false, nil) + `
		}],
		"file_feedback": []
	}`

	doc, err := Decode([]byte(given))
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if doc.Discussion[0].Resolution.State != ResolutionResolvedWithoutInfo {
		t.Errorf("state: got %q, want %q", doc.Discussion[0].Resolution.State, ResolutionResolvedWithoutInfo)
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
			document: `{"pull_request": ` + ft.MinimalPullRequest + `, "discussion": [{
				"location": {"kind": "somewhere_else"},
				"resolution": {"state": "unresolved"},
				"root": ` + ft.CommentJSON(1, ft.MarkdownBody("x"), false, nil) + `}], "file_feedback": []}`,
			wantIn: "/discussion/0/location/kind: value must be one of",
		},
		{
			name:     "position kind",
			document: ft.DocumentWith(ft.FileThreadRaw("a.go", `{"kind": "sideways", "lines": {"first": 1, "last": 1}}`)),
			wantIn:   "/file_feedback/0/location/position/kind: value must be one of",
		},
		{
			name: "body kind",
			document: ft.DocumentWith(ft.ThreadJSON("a.go", `{"kind": "updated", "lines": {"first": 1, "last": 1}}`,
				ft.CommentJSON(1, `{"kind": "rich_text", "text": "x"}`, false, nil))),
			wantIn: "/file_feedback/0/root/body/kind: value must be one of",
		},
		{
			name: "author kind",
			document: ft.DocumentWith(ft.ThreadJSON("a.go", `{"kind": "updated", "lines": {"first": 1, "last": 1}}`,
				`{"id": 1, "author": {"display_name": "x", "kind": "robot"},
				  "body": `+ft.MarkdownBody("x")+`, "created_at": "2026-09-04T03:59:34.961609Z",
				  "pending": false, "replies": []}`)),
			wantIn: "/file_feedback/0/root/author/kind: value must be one of",
		},
		{
			name: "resolution state",
			document: `{"pull_request": ` + ft.MinimalPullRequest + `, "discussion": [{
				"location": {"kind": "pull_request"},
				"resolution": {"state": "half_done"},
				"root": ` + ft.CommentJSON(1, ft.MarkdownBody("x"), false, nil) + `}], "file_feedback": []}`,
			wantIn: "/discussion/0/resolution/state: value must be one of",
		},
	}

	for _, test := range given {
		t.Run(test.name, func(t *testing.T) {
			_, err := Decode([]byte(test.document))
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
	reply := `{"id": 2, "author": ` + ft.HumanAuthor + `, "body": {"kind": "rich_text", "text": "x"},
		"created_at": "2026-09-04T03:59:34.961609Z", "pending": false, "replies": []}`
	root := ft.CommentJSON(1, ft.MarkdownBody("root"), false, []string{reply})
	given := ft.DocumentWith(ft.ThreadJSON("a.go", `{"kind": "updated", "lines": {"first": 1, "last": 1}}`, root))

	_, err := Decode([]byte(given))
	if err == nil {
		t.Fatal("expected an error for an unrecognised tag in a reply")
	}
	if !strings.Contains(err.Error(), "/file_feedback/0/root/replies/0/body/kind") {
		t.Errorf("error %q does not name the reply's body kind", err.Error())
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
			document: ft.DocumentWith(ft.FileThread("a.go", PositionUpdated, 20, 12)),
		},
		{
			name:     "span below the first line",
			document: ft.DocumentWith(ft.FileThread("a.go", PositionUpdated, 0, 0)),
		},
		{
			name: "unchanged position with a malformed updated span",
			document: ft.DocumentWith(ft.FileThreadRaw("a.go",
				`{"kind": "unchanged", "original_lines": {"first": 4, "last": 4}, "updated_lines": {"first": 9, "last": 2}}`)),
		},
		{
			name:     "file revision thread with no path",
			document: ft.DocumentWith(ft.FileThread("", PositionUpdated, 1, 1)),
		},
		{
			name: "wrong type for a line number",
			document: ft.DocumentWith(ft.FileThreadRaw("a.go",
				`{"kind": "updated", "lines": {"first": "one", "last": "one"}}`)),
		},
		{
			// upstream declares a markdown body's text never empty, so a body with
			// no text is malformed. Accepting it would map to a comment the
			// reviewer cannot read or act on.
			name: "markdown body with no text",
			document: ft.DocumentWith(ft.ThreadJSON("a.go", `{"kind": "updated", "lines": {"first": 1, "last": 1}}`,
				`{"id": 1, "author": `+ft.HumanAuthor+`, "body": {"kind": "markdown"},
				  "created_at": "2026-09-04T03:59:34.961609Z", "pending": false, "replies": []}`)),
		},
		{
			name: "markdown body with blank text",
			document: ft.DocumentWith(ft.ThreadJSON("a.go", `{"kind": "updated", "lines": {"first": 1, "last": 1}}`,
				`{"id": 1, "author": `+ft.HumanAuthor+`, "body": {"kind": "markdown", "text": "   "},
				  "created_at": "2026-09-04T03:59:34.961609Z", "pending": false, "replies": []}`)),
		},
		{
			// upstream declares a display name never empty.
			name: "author with no display name",
			document: ft.DocumentWith(ft.ThreadJSON("a.go", `{"kind": "updated", "lines": {"first": 1, "last": 1}}`,
				`{"id": 1, "author": {"display_name": "", "kind": "human"}, "body": `+ft.MarkdownBody("x")+`,
				  "created_at": "2026-09-04T03:59:34.961609Z", "pending": false, "replies": []}`)),
		},
		{
			// the same invariants must hold for a nested reply, not only a root.
			name: "reply with an empty markdown body",
			document: ft.DocumentWith(ft.ThreadJSON("a.go", `{"kind": "updated", "lines": {"first": 1, "last": 1}}`,
				ft.CommentJSON(1, ft.MarkdownBody("root"), false, []string{
					`{"id": 2, "author": ` + ft.HumanAuthor + `, "body": {"kind": "markdown", "text": ""},
					  "created_at": "2026-09-04T03:59:34.961609Z", "pending": false, "replies": []}`,
				}))),
		},
	}

	for _, test := range given {
		t.Run(test.name, func(t *testing.T) {
			if _, err := Decode([]byte(test.document)); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

// the fixture must stay in step with the upstream encoder. A missing file is a
// failure rather than a skip: the pinning is the point of the test.
func TestForeignFixtureIsPresent(t *testing.T) {
	if _, err := os.Stat(ft.Fixture1430); err != nil {
		t.Fatalf("fixture missing: %v", err)
	}
}

// the fields the import ignored before `Check` needed them are now read, so
// validation sees the count the source reported and who took part.
func TestDecodeFixtureReadsReconciliationFields(t *testing.T) {
	given := decodeFixture(t)

	expected := 7
	actual := given.PullRequest.ReportedCommentCount
	if actual != expected {
		t.Errorf("reported comment count: got %d, want %d", actual, expected)
	}
	if len(given.PullRequest.Participants) == 0 {
		t.Error("expected the fixture's participants to be read")
	}
	if given.PullRequest.Participants[0].Role != "reviewer" {
		t.Errorf("participant role: got %q, want %q", given.PullRequest.Participants[0].Role, "reviewer")
	}
	if given.PullRequest.Description == nil {
		t.Error("expected the fixture's description to be read")
	}
	if given.SchemaVersion != 0 {
		t.Errorf("schema version: got %d, want 0 for a document that states none", given.SchemaVersion)
	}
}

// encoding a decoded upstream document reproduces it, so a document the port builds
// is indistinguishable from one upstream wrote.
func TestEncodeReproducesUpstreamOutput(t *testing.T) {
	expected, err := os.ReadFile(ft.Fixture1430)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Decode(expected)
	if err != nil {
		t.Fatal(err)
	}

	actual, err := Encode(*doc)
	if err != nil {
		t.Fatal(err)
	}

	if !jsonEqual(t, actual, expected) {
		t.Errorf("encoded document differs from upstream output:\n%s", actual)
	}
}

// an encoded document always passes the schema, whatever the decoder was given.
func TestEncodeOutputIsSchemaValid(t *testing.T) {
	given := ft.Document(
		[]string{ft.DiscussionThreadJSON(`{"state": "resolved", "by": `+ft.HumanAuthor+`, "at": "`+ft.CreatedAt+`"}`,
			ft.CommentJSON(1, ft.MarkdownBody("overall"), false, nil))},
		[]string{
			ft.FileThreadRaw("a.go", `{"kind": "whole_file"}`),
			ft.FileThreadRaw("b.go", `{"kind": "unchanged", "original_lines": {"first": 1, "last": 2}, "updated_lines": {"first": 3, "last": 4}}`),
			ft.ThreadJSON("c.go", `{"kind": "original", "lines": {"first": 5, "last": 5}}`,
				ft.CommentByJSON(2, `{"display_name": "bot", "kind": "application", "application_kind": "token"}`, `{"kind": "deleted"}`, true, nil)),
		},
	)
	doc, err := Decode([]byte(given))
	if err != nil {
		t.Fatal(err)
	}

	encoded, err := Encode(*doc)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(encoded); err != nil {
		t.Errorf("encoded document fails to decode: %v\n%s", err, encoded)
	}
}

// reports whether `a` and `b` hold the same JSON value, ignoring key order and
// whitespace.
func jsonEqual(t *testing.T, a []byte, b []byte) bool {
	t.Helper()
	var left, right any
	if err := json.Unmarshal(a, &left); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &right); err != nil {
		t.Fatal(err)
	}
	return reflect.DeepEqual(left, right)
}
