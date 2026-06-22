## 1. Data model: anchors

- [x] 1.1 Add an `Anchor` type to `backend/model/model.go` with `Blob string`, `LineNumber int`, and `Context []string` (raw line contents), plus JSON tags.
- [x] 1.2 Add `Anchors []Anchor` to `Comment`; keep the legacy `LineNumber`/`ContextBefore`/`ContextLine`/`ContextAfter` fields readable for the upgrade path.
- [x] 1.3 Add derived accessors: `CurrentLineNumber()` (most-recent anchor's line, 0 if adrift), `IsOutdated()` (most-recent anchor has empty context), `LastGoodContext()` (most recent anchor with non-empty context).
- [x] 1.4 Define adrift precisely: an anchor is adrift iff `len(Context) == 0`; ensure accessors agree on this single predicate.
- [x] 1.5 Unit-test the accessors with given/expected/actual cases: good-only history, trailing-adrift history, all-adrift, empty.

## 2. Comment creation captures the first anchor

- [x] 2.1 Widen context capture: update `getLineContext` (`ts/core/overview.ts`) to return a window of N lines each side (not just before/line/after), drawn from the hunk.
- [x] 2.2 Update the `AddComment` backend signature (`backend/main.go:480`) and `NewCommentWithContext` (`backend/model/model.go:105`) to accept the captured window plus the file's current blob SHA, and build `Anchors[0]` from them.
- [x] 2.3 Fetch the file's blob SHA at creation time via the existing `BlobSHAs` helper (`backend/gitquery.go:34`), keyed on the source branch.
- [x] 2.4 Update the Wails binding and the TS call site so the wider context + blob flow through to `AddComment`.
- [x] 2.5 Unit-test that a created comment has exactly one good anchor with the expected blob, line, and context.

## 3. Reconciliation (pure model method)

- [x] 3.1 Add a pure `Review.ReanchorComments(current map[string]string, diff []DiffFile)` to the model, mirroring `EvictChangedMarks` (`backend/model/model.go:359`): inputs are current blob SHAs by path plus the parsed diff; it mutates comment anchors only.
- [x] 3.2 Implement the short-circuit order per comment: (0) if current blob equals most-recent anchor's blob, do nothing; (0b) else if current blob appears anywhere in history, reuse that anchor; (1) else exact-match last good context against the file's diff lines; (2) else fuzzy-match above threshold; (3) else append an adrift anchor.
- [x] 3.3 Implement exact matching: find a position in the file's diff lines where the captured window matches verbatim; anchor the centre line; append a good anchor for the new blob.
- [x] 3.4 Implement fuzzy matching: similarity = Jaccard overlap of trimmed-line sets between the captured window and a candidate window; accept only at or above a single named threshold constant (0.7).
- [x] 3.5 Implement legacy-baseline adoption: a first anchor with an empty blob adopts the current blob as baseline without being marked adrift (mirrors the legacy-mark backfill in `EvictChangedMarks`).
- [x] 3.6 Unit-test reconciliation per outcome: unchanged-blob no-op, reuse-on-revert recovery, exact-shift, fuzzy-above-threshold, adrift-below-threshold, context-outside-diff adrift, legacy-baseline adoption.

## 4. Wire reconciliation into RecomputeDiff

- [x] 4.1 In `RecomputeDiff` (`backend/main.go:135`), after `evictChangedMarks` and after the new diff is set, collect paths of commented files and fetch their blob SHAs via `BlobSHAs`.
- [x] 4.2 Call `a.review.ReanchorComments(current, a.diffFiles)` and persist the result.
- [x] 4.3 Confirm `ReloadReview` (`backend/main.go:190`) does NOT call reconciliation (no git, no blobs).
- [x] 4.4 Integration-test against a temp repo: a commit that moves a commented line re-anchors it; a commit that removes the line marks it outdated; a revert recovers it.

## 5. Persistence and load-time upgrade

- [x] 5.1 Add a `Comment`-level (or `Anchors`) `UnmarshalJSON` that upgrades a legacy comment (line + context, no anchors) into a single empty-blob first anchor, mirroring the `MarkedFiles.UnmarshalJSON` precedent (`backend/model/model.go:70`).
- [x] 5.2 Ensure round-trip save/load preserves anchor history (`backend/storage.go`).
- [x] 5.3 Anchors-only clean break: stop writing the legacy `line_number`/`context_*` fields on save; anchors are the sole source of truth. Reads still upgrade legacy fields (5.1).
- [x] 5.4 Unit-test loading a legacy state file, a new state file, and a mixed file; assert the upgraded shape and that old files load without error.

## 6. Frontend: types and lookup

- [x] 6.1 Mirror the `Anchor` shape and `Anchors` field in `ts/core/types.ts` (Comment, ~13-23).
- [x] 6.2 Update line-number lookup (`ts/core/comments.ts`) to read the comment's current anchor line instead of a stored `line_number`.
- [x] 6.3 Add pure helpers mirroring the backend accessors (`isOutdated`, `currentLine`, `lastGoodContext`) and unit-test them.

## 7. Frontend: untethered rendering

- [x] 7.1 Add a render path that, for each outdated comment in a file, builds a read-only pseudo-hunk from its last good context and renders it at the top of the file view (above the live hunks).
- [x] 7.2 Ensure the pseudo-hunk has no line-expand affordance and carries the warning marker class.
- [x] 7.3 Keep the existing line-anchored path (`ts/render/diff.ts:appendCommentThread`, `ts/render/hunks.ts`) unchanged for non-outdated comments.
- [x] 7.4 Update mutation patching (`ts/render/mutate.ts`) so deleting an outdated comment removes its untethered pseudo-hunk, and so an add/edit/delete on an outdated comment patches the right subtree.

## 8. Styling

- [x] 8.1 Add a warning (yellow) border style for the outdated pseudo-hunk in `backend/assets/style.css` (comment section ~690-799), reusing existing comment/hunk styling where possible.
- [x] 8.2 Verify the outdated block is visually distinct from a live hunk and from a normal comment thread.

## 9. End-to-end verification

- [x] 9.1 Run the full Go test suite and the frontend build; fix regressions.
- [ ] 9.2 Manually verify in the app: comment a line, change the file (commit/agent edit), refresh — comment follows on a shift, goes outdated when the line is gone, recovers on revert. (Backed by integration tests in `main_test.go`; needs a human GUI pass — `release.install` + restart.)
- [ ] 9.3 Verify an outdated comment renders at the top of its file with the warning border and no expand control, and that deleting it removes the block. (Backed by `render_dom_test.ts`; needs a human GUI pass.)
- [ ] 9.4 Verify a pre-existing (legacy) state file opens correctly and its comments behave as before until the file changes. (Backed by `TestLoadReviewUpgradesLegacyComment`; needs a human GUI pass with a real old state file.)
