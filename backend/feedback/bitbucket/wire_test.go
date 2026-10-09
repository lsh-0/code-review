package bitbucket

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// decodes the fixtures and asserts against their known contents. `aui_5405` is a
// recording of a public pull request. `synthetic_3754` is hand-built.

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "bitbucket", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

func fixturePage(t *testing.T, name string) wirePage[wireComment] {
	t.Helper()
	page, err := decodePage(readFixture(t, name), decodeComment)
	if err != nil {
		t.Fatalf("fixture %s failed to decode: %v", name, err)
	}
	return page
}

func fixturePullRequest(t *testing.T, name string) wirePullRequest {
	t.Helper()
	pr, err := decodePullRequest(readFixture(t, name))
	if err != nil {
		t.Fatalf("fixture %s failed to decode: %v", name, err)
	}
	return pr
}

func filterComments(comments []wireComment, keep func(wireComment) bool) []wireComment {
	kept := make([]wireComment, 0)
	for _, comment := range comments {
		if keep(comment) {
			kept = append(kept, comment)
		}
	}
	return kept
}

func TestWireInlineAnchorPostImageOnly(t *testing.T) {
	given := fixturePage(t, "aui_5405_comments.json").Values

	anchored := filterComments(given, func(c wireComment) bool { return c.Inline != nil })

	if len(anchored) != 1 {
		t.Fatalf("inline comments: got %d, want 1", len(anchored))
	}
	actual := anchored[0].Inline
	if actual.From != nil || actual.StartFrom != nil || actual.StartTo != nil {
		t.Errorf("expected only `to` to be set, got %+v", actual)
	}
	if actual.To == nil || *actual.To != 12 {
		t.Errorf("to: got %v, want 12", actual.To)
	}
	if actual.Path != "tests/a11y-unit-tests/package.json" {
		t.Errorf("path: got %q", actual.Path)
	}
}

func TestWireReplyCarriesParentOnly(t *testing.T) {
	given := fixturePage(t, "aui_5405_comments.json").Values

	replies := filterComments(given, func(c wireComment) bool { return c.ParentID != nil })

	if len(replies) != 1 {
		t.Fatalf("replies: got %d, want 1", len(replies))
	}
	if *replies[0].ParentID != 2008409949 {
		t.Errorf("parent id: got %d, want 2008409949", *replies[0].ParentID)
	}
	if replies[0].Inline != nil {
		t.Error("a reply carries no anchor of its own")
	}
}

func TestWireDeletedCommentHasEmptyBody(t *testing.T) {
	given := fixturePage(t, "aui_5405_comments.json").Values

	deleted := filterComments(given, func(c wireComment) bool { return c.Deleted })

	if len(deleted) != 2 {
		t.Fatalf("deleted comments: got %d, want 2", len(deleted))
	}
	for _, comment := range deleted {
		if comment.BodyRaw != "" {
			t.Errorf("comment %d: deleted body is %q, want empty", comment.ID, comment.BodyRaw)
		}
	}
}

// every application user has no nickname, while humans have one, as observed in
// recorded payloads.
func TestWireApplicationAuthorsLackANickname(t *testing.T) {
	given := fixturePage(t, "synthetic_3754_comments.json").Values

	apps := filterComments(given, func(c wireComment) bool { return c.User.Type == "app_user" })
	humans := filterComments(given, func(c wireComment) bool { return c.User.Type == "user" })

	if len(apps) != 12 {
		t.Errorf("application comments: got %d, want 12", len(apps))
	}
	if len(humans) != 13 {
		t.Errorf("human comments: got %d, want 13", len(humans))
	}
	for _, comment := range apps {
		if comment.User.Nickname != nil {
			t.Errorf("comment %d: application author has a nickname", comment.ID)
		}
		if comment.User.Kind == nil || *comment.User.Kind != "workspace_access_token" {
			t.Errorf("comment %d: kind %v, want workspace_access_token", comment.ID, comment.User.Kind)
		}
	}
	for _, comment := range humans {
		if comment.User.Nickname == nil {
			t.Errorf("comment %d: human author has no nickname", comment.ID)
		}
	}
}

func TestWireDisplayNameIsAlwaysPresent(t *testing.T) {
	given := append(fixturePage(t, "aui_5405_comments.json").Values, fixturePage(t, "synthetic_3754_comments.json").Values...)
	for _, comment := range given {
		if comment.User.DisplayName == "" {
			t.Errorf("comment %d: empty display name", comment.ID)
		}
	}
}

// SYNTHETIC. No recorded payload omits `account_id` or `uuid`, so this covers the
// defensive path only.
func TestWireAuthorWithoutIdentifiersDecodes(t *testing.T) {
	actual, err := decodeUser([]byte(`{"display_name": "Somebody"}`), "user")
	if err != nil {
		t.Fatal(err)
	}
	if actual.AccountID != nil || actual.UUID != nil {
		t.Errorf("expected absent identifiers, got %+v", actual)
	}
	if actual.Type != "user" {
		t.Errorf("type: got %q, want the default user", actual.Type)
	}
}

func TestWireResolutionOnThreadRootsOnlyAndExpanded(t *testing.T) {
	given := fixturePage(t, "synthetic_3754_comments.json").Values

	if len(given) != 25 {
		t.Fatalf("comments: got %d, want 25", len(given))
	}
	resolved := filterComments(given, func(c wireComment) bool { return c.Resolution.State != resolutionAbsent })
	if len(resolved) != 4 {
		t.Errorf("resolved: got %d, want 4", len(resolved))
	}
	for _, comment := range resolved {
		if comment.ParentID != nil {
			t.Errorf("comment %d: resolution on a reply", comment.ID)
		}
		if comment.Resolution.State != resolutionDetailed {
			t.Errorf("comment %d: resolution not detailed, so the fixture lacks the field expansion", comment.ID)
		}
	}
}

func TestWireResolutionStates(t *testing.T) {
	given := []struct {
		name       string
		resolution string
		expected   resolutionState
	}{
		{"absent key", "", resolutionAbsent},
		{"null", `,"resolution": null`, resolutionAbsent},
		{"unexpanded {}", `,"resolution": {}`, resolutionEmpty},
		{"detailed", `,"resolution": {"created_on": "t", "user": {"display_name": "B"}}`, resolutionDetailed},
	}
	for _, test := range given {
		t.Run(test.name, func(t *testing.T) {
			payload := `{"id":1,"created_on":"t","content":{"raw":"x"},"user":{"display_name":"A"}` + test.resolution + `}`
			actual, err := decodeComment([]byte(payload), "comment")
			if err != nil {
				t.Fatal(err)
			}
			if actual.Resolution.State != test.expected {
				t.Errorf("got %v, want %v", actual.Resolution.State, test.expected)
			}
		})
	}
}

func TestWireRejects(t *testing.T) {
	given := []struct {
		name    string
		payload string
		message string
	}{
		{"missing id", `{"created_on":"t","content":{},"user":{"display_name":"A"}}`, "comment.id: required"},
		{"wrong type", `{"id":"one","created_on":"t","content":{},"user":{"display_name":"A"}}`, "comment.id"},
		{"missing user", `{"id":1,"created_on":"t","content":{}}`, "comment.user: required"},
		{"missing content", `{"id":1,"created_on":"t","user":{"display_name":"A"}}`, "comment.content: required"},
		{"to as text", `{"id":1,"created_on":"t","content":{},"user":{"display_name":"A"},"inline":{"path":"a","to":"9"}}`, "comment.inline.to"},
		{"not an object", `[]`, "expected an object"},
	}
	for _, test := range given {
		t.Run(test.name, func(t *testing.T) {
			_, err := decodeComment([]byte(test.payload), "comment")
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Errorf("got %v, want an error containing %q", err, test.message)
			}
		})
	}
}

// unknown keys are ignored, so an addition to the API does not break retrieval.
func TestWireIgnoresUnknownKeys(t *testing.T) {
	payload := `{"id":1,"created_on":"t","content":{"raw":"x","html":"<p>x</p>"},"user":{"display_name":"A","links":{}},"novel":true}`
	if _, err := decodeComment([]byte(payload), "comment"); err != nil {
		t.Errorf("expected unknown keys to be ignored, got %v", err)
	}
}

func TestWirePullRequestFields(t *testing.T) {
	actual := fixturePullRequest(t, "synthetic_3754_pr.json")

	if actual.ID != 3754 || actual.State != "OPEN" {
		t.Errorf("id/state: got %d/%q", actual.ID, actual.State)
	}
	if actual.CommentCount != 24 || actual.TaskCount != 0 {
		t.Errorf("counts: got %d/%d, want 24/0", actual.CommentCount, actual.TaskCount)
	}
	if actual.Description == "" {
		t.Error("expected a description")
	}
	if len(actual.Participants) != 4 {
		t.Errorf("participants: got %d, want 4", len(actual.Participants))
	}
	if actual.Source.Commit == nil || actual.Destination.Commit == nil {
		t.Error("expected both commits")
	}
	if actual.HTMLURL != "https://bitbucket.org/example-org/example-platform/pull-requests/3754" {
		t.Errorf("html url: got %q", actual.HTMLURL)
	}
}

// the collection total counts deleted comments while `comment_count` does not.
// Verified on three pull requests upstream, two of which are committed here.
func TestWireCountInvariantHoldsOnBothFixtures(t *testing.T) {
	given := []struct{ comments, pr string }{
		{"aui_5405_comments.json", "aui_5405_pr.json"},
		{"synthetic_3754_comments.json", "synthetic_3754_pr.json"},
	}
	for _, test := range given {
		comments := fixturePage(t, test.comments).Values
		deleted := filterComments(comments, func(c wireComment) bool { return c.Deleted })
		expected := fixturePullRequest(t, test.pr).CommentCount
		actual := len(comments) - len(deleted)
		if actual != expected {
			t.Errorf("%s: retrieved less deleted is %d, want %d", test.comments, actual, expected)
		}
	}
}

func TestWirePaginationFixturesCarryRealCursors(t *testing.T) {
	first := fixturePage(t, "aui_5405_comments_page1.json")
	second := fixturePage(t, "aui_5405_comments_page2.json")
	third := fixturePage(t, "aui_5405_comments_page3.json")

	if first.Next == nil || second.Next == nil {
		t.Error("expected pages 1 and 2 to carry a next cursor")
	}
	if third.Next != nil {
		t.Error("expected page 3 to be final")
	}
	if total := len(first.Values) + len(second.Values) + len(third.Values); total != 5 {
		t.Errorf("pages concatenate to %d comments, want 5", total)
	}
}

// a saved comments payload replays whether it is a page envelope or a bare array.
func TestDecodeCommentsAcceptsAnEnvelopeOrAnArray(t *testing.T) {
	envelope, err := decodeComments(readFixture(t, "aui_5405_comments.json"))
	if err != nil {
		t.Fatal(err)
	}
	array, err := decodeComments([]byte(`[{"id":1,"created_on":"t","content":{"raw":"x"},"user":{"display_name":"A"}}]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(envelope) != 5 || len(array) != 1 {
		t.Errorf("got %d and %d comments, want 5 and 1", len(envelope), len(array))
	}
}
