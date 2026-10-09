package feedback

import (
	"os"
	"reflect"
	"testing"

	ft "code-review/feedback/feedbacktest"
)

// any document `Decode` accepts must encode to a document that is schema-valid and
// decodes to an equal value. This is what lets the fetch path pass a normalised
// document through `Encode` and `Decode` without changing it.
//
// Run beyond the seed corpus with:
//
//	go test -run xxx -fuzz FuzzEncodeRoundTrip -fuzztime 60s ./feedback
func FuzzEncodeRoundTrip(f *testing.F) {
	if data, err := os.ReadFile(ft.Fixture1430); err == nil {
		f.Add(data)
	}
	f.Add([]byte(ft.DocumentWith(ft.FileThreadRaw("a.go", `{"kind": "whole_file"}`))))
	f.Add([]byte(ft.DocumentWith(ft.FileThreadRaw("a.go",
		`{"kind": "unchanged", "original_lines": {"first": 1, "last": 2}, "updated_lines": {"first": 3, "last": 4}}`))))
	f.Add([]byte(ft.Document(
		[]string{ft.DiscussionThreadJSON(`{"state": "resolved_without_detail"}`,
			ft.CommentJSON(1, ft.MarkdownBody("a"), false, []string{ft.CommentJSON(2, `{"kind": "deleted"}`, true, nil)}))},
		nil)))

	f.Fuzz(func(t *testing.T, data []byte) {
		given, err := Decode(data)
		if err != nil {
			return
		}

		encoded, err := Encode(*given)
		if err != nil {
			t.Fatalf("an accepted document does not encode: %v", err)
		}

		actual, err := Decode(encoded)
		if err != nil {
			t.Fatalf("an encoded document is rejected: %v\n%s", err, encoded)
		}

		expected := normalised(*given)
		if !reflect.DeepEqual(normalised(*actual), expected) {
			t.Errorf("round trip changed the document:\n%s", encoded)
		}
	})
}

// `doc` with nil lists made empty and fields its tags make meaningless cleared, which
// is the form `Encode` writes. A decoded document can carry either, and both mean the
// same.
func normalised(doc Document) Document {
	clean := func(threads []Thread) []Thread {
		out := make([]Thread, 0, len(threads))
		for _, thread := range threads {
			out = append(out, normalisedThread(thread))
		}
		return out
	}
	doc.Discussion = clean(doc.Discussion)
	doc.FileFeedback = clean(doc.FileFeedback)
	if doc.PullRequest.Participants == nil {
		doc.PullRequest.Participants = []Participant{}
	}
	return doc
}

func normalisedThread(thread Thread) Thread {
	if thread.Location.Kind != LocationFileRevision {
		thread.Location = Location{Kind: thread.Location.Kind}
	}
	if thread.Resolution.State != ResolutionResolved {
		thread.Resolution = Resolution{State: thread.Resolution.State}
	}
	thread.Root = normalisedComment(thread.Root)
	return thread
}

func normalisedComment(comment Comment) Comment {
	replies := make([]Comment, 0, len(comment.Replies))
	for _, reply := range comment.Replies {
		replies = append(replies, normalisedComment(reply))
	}
	comment.Replies = replies
	return comment
}
