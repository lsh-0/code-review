package bitbucket

import (
	"reflect"
	"sort"
	"testing"
	"time"

	"code-review/feedback"
)

// properties of normalisation over generated comment lists, ported from upstream's
// QCheck suite. The fuzzer supplies bytes, and `generated` reads them as a comment
// list, so every byte string is a valid input and the fuzzer explores the
// structures directly: parent links that dangle, loop, or repeat identifiers, and
// anchors in every combination.
//
// Run beyond the seed corpus with:
//
//	go test -run xxx -fuzz FuzzNormalise -fuzztime 60s ./feedback/bitbucket

// a cursor over fuzzer bytes. Past the end every read yields zero, so any input
// reads as a complete comment list.
type bytesource struct {
	data []byte
	at   int
}

func (b *bytesource) next() int {
	if b.at >= len(b.data) {
		return 0
	}
	value := int(b.data[b.at])
	b.at++
	return value
}

// a value in [0, n).
func (b *bytesource) below(n int) int { return b.next() % n }

func (b *bytesource) optionalLine() *int {
	if b.below(2) == 0 {
		return nil
	}
	line := b.below(60) // 0 is a deliberately out-of-range line
	return &line
}

// reads up to 24 comments with identifiers drawn from a small range, so duplicates,
// cycles and orphans arise often.
func generated(data []byte) []wireComment {
	source := &bytesource{data: data}
	count := source.below(25)
	comments := make([]wireComment, 0, count)
	types := []string{"user", "app_user", "martian"}
	for range count {
		comment := wireComment{
			ID:        1 + source.below(30),
			CreatedOn: time.Date(2026, 8, 4, source.below(24), source.below(60), 0, 0, time.UTC).Format(time.RFC3339),
			BodyRaw:   []string{"text", "", " ", "more"}[source.below(4)],
			Deleted:   source.below(5) == 0,
			Pending:   source.below(7) == 0,
			User:      wireUser{DisplayName: []string{"A", "B", ""}[source.below(3)], Type: types[source.below(3)]},
		}
		if source.below(3) == 0 {
			comment.CreatedOn = "not a time"
		}
		if source.below(2) == 0 {
			parent := 1 + source.below(32)
			comment.ParentID = &parent
		}
		if source.below(2) == 0 {
			comment.Inline = &wireInline{
				Path:      []string{"a.go", "b.go", ""}[source.below(3)],
				From:      source.optionalLine(),
				To:        source.optionalLine(),
				StartFrom: source.optionalLine(),
				StartTo:   source.optionalLine(),
			}
		}
		switch source.below(4) {
		case 1:
			comment.Resolution = wireResolution{State: resolutionEmpty}
		case 2:
			comment.Resolution = wireResolution{State: resolutionDetailed, CreatedOn: comment.CreatedOn, User: comment.User}
		}
		comments = append(comments, comment)
	}
	return comments
}

func FuzzNormalise(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{5, 1, 0, 0, 0, 0, 0, 0, 0, 1, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 3})
	f.Add([]byte{24, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9})
	f.Add([]byte("a reply cycle, an orphan, a duplicate and an inverted span in one"))

	f.Fuzz(func(t *testing.T, data []byte) {
		given := generated(data)

		actual, _ := normalise(wirePR(), given)

		// every distinct input identifier appears exactly once.
		expected := map[int]bool{}
		for _, comment := range given {
			expected[comment.ID] = true
		}
		seen := map[int]int{}
		for _, comment := range feedback.Comments(actual) {
			seen[comment.ID]++
		}
		if len(seen) != len(expected) {
			t.Fatalf("%d distinct comments in, %d out", len(expected), len(seen))
		}
		for id, n := range seen {
			if n != 1 || !expected[id] {
				t.Fatalf("comment %d appears %d times", id, n)
			}
		}

		// every thread sits in the collection its location names.
		for _, thread := range actual.Discussion {
			if thread.Location.Kind != feedback.LocationPullRequest {
				t.Fatalf("a %s thread is in discussion", thread.Location.Kind)
			}
		}
		for _, thread := range actual.FileFeedback {
			if thread.Location.Kind != feedback.LocationFileRevision {
				t.Fatalf("a %s thread is in file feedback", thread.Location.Kind)
			}
		}

		// replies are ordered by creation time, then identifier.
		var ordered func(comment feedback.Comment)
		ordered = func(comment feedback.Comment) {
			replies := comment.Replies
			if !sort.SliceIsSorted(replies, func(i, j int) bool {
				a, _ := feedback.ParseTimestamp(replies[i].CreatedAt)
				b, _ := feedback.ParseTimestamp(replies[j].CreatedAt)
				if !a.Equal(b) {
					return a.Before(b)
				}
				return replies[i].ID < replies[j].ID
			}) {
				t.Fatalf("replies of %d are out of order: %v", comment.ID, ids(replies))
			}
			for _, reply := range replies {
				ordered(reply)
			}
		}
		for _, thread := range actual.Threads() {
			ordered(thread.Root)
		}

		// the result is a valid import document, and survives an encode round trip.
		encoded, err := feedback.Encode(actual)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := feedback.Decode(encoded)
		if err != nil {
			t.Fatalf("normalised output is not a valid import document: %v\n%s", err, encoded)
		}
		reencoded, err := feedback.Encode(*decoded)
		if err != nil || string(reencoded) != string(encoded) {
			t.Fatalf("encode then decode is not the identity")
		}

		// normalisation is deterministic.
		again, _ := normalise(wirePR(), given)
		if !reflect.DeepEqual(again, actual) {
			t.Fatal("normalising the same input twice differs")
		}
	})
}
