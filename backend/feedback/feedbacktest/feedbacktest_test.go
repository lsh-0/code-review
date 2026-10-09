package feedbacktest

import (
	"encoding/json"
	"os"
	"testing"
)

// every builder produces well-formed JSON, so a test that fails on a built document
// fails on what it varies and not on a typo in a builder.
func TestBuildersProduceJSON(t *testing.T) {
	given := map[string]string{
		"minimal pull request": MinimalPullRequest,
		"document with":        DocumentWith(FileThread("a.go", "updated", 1, 2)),
		"document":             Document([]string{DiscussionThreadJSON(`{"state": "unresolved"}`, CommentJSON(1, MarkdownBody("x"), false, nil))}, nil),
		"raw thread":           DocumentWith(FileThreadRaw("a.go", `{"kind": "whole_file"}`)),
		"comment by":           CommentByJSON(1, HumanAuthor, `{"kind": "deleted"}`, true, []string{CommentJSON(2, MarkdownBody("y"), false, nil)}),
	}
	for name, document := range given {
		if !json.Valid([]byte(document)) {
			t.Errorf("%s: not valid JSON:\n%s", name, document)
		}
	}
}

func TestFixturePathExists(t *testing.T) {
	if _, err := os.Stat(Fixture1430); err != nil {
		t.Errorf("fixture path %q: %v", Fixture1430, err)
	}
	if _, err := os.Stat(Testdata("bitbucket")); err != nil {
		t.Errorf("testdata path: %v", err)
	}
}
