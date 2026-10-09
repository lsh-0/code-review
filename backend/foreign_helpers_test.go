package main

import (
	"code-review/feedback"
	ft "code-review/feedback/feedbacktest"
	"os"
	"testing"
)

// shorthands for the document builders in `feedbacktest`, so the import tests read
// as they did when the builders lived in this package.
const (
	minimalPullRequest = ft.MinimalPullRequest
	humanAuthor        = ft.HumanAuthor
)

var (
	// absolute, so a test that has entered a temporary repository still reads it.
	foreignFixture = ft.Fixture1430

	markdownBody  = ft.MarkdownBody
	commentJSON   = ft.CommentJSON
	threadJSON    = ft.ThreadJSON
	fileThreadRaw = ft.FileThreadRaw
	fileThread    = ft.FileThread
	documentWith  = ft.DocumentWith
)

func decodeFixture(t *testing.T) *feedback.Document {
	t.Helper()
	data, err := os.ReadFile(foreignFixture)
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}
	doc, err := feedback.Decode(data)
	if err != nil {
		t.Fatalf("failed to decode fixture: %v", err)
	}
	return doc
}
