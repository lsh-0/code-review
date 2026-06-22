## Context

Comments today carry a single new-side `LineNumber` plus three lines of captured context (`ContextBefore/Line/After`), with no record of the file content they were anchored against (`backend/model/model.go:21-31`). When the file changes mid-review the line number is reinterpreted against a different diff and the comment slides, mis-places, or disappears silently. The proposal replaces this with a per-comment history of blob-keyed anchors and a heuristic that either re-places a comment or marks it adrift.

This design reuses an existing, proven pattern. Marked files already survive content changes by storing the git blob SHA at mark time and reconciling on `RecomputeDiff`: `evictChangedMarks` (`backend/main.go:168`) fetches per-file SHAs via `BlobSHAs(repoPath, sourceBranch, paths)` (`backend/gitquery.go:34`) and hands them to a pure model method `Review.EvictChangedMarks(current map[path]sha)` (`backend/model/model.go:359`). Re-anchoring slots into the same call site with the same blob source and the same pure-method shape.

The diff the heuristic matches against is structured, not raw text: `[]DiffFile{ Hunks []DiffHunk{ Lines []DiffLine{ Content, NewLineNo, OldLineNo } } }` (`backend/diff_parser.go:17-36`). A "context window" is therefore a slice of `DiffLine.Content` strings, and re-anchoring is string-slice matching against the lines present in the recomputed diff.

## Goals / Non-Goals

**Goals:**
- Persist, per comment, a history of anchors keyed by file blob SHA: `{Blob, LineNumber, Context}`. `Anchors[0]` (creation) is guaranteed good.
- On `RecomputeDiff`, reconcile each comment against its file's current blob in two steps: **first** reuse an existing anchor if the current blob SHA already appears in the comment's history (exact-equality lookup, no matching); **only if that fails**, run exact-then-fuzzy matching against the **last good anchor's** context.
- Derive "outdated" (most-recent anchor is adrift) and "current line number" (most-recent anchor's line) as computed properties, never stored — keeping them orthogonal to `Status`.
- Recover an adrift comment when its file's content returns: a revert that restores a previously-seen blob recovers via SHA reuse (best case, no matching); a new-but-similar blob may recover via the last-good search. Recovery never fuzzy-searches older anchors' context.
- Render an outdated comment untethered at the top of its file view, showing its most-recent good context as a read-only pseudo-hunk with a warning border; deletion removes that context with it.
- Upgrade legacy comments (line + 3-line context, no anchors, no blob) in place on load, with no separate migration step.

**Non-Goals:**
- Diff-driven line remapping (using `git diff old..new` to compute exact line movements). The heuristic matches captured context only; commit-based tracking is a possible future feature and is why we key on blob, not commit.
- Fuzzy-searching older anchors' context. When a *new* blob requires the heuristic, the match target is the last good context only — the fuzzy search is not fanned out over every historical context window. (Exact SHA *reuse* does scan the whole history, but that is an equality lookup, not a search.)
- Anchoring a comment onto code that is not present in the recomputed diff. Comments render only against diff lines today; re-anchoring inherits that boundary — a comment whose context now lives in an unchanged region outside every hunk is adrift, not silently placed.
- Manual reviewer-driven re-anchoring UI. Re-anchoring is automatic.
- Touching the marks subsystem. Marks and anchors stay independent consumers of the same blob signal (see Decisions).

## Decisions

### 1. Anchor history keyed by blob SHA, not commit SHA

Each comment owns `Anchors []Anchor`, where:

```go
type Anchor struct {
    Blob       string   // git blob SHA of the file content this anchor was computed against
    LineNumber int      // new-side placement; 0 ⇒ adrift
    Context    []string // captured line contents around the placement; empty ⇒ adrift
}
```

An anchor is **adrift** iff it has no `Context` (equivalently, `LineNumber == 0`). A comment is **outdated** iff its most-recent anchor is adrift. Current line number = most-recent anchor's `LineNumber`.

*Why blob over commit:* keying on file content means commits that don't touch the file add no anchor (the blob is unchanged), so the history is bounded by actual content changes rather than recompute count — this dissolves the unbounded-growth concern without a collapse rule. It also makes a revert a cache hit: a restored blob already in the history is recovered by the SHA-reuse step (Decision 4) with no matching. This matches the `durable-file-marks` precedent exactly (`FileMark.Blob`, `model.go:57-60`), keeping the two subsystems consistent. The cost is that a blob SHA says *that* the file changed, not *how* — acceptable because we chose context matching over diff-driven remapping.

*Alternative considered — single frozen anchor + `outdated bool`:* simpler, but loses provenance, can't recover on revert, and pairs a boolean with a nullable payload that can desync. Rejected.

*Alternative considered — `Status` enum gains `outdated`:* conflates two orthogonal axes (reviewer decision vs. system placement); a comment can be both open and outdated. Rejected — this was the originating problem.

### 2. Outdated and current-line are derived, not stored

No `outdated` field, no top-level `LineNumber` as source of truth. Both are accessors over `Anchors`:
- `CurrentLineNumber()` → most-recent anchor's `LineNumber` (0 ⇒ adrift).
- `IsOutdated()` → most-recent anchor has empty `Context`.
- `LastGoodContext()` → context of the most recent anchor that has one (for the heuristic's match target and for untethered rendering).

This keeps the orthogonality structural: `Status` stays exactly as it is (active/resolved/ignored, reviewer-controlled), and nothing about re-anchoring can produce an impossible state. Frontend line lookup (`ts/core/comments.ts`) and `CommentRootLine` (`model.go:202`) read `CurrentLineNumber()` instead of a stored field.

### 3. Re-anchoring runs in `RecomputeDiff`, as a pure model method

Mirror `evictChangedMarks`. In `RecomputeDiff` (`backend/main.go:135`), after the new diff is built and marks are reconciled:
1. Collect the paths of files that have comments.
2. Fetch current blob SHAs with the existing `BlobSHAs(a.repoPath, a.review.SourceBranch, paths)`.
3. Call a pure `a.review.ReanchorComments(current map[path]sha, diff []DiffFile)` that, for each comment whose current-anchor blob differs from the file's current blob, runs the heuristic and appends an anchor.

Purity (inputs: current SHAs + parsed diff; output: mutated anchor lists) keeps it unit-testable without git, exactly as `EvictChangedMarks` is tested (`model/model_test.go:536+`). The diff is passed in because the heuristic matches against the lines present in it.

*Why here:* `RecomputeDiff` is already the single git-touching reconciliation point and already fetches blob SHAs for marks; re-anchoring shares both. `ReloadReview` (the cheap no-git reload) is deliberately left untouched.

### 4. Reconciliation: SHA-reuse first, then exact-then-fuzzy against the last good context

Per comment, against its file's current blob, the order is strict and short-circuits at the first success:

0. **Reuse (SHA equality):** if the current blob SHA already appears anywhere in the comment's anchor history, reuse that anchor's `{LineNumber, Context}` directly. No matching runs. This is the best case — it makes revert/toggle (A→B→A) stable and is cheap: an equality scan over a tiny list. A reused good anchor recovers an adrift comment; a reused adrift anchor stays adrift. (If the current blob equals the most-recent anchor's blob, the file did not change and there is nothing to do.)

Only when no stored anchor matches the current blob do we have genuinely new content, and only then does the heuristic run. Target = `LastGoodContext()` (a slice of line contents centred on the anchored line). Candidate positions = the new diff's lines for that file.

1. **Exact context match:** find a position where the last good context matches the new diff's line contents verbatim; anchor the line at the matched window's offset. Append a good anchor for the new blob.
2. **Fuzzy:** if no exact match, score each candidate window by similarity (line-set / ordered-line overlap) within a bounded search and take the best. Accept only if the best score ≥ a confidence threshold; append a good anchor.
3. **Adrift:** if nothing clears the threshold, append an adrift anchor (blob only).

**A re-anchor captures a *fresh* context window from the new content; it does not carry the old window forward.** The last good context is only the *search key* used to locate the line in the new diff. Once located, the new anchor records the window as it now reads in the new blob, with its own offset. So each anchor's context witnesses the blob it belongs to, and the next reconcile's `LastGoodContext()` returns the *latest* successful capture — the comment chases the code as it drifts (per Decision 1's last-good-chase intent), rather than matching every future revision against the original creation window. Carrying the creation window forward would silently freeze the search key and defeat the history.

**Anchoring at the captured offset, not the window centre.** The anchored line is not necessarily the centre of its window: capture clips at hunk edges, so a comment near the top of a hunk has fewer lines above it. Each anchor therefore stores `Offset`, the anchored line's index within its `Context`; matching returns `windowStart + offset`, and the fresh re-capture recomputes its own offset under the same edge-clipping. Without this, comments near a hunk boundary re-anchor off by the clip amount.

The threshold is the one genuinely judgement-heavy knob and is called out in Open Questions. The capture radius (`captureRadius`, mirrored on the frontend) and the similarity metric are design parameters; the proposal widened capture beyond 3 lines specifically to give fuzzy matching material.

*Why reuse-first:* exact SHA equality is the unambiguous best case (the content is literally one we placed before) and costs nothing, so it gates the heuristic entirely. *Why exact-context-before-fuzzy:* the common new-content case (a small edit elsewhere shifts the line) resolves by verbatim match with no fuzziness risk; fuzzy is the fallback that earns the "outdated" call when it fails.

### 5. Captured context width and what counts as a "line"

Capture a window of `DiffLine.Content` around the anchored line — both the centre line and N neighbours on each side, drawn from the hunk the comment sits in. Storing raw `Content` (not trimmed/normalised) keeps the captured pseudo-hunk faithful for untethered rendering. Matching may normalise (e.g. trim trailing whitespace) without changing what is stored.

### 6. Untethered rendering of outdated comments

An outdated comment renders at the **top of its file's view** (decided), as a read-only pseudo-hunk built from `LastGoodContext()`, with a warning (yellow) border and no line-expand affordance (there is no live hunk to expand into). The existing line-anchored render path (`ts/render/diff.ts:appendCommentThread`, keyed on `NewLineNo`) is bypassed for outdated comments; a sibling path renders the captured context block instead. Deleting the comment removes the comment and its captured context together — there is no other owner.

### 7. Marks and anchors stay independent

Both react to a file's blob SHA changing, but they answer different questions and have different owners: a mark is reviewer sign-off ("I've finished looking at this file"), an anchor is system-derived placement ("where does this comment's code live now"). They can legitimately diverge — an edit that moves code but leaves a commented line intact should re-anchor cleanly yet still unmark the file, because the reviewer has not re-reviewed the new state. Coupling them would be a category error. They remain separate consumers of the same `BlobSHAs` result within `RecomputeDiff`; neither knows about the other.

### 8. Legacy upgrade on load, no migration step

On load, a comment with no `Anchors` but a non-zero `LineNumber`/context is upgraded in place to a single `Anchors[0] = {Blob: "", LineNumber, Context: [before, line, after]}` — empty blob marking it legacy, exactly as a legacy bare-path mark carries an empty blob awaiting backfill (`model.go:55-56, 67-87`). This is done in the model's JSON unmarshalling (the `MarkedFiles.UnmarshalJSON` pattern, `model.go:70`), so old state files load with no separate migration command. The first `RecomputeDiff` then treats the empty-blob anchor like a legacy mark: adopt the file's current blob as the baseline (no spurious adrift) and proceed.

## Risks / Trade-offs

- **Fuzzy match places a comment on plausible-but-wrong code** → exact-first handles the common shift cleanly; fuzzy accepts only above a confidence threshold and falls to adrift otherwise. Marking adrift (visible, untethered, warning border) is the safe failure — better a clearly-flagged orphan than a silent mis-anchor, which is the status quo we're fixing.
- **Threshold too strict → comments go adrift on trivial edits; too loose → mis-anchors** → tune against real cases; expose as a single named constant so it is one place to adjust. Listed as an open question.
- **Capturing wider context grows the state file** → feature branches are short-lived and anchors are blob-deduped, so growth is bounded by distinct content versions, not recompute count. Acceptable for a local-first single-user store.
- **A comment's context now lives outside every hunk (unchanged region)** → it goes adrift by design (Non-Goals): re-anchoring only targets lines present in the diff, consistent with where comments can render at all. The reviewer still sees the comment, untethered, with its captured context.
- **Legacy comment whose empty-blob baseline never matched its real creation content** → first recompute adopts the current blob as baseline without re-matching (like legacy marks), so legacy comments keep their existing placement until the file actually changes again. No worse than today.
- **Frontend now has two render paths (anchored + untethered)** → the untethered path is additive and isolated to outdated comments; the existing `data-line` patching (`ts/render/mutate.ts`) is unchanged for anchored comments.

## Migration Plan

- Additive schema: `Anchor`/`Anchors` added; legacy `LineNumber`/`Context*` fields read by the upgrade path and subsumed into `Anchors[0]`.
- No migration command. Backward-compatible load via model `UnmarshalJSON` (the `MarkedFiles` precedent). Old state files open unchanged; comments behave as before until the first recompute after the file's content changes.
- Rollback: an older binary ignores unknown `Anchors` JSON and still reads `LineNumber`/context, so a state file written by the new binary degrades gracefully (it would lose anchor history but keep the last placement) — provided the upgrade keeps writing the legacy fields during a transition window. Whether to dual-write legacy fields for one release is an open question.

## Resolved Decisions

These were open during design and are now settled (recorded here for the archive):

- **Confidence threshold and similarity metric.** Jaccard overlap of trimmed, non-empty line sets, accepted at `>= 0.7` via the single `fuzzyMatchThreshold` constant.
- **Context window radius.** A fixed radius of 3 lines each side, captured from within the hunk and clipped at hunk/diff edges. The frontend (`captureContextRadius`) and backend re-capture (`captureRadius`) mirror this value.
- **Rollback stance.** Clean break to anchors-only: legacy `line_number`/`context_*` fields are read (and upgraded) on load but never written. No dual-write.
- **Recovery scope.** Last-good-only for v1: recovery uses the last good context (plus exact-blob reuse for reverts). Reaching back to `Anchors[0]` or fuzzy-searching older anchors when the last-good chain is adrift is a deliberate future feature, not part of this change.
