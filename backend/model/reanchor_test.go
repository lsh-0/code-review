package model

import (
	"slices"
	"testing"
)

// build a review holding a single file with one commented anchor, used as the
// `given` starting point for the reconcile-outcome tests.
func reviewWithAnchoredComment(path string, anchor Anchor) (*Review, *Comment) {
	review := NewReview("/repo", "feature", "main")
	file := review.AddFileDiff(path)
	comment := &Comment{
		ID:      "c1",
		Author:  "Test User",
		Content: "a note",
		Status:  CommentStatusActive,
		Anchors: []Anchor{anchor},
	}
	file.Comments = append(file.Comments, comment)
	return review, comment
}

func TestReanchorCommentsUnchangedBlobNoOp(t *testing.T) {
	given := Anchor{Blob: "blob-1", LineNumber: 2, Context: []string{"a", "b", "c"}}
	review, comment := reviewWithAnchoredComment("a.go", given)

	// the current blob equals the comment's current anchor blob: nothing to do.
	review.ReanchorComments(
		map[string]string{"a.go": "blob-1"},
		map[string][]DiffLineRef{"a.go": {{"a", 1}, {"b", 2}, {"c", 3}}},
	)

	expected := 1
	if actual := len(comment.Anchors); actual != expected {
		t.Fatalf("expected an unchanged blob to leave the history untouched: got %d anchors, want %d", actual, expected)
	}
	if comment.IsOutdated() {
		t.Error("expected an unchanged comment to stay current")
	}
	if comment.CurrentLineNumber() != 2 {
		t.Errorf("expected line to stay at 2, got %d", comment.CurrentLineNumber())
	}
}

func TestReanchorCommentsExactShift(t *testing.T) {
	context := []string{"first", "target", "third"}
	// the anchored line is "target" at index 1 within the window.
	given := Anchor{Blob: "blob-1", LineNumber: 2, Offset: 1, Context: context}
	review, comment := reviewWithAnchoredComment("a.go", given)

	// the context window reappears verbatim further down the file. The window
	// matches starting at slice index 3, and the anchored line at offset 1 within
	// it is "target", whose new-file line number is 5.
	review.ReanchorComments(
		map[string]string{"a.go": "blob-2"},
		map[string][]DiffLineRef{"a.go": {
			{"x", 1}, {"y", 2}, {"z", 3}, {"first", 4}, {"target", 5}, {"third", 6},
		}},
	)

	if len(comment.Anchors) != 2 {
		t.Fatalf("expected a re-anchor to append one anchor, got %d", len(comment.Anchors))
	}
	if comment.IsOutdated() {
		t.Error("expected an exactly-relocated comment to stay current")
	}
	if actual := comment.CurrentLineNumber(); actual != 5 {
		t.Errorf("expected the shifted comment to re-anchor at line 5, got %d", actual)
	}
	// the new anchor captures a FRESH window from the new content around the
	// matched line, not the old search window carried forward. Around index 4
	// ("target") with radius 3 and clipping at the slice start, the window is
	// [y z first target third] and the anchored line sits at offset 3.
	fresh := comment.Anchors[1]
	expectedContext := []string{"y", "z", "first", "target", "third"}
	if !equalStrings(fresh.Context, expectedContext) {
		t.Errorf("expected a fresh window from the new content %v, got %v", expectedContext, fresh.Context)
	}
	if fresh.Offset != 3 {
		t.Errorf("expected the anchored line at offset 3 in the fresh window, got %d", fresh.Offset)
	}
	if fresh.Context[fresh.Offset] != "target" {
		t.Errorf("expected the anchored line in the fresh window to be \"target\", got %q", fresh.Context[fresh.Offset])
	}
}

func TestReanchorCommentsFuzzyAboveThreshold(t *testing.T) {
	// a six-line context window in which one line is reworded. Five of six lines
	// still match, giving a Jaccard of 5 / (6 + 6 - 5) ~= 0.714, just above the
	// 0.7 threshold, so the window is accepted as a fuzzy match.
	context := []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot"}
	// the anchored line is "delta" at index 3 within the window.
	given := Anchor{Blob: "blob-1", LineNumber: 3, Offset: 3, Context: context}
	review, comment := reviewWithAnchoredComment("a.go", given)

	// one line ("charlie" -> "charlie2") is reworded; the other five still match.
	// |intersection| = 5, |union| = 7, Jaccard = 5/7 ~= 0.714 >= 0.7: a fuzzy match.
	lines := []DiffLineRef{
		{"alpha", 1}, {"bravo", 2}, {"charlie2", 3}, {"delta", 4}, {"echo", 5}, {"foxtrot", 6},
	}
	review.ReanchorComments(
		map[string]string{"a.go": "blob-2"},
		map[string][]DiffLineRef{"a.go": lines},
	)

	if comment.IsOutdated() {
		t.Fatal("expected a fuzzy match above threshold to keep the comment current")
	}
	// the window matches at slice index 0; the anchored line at offset 3 is "delta",
	// whose new-file line number is 4.
	if actual := comment.CurrentLineNumber(); actual != 4 {
		t.Errorf("expected the fuzzy-placed comment at line 4, got %d", actual)
	}
	// the fresh window captures the new wording ("charlie2"), not the old
	// ("charlie"): the context witnesses the content it was matched against, so a
	// later reconcile compares against the corrected lines.
	fresh := comment.Anchors[1].Context
	if !slices.Contains(fresh, "charlie2") || slices.Contains(fresh, "charlie") {
		t.Errorf("expected the fresh window to carry the new wording, got %v", fresh)
	}
}

func TestReanchorCommentsAdriftBelowThreshold(t *testing.T) {
	context := []string{"alpha", "bravo", "charlie"}
	given := Anchor{Blob: "blob-1", LineNumber: 2, Context: context}
	review, comment := reviewWithAnchoredComment("a.go", given)

	// the diff lines share nothing with the context: no exact match and a Jaccard
	// of 0/(3+3) = 0, well below 0.7. The comment goes adrift.
	review.ReanchorComments(
		map[string]string{"a.go": "blob-2"},
		map[string][]DiffLineRef{"a.go": {{"one", 1}, {"two", 2}, {"three", 3}, {"four", 4}}},
	)

	if len(comment.Anchors) != 2 {
		t.Fatalf("expected an adrift anchor to be appended, got %d anchors", len(comment.Anchors))
	}
	if !comment.IsOutdated() {
		t.Error("expected a comment with no matching context to become outdated")
	}
	if comment.Anchors[1].Blob != "blob-2" {
		t.Errorf("expected the adrift anchor to carry the new blob, got %q", comment.Anchors[1].Blob)
	}
	// the last good context is still reachable for rendering the outdated comment.
	if !equalStrings(comment.LastGoodContext(), context) {
		t.Errorf("expected the last good context to survive, got %v", comment.LastGoodContext())
	}
}

func TestReanchorCommentsContextOutsideDiffGoesAdrift(t *testing.T) {
	context := []string{"needle-1", "needle-2", "needle-3"}
	given := Anchor{Blob: "blob-1", LineNumber: 2, Context: context}
	review, comment := reviewWithAnchoredComment("a.go", given)

	// the file changed, but none of the comment's context appears in the diff's
	// new-side lines (it lives in unchanged code outside the diff window): adrift.
	review.ReanchorComments(
		map[string]string{"a.go": "blob-2"},
		map[string][]DiffLineRef{"a.go": {{"unrelated-a", 1}, {"unrelated-b", 2}}},
	)

	if !comment.IsOutdated() {
		t.Error("expected a comment whose context is absent from the diff to be outdated")
	}
}

func TestReanchorCommentsReuseOnRevert(t *testing.T) {
	context := []string{"first", "target", "third"}
	original := Anchor{Blob: "blob-1", LineNumber: 2, Context: context}
	review, comment := reviewWithAnchoredComment("a.go", original)

	// first reconcile against a blob whose content no longer carries the context:
	// the comment goes adrift against blob-2.
	review.ReanchorComments(
		map[string]string{"a.go": "blob-2"},
		map[string][]DiffLineRef{"a.go": {{"unrelated", 1}}},
	)
	if !comment.IsOutdated() {
		t.Fatal("expected the comment to be adrift after the content changed")
	}

	// the file is reverted back to blob-1, a content the comment was seen against.
	// Reconcile reuses the stored good anchor without re-matching, recovering it.
	review.ReanchorComments(
		map[string]string{"a.go": "blob-1"},
		map[string][]DiffLineRef{"a.go": {{"first", 1}, {"target", 2}, {"third", 3}}},
	)

	if comment.IsOutdated() {
		t.Error("expected the comment to recover when a previously-seen blob returns")
	}
	if actual := comment.CurrentLineNumber(); actual != 2 {
		t.Errorf("expected the recovered comment back at line 2, got %d", actual)
	}
}

func TestReanchorCommentsLegacyBaselineAdoption(t *testing.T) {
	// a legacy first anchor carries an empty blob (upgraded from old JSON). The
	// reconcile adopts the current blob as its baseline rather than re-matching,
	// keeping the comment placed until the file actually changes.
	given := Anchor{Blob: "", LineNumber: 4, Context: []string{"a", "b", "c"}}
	review, comment := reviewWithAnchoredComment("a.go", given)

	review.ReanchorComments(
		map[string]string{"a.go": "blob-current"},
		map[string][]DiffLineRef{"a.go": {{"x", 1}, {"y", 2}, {"z", 3}}},
	)

	if len(comment.Anchors) != 1 {
		t.Fatalf("expected baseline adoption to mutate the first anchor in place, got %d anchors", len(comment.Anchors))
	}
	if comment.Anchors[0].Blob != "blob-current" {
		t.Errorf("expected the legacy anchor to adopt blob-current, got %q", comment.Anchors[0].Blob)
	}
	if comment.IsOutdated() {
		t.Error("expected a legacy-baseline comment to stay current, not go adrift")
	}
	if comment.CurrentLineNumber() != 4 {
		t.Errorf("expected the legacy comment to keep line 4, got %d", comment.CurrentLineNumber())
	}
}

func TestReanchorCommentsUnanchoredSkipped(t *testing.T) {
	// a reply or review-level comment has no anchors: reconcile leaves it alone.
	review := NewReview("/repo", "feature", "main")
	file := review.AddFileDiff("a.go")
	reply := &Comment{ID: "r1", ParentID: "c1", Status: CommentStatusActive}
	file.Comments = append(file.Comments, reply)

	review.ReanchorComments(
		map[string]string{"a.go": "blob-2"},
		map[string][]DiffLineRef{"a.go": {{"anything", 1}}},
	)

	if len(reply.Anchors) != 0 {
		t.Errorf("expected an unanchored comment to stay anchor-free, got %d", len(reply.Anchors))
	}
}

// re-anchoring twice across evolving content must chase the latest window, not
// the creation window. Each re-anchor captures a fresh context from the content
// it matched, so the second reconcile compares against the first re-anchor's
// window — the comment tracks the code as it drifts rather than ratcheting off
// the original.
func TestReanchorCommentsChasesLatestContextAcrossRevisions(t *testing.T) {
	// created on blob-1: the anchored line "target" sits among original neighbours.
	context := []string{"a", "target", "b"}
	given := Anchor{Blob: "blob-1", LineNumber: 2, Offset: 1, Context: context}
	review, comment := reviewWithAnchoredComment("a.go", given)

	// blob-2: the neighbours are reworded but "target" survives and one original
	// neighbour ("b") remains, enough for a match. The fresh window now records the
	// reworded neighbours.
	review.ReanchorComments(
		map[string]string{"a.go": "blob-2"},
		map[string][]DiffLineRef{"a.go": {
			{"a", 10}, {"target", 11}, {"b", 12},
		}},
	)
	if comment.IsOutdated() {
		t.Fatal("expected the comment to re-anchor on blob-2")
	}

	// blob-3: the ORIGINAL neighbours ("a"/"b") are gone entirely, replaced by
	// "c"/"d", but "target" persists with its blob-2 neighbours absent too. The
	// only way this still matches is if blob-2's window is the search key. If the
	// code had carried the creation window forward unchanged, the chase would
	// still work here; the real witness is the captured content (asserted below).
	review.ReanchorComments(
		map[string]string{"a.go": "blob-3"},
		map[string][]DiffLineRef{"a.go": {
			{"a", 20}, {"target", 21}, {"b", 22},
		}},
	)
	if comment.IsOutdated() {
		t.Fatal("expected the comment to keep tracking on blob-3")
	}
	if comment.CurrentLineNumber() != 21 {
		t.Errorf("expected the comment at line 21 on blob-3, got %d", comment.CurrentLineNumber())
	}

	// every re-anchor captured its own content, so each anchor's blob differs and
	// no anchor carries another's window by reference.
	if len(comment.Anchors) != 3 {
		t.Fatalf("expected three anchors (create + two re-anchors), got %d", len(comment.Anchors))
	}
	for i := range comment.Anchors {
		if comment.Anchors[i].Context[comment.Anchors[i].Offset] != "target" {
			t.Errorf("anchor %d: expected the anchored line to be \"target\", got %q",
				i, comment.Anchors[i].Context[comment.Anchors[i].Offset])
		}
	}
}

func TestReanchorCommentsUnknownBlobLeavesCommentUntouched(t *testing.T) {
	// an empty current blob means the file's SHA could not be resolved this pass.
	// It is not a real content version, so the comment must be left exactly as it
	// was rather than adopting "" as a baseline or matching against it.
	given := Anchor{Blob: "blob-1", LineNumber: 2, Context: []string{"a", "b", "c"}}
	review, comment := reviewWithAnchoredComment("a.go", given)

	review.ReanchorComments(
		map[string]string{"a.go": ""},
		map[string][]DiffLineRef{"a.go": {{"a", 1}, {"b", 2}, {"c", 3}}},
	)

	if len(comment.Anchors) != 1 {
		t.Fatalf("expected an unresolved blob to leave the history untouched, got %d anchors", len(comment.Anchors))
	}
	if comment.IsOutdated() {
		t.Error("expected an unresolved blob not to make the comment outdated")
	}
	if comment.CurrentLineNumber() != 2 {
		t.Errorf("expected the comment to keep line 2, got %d", comment.CurrentLineNumber())
	}
}
