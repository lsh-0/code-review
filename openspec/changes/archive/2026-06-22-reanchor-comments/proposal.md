## Why

A comment is anchored only by a new-side `LineNumber`, captured against whatever diff happened to be on screen, with no record of which file content it was anchored against (`backend/model/model.go:21-31`). When the file changes during review — a new commit lands, or an AI agent edits the file — the stored line number no longer points at the same code. The comment silently slides to a different line, lands on unrelated code, or vanishes when its line falls outside every hunk. The reviewer has no signal that this happened and no way to recover the comment's original meaning.

## What Changes

- **Replace the single line-number anchor with a history of anchors, one per distinct file content** the comment has been viewed against. Each anchor records the file's git blob SHA, a placement (line number), and a captured context window from that content. `Anchors[0]` — the anchor at creation time — is guaranteed good: the reviewer was looking at a live hunk when they wrote the comment.
- **Capture a wider context window** at each successful anchoring (more than today's single before/line/after) so the heuristic has material to match on.
- **Re-anchor comments when the file content changes.** Keyed on blob SHA, so commits that don't touch the file add no anchor. When a comment's file presents a new blob, the heuristic runs exact-then-fuzzy against the **last good anchor's** context (the most recent anchor that has a captured context); success appends a new good anchor (line + context), failure appends an **adrift** anchor (blob SHA only, no line, no context).
- **"Outdated" is a derived state, not a stored status.** A comment is outdated iff its most-recent anchor is adrift. This is orthogonal to `Status` (active/resolved/ignored), which stays reviewer-controlled — a comment can be both open and outdated. The current line number is read from the most-recent anchor; if it has none, the comment is adrift/orphaned.
- **Recovery is allowed.** A later blob change (e.g. a revert that restores the original code) re-runs the heuristic from the last good context and can re-anchor a previously adrift comment, clearing its outdated state. A blob already seen with a good placement is reused rather than re-matched.
- **Render outdated comments untethered.** An outdated comment is displayed at the top of its file's view, showing the context from its most recent **good** anchor as a read-only pseudo-hunk with a warning (yellow) border. It cannot expand lines (there is no live hunk to expand into).
- **Deleting an outdated comment deletes its captured context** along with it; there is no other owner of that frozen context.
- **Legacy comments upgrade on load.** Existing comments (line number + 3-line context, no anchor list, no blob) are upgraded in place to a synthesised `Anchors[0]` with an empty/unknown blob — mirroring the legacy bare-path mark convention (`model.go:55-56,67-69`). Old state files load without a separate migration step; the first recompute backfills a real blob-keyed anchor.

## Capabilities

### New Capabilities
- `comment-reanchoring`: Maintaining a per-comment history of blob-keyed anchors (blob SHA + line + captured context), re-anchoring on file-content change via exact-then-fuzzy heuristics against the last good anchor, deriving outdated state when no placement is found, recovering when content returns, and rendering/deleting outdated comments untethered from any live hunk. Includes load-time upgrade of legacy comments.

### Modified Capabilities
- `review-state-sync`: `RecomputeDiff` gains a re-anchoring responsibility — after the new diff is built, each comment is re-anchored against its file's current blob (or left unchanged if the blob is unchanged), and the affected results are reported back to the frontend rather than assuming line numbers are stable.

## Impact

- **Data model**: `model.Comment` (`backend/model/model.go:21-31`) gains an `Anchors` list of a new `Anchor` type (blob SHA, line number, captured context); the standalone `LineNumber`/`Context*` fields are subsumed by `Anchors[0]` (with load-time upgrade). Mirrored in `ts/core/types.ts:13-23`. Outdated/current-line are computed accessors, not stored fields.
- **Backend**: comment creation (`main.go:480-489`, `AddCommentWithContext`) captures `Anchors[0]` including the file's blob SHA; a new re-anchoring routine runs the heuristic; `RecomputeDiff` (per `review-state-sync` spec) drives it; persistence (`backend/storage.go`) round-trips anchors with backward-compatible loading (`UnmarshalJSON`-style upgrade like `MarkedFiles`).
- **Frontend**: anchoring/lookup (`ts/core/comments.ts` line-number lookup now reads the current anchor; `ts/core/overview.ts:getLineContext` widens the captured window), rendering (`ts/render/comments.ts`, `ts/render/hunks.ts`, `ts/render/diff.ts`, `ts/render/mutate.ts`) — a new untethered/outdated render path plus the captured-context pseudo-hunk.
- **CSS**: warning-bordered outdated comment styling (`backend/assets/style.css`, comment section ~690-799).
- **Relationship to marks**: file-content change is the shared trigger for both unmarking a file (`durable-file-marks`) and re-anchoring its comments, but the two remain independent consumers of the same blob-SHA signal — marks are reviewer sign-off, anchors are system-derived placement.
- **Compatibility**: existing review JSON files (no anchor list, 3-line context, no blob) load via in-place upgrade and behave as before until the first recompute.
