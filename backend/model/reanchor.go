package model

import "strings"

// a new-side diff line the reconciler can match against: its raw content and the
// new-file line number it sits at. The caller flattens the parsed diff into
// these (one per non-removed line) so the model stays decoupled from the diff
// parser. A matched window's anchored line reports the `LineNumber` of the ref
// at its offset.
type DiffLineRef struct {
	Content    string
	LineNumber int
}

// the Jaccard-similarity floor a fuzzy candidate window must reach to be
// accepted as a re-anchor. Below it the comment is left adrift rather than
// placed on uncertain code. A single knob: raise it to demand closer matches
// (more comments go outdated), lower it to place more aggressively (more risk
// of a wrong placement).
const fuzzyMatchThreshold = 0.7

// re-anchor every comment against its file's current content. `currentBlobs`
// maps file path to the file's current git blob SHA; `diffLines` maps file path
// to the new-side line contents present in the recomputed diff. For each
// commented file the comments are reconciled in place against that file's
// current blob and diff lines.
//
// Pure: it mutates only the comments reachable from the review, deriving every
// decision from its inputs, so it is testable without git. A file absent from
// `currentBlobs` (deleted, or carrying no comments) is skipped.
func (r *Review) ReanchorComments(currentBlobs map[string]string, diffLines map[string][]DiffLineRef) {
	for _, file := range r.Files {
		blob, present := currentBlobs[file.FilePath]
		if !present {
			continue
		}
		lines := diffLines[file.FilePath]
		for _, comment := range file.Comments {
			reanchorComment(comment, blob, lines)
		}
	}
}

// reconcile a single comment against its file's current `blob` and the new-side
// `lines` present in the diff, following the strict short-circuit order: an
// unanchored comment (reply, review-level) is left alone; an unchanged blob is a
// no-op; a blob already in history is reused; otherwise the last good context is
// matched (exact, then fuzzy) and a good or adrift anchor is appended.
func reanchorComment(comment *Comment, blob string, lines []DiffLineRef) {
	current := comment.currentAnchor()
	if current == nil {
		return // reply or review-level comment: no anchor to reconcile.
	}

	// an unknown current blob (the file's SHA could not be resolved this pass)
	// is not a real content version: leave the comment untouched rather than
	// adopt "" as a baseline or match against it.
	if blob == "" {
		return
	}

	// legacy first anchor (empty blob, awaiting backfill): adopt the current
	// blob as its baseline without re-matching, so a legacy comment keeps its
	// placement until the file actually changes.
	if current.Blob == "" {
		current.Blob = blob
		return
	}

	// unchanged content for this comment: nothing to do.
	if current.Blob == blob {
		return
	}

	// reuse: this exact content was reconciled before. Recover (or stay adrift)
	// from the stored anchor without any matching.
	if reused := findAnchorByBlob(comment.Anchors, blob); reused != nil {
		comment.Anchors = append(comment.Anchors, *reused)
		return
	}

	// genuinely new content: use the last good context only to *locate* the line
	// in the new diff. The new anchor then captures a fresh window from the new
	// content at that position — the context witnesses the blob it belongs to, so
	// the next reconcile matches against the latest content, not the original.
	good := comment.lastGoodAnchor()
	if good != nil {
		if anchored, ok := placeContext(good.Context, good.Offset, lines); ok {
			context, offset := captureWindow(lines, anchored)
			comment.Anchors = append(comment.Anchors, Anchor{
				Blob:       blob,
				LineNumber: lines[anchored].LineNumber,
				Offset:     offset,
				Context:    context,
			})
			return
		}
	}

	// lost: append an adrift anchor carrying only the new blob.
	comment.Anchors = append(comment.Anchors, Anchor{Blob: blob})
}

// the first anchor in `anchors` whose blob equals `blob`, or nil. Used to reuse
// a placement when content the comment was seen against returns.
func findAnchorByBlob(anchors []Anchor, blob string) *Anchor {
	for i := range anchors {
		if anchors[i].Blob == blob {
			return &anchors[i]
		}
	}
	return nil
}

// the radius of the context window captured around an anchored line: this many
// lines each side, so a fresh window spans up to 2*captureRadius+1 lines.
//
// MUST stay equal to the frontend's `captureContextRadius` (ts/core/overview.ts):
// the frontend captures the window at comment-creation time and this re-captures
// it on re-anchor, so a re-anchored window must be the same size as a freshly
// created one. The two constants are independent; keep them in lockstep.
const captureRadius = 3

// locate the anchored line within `lines` and return its index there. `context`
// is the search key (the last good window) and `offset` is the anchored line's
// index within it. Tries an exact window match first, then a fuzzy match
// accepted only at or above `fuzzyMatchThreshold`. Reports false when neither
// succeeds or the inputs are degenerate. The returned index is into `lines`, so
// the caller reads `lines[idx]` for the line number and `captureWindow` for a
// fresh context window.
func placeContext(context []string, offset int, lines []DiffLineRef) (int, bool) {
	if len(context) == 0 || len(lines) == 0 || offset < 0 || offset >= len(context) {
		return 0, false
	}

	if start, ok := exactWindow(context, lines); ok {
		return start + offset, true
	}
	if start, ok := fuzzyWindow(context, lines); ok {
		return start + offset, true
	}
	return 0, false
}

// a fresh context window from `lines` centred on the anchored line at index
// `anchored`, spanning `captureRadius` lines each side and clipped at the slice
// edges. Returns the window's line contents and the anchored line's offset
// within it (`captureRadius` for an interior line, smaller when clipped at the
// start). This is what a re-anchored comment stores, so its context witnesses
// the new content rather than carrying the old window forward.
func captureWindow(lines []DiffLineRef, anchored int) ([]string, int) {
	lo := max(anchored-captureRadius, 0)
	hi := min(anchored+captureRadius, len(lines)-1)
	window := make([]string, 0, hi-lo+1)
	for i := lo; i <= hi; i++ {
		window = append(window, lines[i].Content)
	}
	return window, anchored - lo
}

// the start index of the first window in `lines` matching `context` verbatim, or
// false. Windows are compared on trimmed line content.
func exactWindow(context []string, lines []DiffLineRef) (int, bool) {
	width := len(context)
	for start := 0; start+width <= len(lines); start++ {
		if windowEquals(context, lines[start:start+width]) {
			return start, true
		}
	}
	return 0, false
}

// the start index of the window in `lines` most similar to `context` by Jaccard
// overlap of trimmed-line sets, provided the best score reaches
// `fuzzyMatchThreshold`. Reports false when no window clears the threshold.
func fuzzyWindow(context []string, lines []DiffLineRef) (int, bool) {
	width := len(context)
	want := trimmedSet(context)
	best := 0.0
	bestStart := -1
	for start := 0; start+width <= len(lines); start++ {
		score := jaccard(want, trimmedRefSet(lines[start:start+width]))
		if score > best {
			best = score
			bestStart = start
		}
	}
	if bestStart >= 0 && best >= fuzzyMatchThreshold {
		return bestStart, true
	}
	return 0, false
}

// report whether a context window equals a diff-line window line-for-line on
// trimmed content.
func windowEquals(context []string, lines []DiffLineRef) bool {
	if len(context) != len(lines) {
		return false
	}
	for i := range context {
		if strings.TrimSpace(context[i]) != strings.TrimSpace(lines[i].Content) {
			return false
		}
	}
	return true
}

// the set of non-empty trimmed lines in `window`. Blank lines are dropped so
// they do not inflate similarity between unrelated windows.
func trimmedSet(window []string) map[string]struct{} {
	set := make(map[string]struct{}, len(window))
	for _, line := range window {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			set[trimmed] = struct{}{}
		}
	}
	return set
}

// the set of non-empty trimmed contents of a diff-line window.
func trimmedRefSet(window []DiffLineRef) map[string]struct{} {
	set := make(map[string]struct{}, len(window))
	for _, line := range window {
		trimmed := strings.TrimSpace(line.Content)
		if trimmed != "" {
			set[trimmed] = struct{}{}
		}
	}
	return set
}

// the Jaccard similarity (|intersection| / |union|) of two string sets. Two
// empty sets are treated as fully dissimilar (0), so an all-blank window never
// matches.
func jaccard(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	intersection := 0
	for k := range a {
		if _, ok := b[k]; ok {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}
