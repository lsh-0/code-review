package main

import (
	"code-review/feedback"
	"code-review/model"
	"os"
	"strings"
	"testing"
)

// fuzz and property tests over the import path.
//
// The decoder is the tool's only reader of a file it did not write, so it is the one
// place a malformed input reaches the program. These assert the two things the unit
// tests cannot: that no input crashes it, and that the mapping's invariants hold for
// inputs nobody thought to write down.
//
// Run the fuzzers beyond their seed corpus with:
//
//	go test -run xxx -fuzz FuzzDecodeForeignDocument -fuzztime 60s
//
// A plain `go test` runs the seeds only, which keeps the suite fast.

// no input may panic the decoder, and a successful decode must satisfy the invariants
// the rest of the import relies on: every thread carries a recognised location tag,
// every file-anchored thread has a path, and every span it accepted is well formed.
//
// A decode error is a valid outcome for arbitrary bytes; a panic never is.
func FuzzDecodeForeignDocument(f *testing.F) {
	// the real document, so the fuzzer starts from a structurally valid input and
	// mutates outward rather than exploring from noise.
	if data, err := os.ReadFile(foreignFixture); err == nil {
		f.Add(data)
	}

	f.Add([]byte(`{}`))
	f.Add([]byte(`[]`))
	f.Add([]byte(``))
	f.Add([]byte(`null`))
	f.Add([]byte(documentWith(fileThread("a.go", feedback.PositionUpdated, 1, 1))))
	f.Add([]byte(documentWith(fileThreadRaw("a.go", `{"kind": "whole_file"}`))))
	f.Add([]byte(documentWith(fileThreadRaw("a.go", `{"kind": "original", "lines": {"first": 1, "last": 1}}`))))
	// deeply nested replies: the recursive decode and the recursive validation.
	f.Add([]byte(documentWith(threadJSON("a.go", `{"kind": "updated", "lines": {"first": 1, "last": 1}}`,
		commentJSON(1, markdownBody("a"), false, []string{
			commentJSON(2, markdownBody("b"), false, []string{
				commentJSON(3, markdownBody("c"), false, nil),
			}),
		})))))

	f.Fuzz(func(t *testing.T, data []byte) {
		doc, err := feedback.Decode(data)
		if err != nil {
			// arbitrary bytes are usually not a document. Rejection is correct.
			return
		}
		if doc == nil {
			t.Fatal("a successful decode returned a nil document")
		}

		if doc.PullRequest.ID == 0 {
			t.Error("an accepted document has no pull request id")
		}

		for _, thread := range doc.Threads() {
			switch thread.Location.Kind {
			case feedback.LocationPullRequest:
			case feedback.LocationFileRevision:
				if thread.Location.Path == "" {
					t.Error("an accepted file-anchored thread has no path")
				}
				assertAcceptedPosition(t, thread.Location.Position)
			default:
				t.Errorf("an accepted thread carries an unrecognised location kind %q", thread.Location.Kind)
			}

			switch thread.Resolution.State {
			case feedback.ResolutionUnresolved, feedback.ResolutionResolved, feedback.ResolutionResolvedWithoutInfo:
			default:
				t.Errorf("an accepted thread carries an unrecognised resolution state %q", thread.Resolution.State)
			}
		}
	})
}

// every span an accepted position exposes must be well formed, so no downstream caller
// has to re-check what the decoder claims to have validated.
func assertAcceptedPosition(t *testing.T, position feedback.Position) {
	t.Helper()
	switch position.Kind {
	case feedback.PositionWholeFile:
	case feedback.PositionOriginal, feedback.PositionUpdated:
		if !position.Lines.Valid() {
			t.Errorf("an accepted %s position has a malformed span %d-%d",
				position.Kind, position.Lines.First, position.Lines.Last)
		}
	case feedback.PositionUnchanged:
		if !position.OriginalLines.Valid() {
			t.Errorf("an accepted unchanged position has a malformed original span %d-%d",
				position.OriginalLines.First, position.OriginalLines.Last)
		}
		if !position.UpdatedLines.Valid() {
			t.Errorf("an accepted unchanged position has a malformed updated span %d-%d",
				position.UpdatedLines.First, position.UpdatedLines.Last)
		}
	default:
		t.Errorf("an accepted position carries an unrecognised kind %q", position.Kind)
	}
}

// the mapping must hold its invariants for any document the decoder accepts, not only
// the ones a unit test names. Mapping runs against empty placement inputs, which is
// the harshest case: every comment goes adrift and nothing is filtered out by a
// missing path.
func FuzzMapForeignDocument(f *testing.F) {
	if data, err := os.ReadFile(foreignFixture); err == nil {
		f.Add(data)
	}
	f.Add([]byte(documentWith(fileThread("a.go", feedback.PositionUpdated, 1, 1))))
	f.Add([]byte(documentWith(threadJSON("a.go", `{"kind": "updated", "lines": {"first": 1, "last": 1}}`,
		commentJSON(1, markdownBody("a"), false, []string{
			commentJSON(2, markdownBody("b"), false, nil),
			commentJSON(3, `{"kind": "deleted"}`, false, nil),
		})))))

	f.Fuzz(func(t *testing.T, data []byte) {
		doc, err := feedback.Decode(data)
		if err != nil {
			return
		}

		mapped, report := mapForeignDocument(doc, map[string][]model.DiffLineRef{}, map[string]string{})

		assertMappingInvariants(t, mapped, report)
	})
}

// the invariants every mapping result must satisfy, whatever the document said.
func assertMappingInvariants(t *testing.T, mapped []mappedComment, report importReport) {
	t.Helper()

	if report.Imported != len(mapped) {
		t.Errorf("report claims %d imported, mapping produced %d", report.Imported, len(mapped))
	}

	roots := make(map[string]bool)
	replies := make(map[string]bool)
	ids := make(map[string]bool)

	for _, entry := range mapped {
		comment := entry.Comment

		// provenance: every imported comment must name its source, or the merge
		// cannot match it and the immutability guards cannot recognise it.
		if !comment.IsForeign() {
			t.Error("a mapped comment carries no origin")
		}
		if comment.Origin == nil || comment.Origin.ForeignID == "" {
			t.Error("a mapped comment carries no foreign id")
		}

		// identity: ids must be unique, or a mutation would hit the wrong comment.
		if ids[comment.ID] {
			t.Errorf("duplicate comment id %q", comment.ID)
		}
		ids[comment.ID] = true

		if comment.ParentID == "" {
			roots[comment.ID] = true
		} else {
			replies[comment.ID] = true
		}

		// a review-level comment carries no anchor; a file-anchored root always
		// carries exactly one.
		if entry.FilePath == "" {
			if len(comment.Anchors) != 0 {
				t.Errorf("a review-level comment carries %d anchors", len(comment.Anchors))
			}
		} else if comment.ParentID == "" && len(comment.Anchors) != 1 {
			t.Errorf("a file-anchored root carries %d anchors, want 1", len(comment.Anchors))
		}

		// a reply never carries an anchor of its own, in any surface.
		if comment.ParentID != "" && len(comment.Anchors) != 0 {
			t.Errorf("a reply carries %d anchors, want 0", len(comment.Anchors))
		}

		// content: a skipped comment must not have been mapped at all, so no mapped
		// comment may have empty content.
		if comment.Content == "" {
			t.Error("a mapped comment carries no content")
		}
		if comment.Author == "" {
			t.Error("a mapped comment carries no author")
		}
	}

	// the flattening invariant: threads are one level deep, so no comment may be
	// parented to a reply, and every reply's parent must be a mapped root.
	for _, entry := range mapped {
		parent := entry.Comment.ParentID
		if parent == "" {
			continue
		}
		if replies[parent] {
			t.Errorf("comment %q is parented to a reply", entry.Comment.ID)
		}
		if !roots[parent] {
			t.Errorf("comment %q names a parent that is not a mapped root", entry.Comment.ID)
		}
	}

	// the report must partition the file-anchored roots it counted.
	fileRoots := 0
	for _, entry := range mapped {
		if entry.FilePath != "" && entry.Comment.ParentID == "" {
			fileRoots++
		}
	}
	if report.Placed+report.Untethered != fileRoots {
		t.Errorf("placed (%d) + untethered (%d) != file-anchored roots (%d)",
			report.Placed, report.Untethered, fileRoots)
	}
	if report.Replies != len(replies) {
		t.Errorf("report claims %d replies, mapping produced %d", report.Replies, len(replies))
	}

	// with no diff lines supplied nothing can be placed, and every unplaceable path
	// must be one the report also lists as unmatched.
	if report.Placed != 0 {
		t.Errorf("nothing can be placed against an empty diff, report claims %d", report.Placed)
	}
	for _, path := range report.UnplaceablePaths {
		if path == "" {
			t.Error("an unplaceable entry names an empty path")
		}
	}

	// every reported path list must be sorted and free of duplicates, so a report is
	// deterministic for a given document.
	assertSortedUnique(t, "unmatched", report.UnmatchedPaths)
	assertSortedUnique(t, "unplaceable", report.UnplaceablePaths)
}

func assertSortedUnique(t *testing.T, label string, paths []string) {
	t.Helper()
	for i := 1; i < len(paths); i++ {
		if paths[i-1] >= paths[i] {
			t.Errorf("%s paths are not sorted and unique: %v", label, paths)
			return
		}
	}
}

// merging a document into a review must be idempotent: importing the same document
// any number of times leaves the same comments, so a reviewer who re-imports by habit
// does not accumulate duplicates.
func FuzzMergeIsIdempotent(f *testing.F) {
	if data, err := os.ReadFile(foreignFixture); err == nil {
		f.Add(data)
	}
	f.Add([]byte(documentWith(fileThread("a.go", feedback.PositionUpdated, 1, 1))))
	f.Add([]byte(documentWith(threadJSON("a.go", `{"kind": "updated", "lines": {"first": 1, "last": 1}}`,
		commentJSON(1, markdownBody("a"), false, []string{commentJSON(2, markdownBody("b"), false, nil)})))))

	f.Fuzz(func(t *testing.T, data []byte) {
		doc, err := feedback.Decode(data)
		if err != nil {
			return
		}

		lines := map[string][]model.DiffLineRef{"a.go": diffLinesFor(1, 40)}
		blobs := map[string]string{"a.go": "sha"}
		source := foreignSource(doc, "t1")

		review := model.NewReview("/repo", "feature", "master")

		// a local comment and a local review-level comment, which no import may touch.
		localFile := review.AddFileDiff("local-only.go")
		localComment := localFile.AddComment("my own note", 4, "Test User")
		review.AddComment("my own overall note", "Test User")

		first, _ := mapForeignDocument(doc, lines, blobs)
		mergeForeignComments(review, source, first)
		afterFirst := commentFingerprints(review)

		second, _ := mapForeignDocument(doc, lines, blobs)
		mergeForeignComments(review, source, second)
		afterSecond := commentFingerprints(review)

		if len(afterFirst) != len(afterSecond) {
			t.Errorf("re-import changed the comment count: %d then %d", len(afterFirst), len(afterSecond))
		}
		for key, count := range afterFirst {
			if afterSecond[key] != count {
				t.Errorf("re-import changed comment %q: seen %d then %d", key, count, afterSecond[key])
			}
		}

		// the local comments must be untouched by any import.
		if localFile := review.GetFileDiff("local-only.go"); localFile == nil {
			t.Error("the local-only file was removed by an import")
		} else if found := localFile.GetComment(localComment.ID); found == nil {
			t.Error("the local comment was removed by an import")
		} else if found.Content != "my own note" || found.IsForeign() {
			t.Error("the local comment was altered by an import")
		}

		localReviewLevel := 0
		for _, comment := range review.Comments {
			if !comment.IsForeign() {
				localReviewLevel++
			}
		}
		if localReviewLevel != 1 {
			t.Errorf("local review-level comments: got %d, want 1", localReviewLevel)
		}

		// exactly one registry entry per source, however many times it is imported.
		if len(review.Sources) != 1 {
			t.Errorf("sources after two imports: got %d, want 1", len(review.Sources))
		}
	})
}

// a stable multiset key per comment, ignoring the generated id: the surface it sits
// on, its provenance, and its content. Two merges that produce the same multiset have
// produced the same review, whatever ids were minted along the way.
func commentFingerprints(review *model.Review) map[string]int {
	counts := make(map[string]int)
	add := func(surface string, comment *model.Comment) {
		parent := "root"
		if comment.ParentID != "" {
			parent = "reply"
		}
		foreignID := ""
		if comment.Origin != nil {
			foreignID = comment.Origin.ForeignID
		}
		key := strings.Join([]string{
			surface, parent, comment.SourceID(), foreignID, comment.Content, string(comment.Status),
		}, "\x00")
		counts[key]++
	}

	for _, file := range review.Files {
		for _, comment := range file.Comments {
			add(file.FilePath, comment)
		}
	}
	for _, comment := range review.Comments {
		add("", comment)
	}
	return counts
}
