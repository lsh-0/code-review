## Context

The GUI renders a branch-vs-main diff and lets the reviewer leave line-anchored comments that persist
to a JSON file agents can edit. Three facts set up this change:

- **Expanded lines exist only in the DOM.** `spliceRevealed` fetches file lines and inserts DOM rows;
  the diff model (`state.diffFiles`) never learns of them. Expansion frontier state lives in
  `Affordance` objects held by DOM event closures. Comment mutations therefore *patch* the DOM
  (`mutate.ts`) because a re-render from the model would erase the revealed lines.
- **Comment semantics are implemented twice.** `ts/core/comments.ts` mirrors the Go model
  function-for-function, and `ts/core/types.ts` re-declares the anchor wire shape. The agent CLI
  (`backend/cli.go`) already avoids this: it sends derived `Line`/`Outdated` and keeps anchors
  private.
- **The reported bug is a symptom of both.** A comment on a revealed line captures an empty context
  (capture reads the model, which lacks the line), is stored adrift, and renders orphaned. Even with
  a good context it would re-orphan, because the reconcile corpus is diff lines only — and even with
  a good anchor it would be invisible on reopen, because hunks render collapsed.

The owner's decision at the review gate: fix the model, not the symptoms. The architecture review of
2026-07-29 (`docs/architecture-reviews/2026-07-29.md`) is the companion document; its direction is
folded in here so this design stands alone. Decisions carried unchanged from the replaced change
`anchor-comments-on-expanded-lines` (git b5df2f2): backend-side capture, full-file re-anchor corpus,
no anchor migration, adrift keeps meaning "content gone".

## Goals / Non-Goals

**Goals:**

- One model of what is on screen: a file's view is derived from state, and the DOM is disposable.
- A comment added to any visible line — hunk or revealed — anchors, survives reconciliation, and is
  visible again on reopen without manual expansion.
- A comment far from any hunk renders as its own small window, not via a giant contiguous reveal.
- The frontend contains no anchor semantics; one implementation lives in Go and serves GUI and CLI.

**Non-Goals:**

- Changing the matching algorithm, the anchor storage shape, or the review-state JSON. No migration.
- Virtualised rendering of very large regions (windows make regions small; windowed rendering remains
  the escape hatch if ever needed).
- Showing outside-hunk comments in the overview page (pre-existing gap, unchanged by this design —
  the derived `context` field makes the follow-up cheap; tracked as its own item).
- Persisting grown windows across app restarts.

## Decisions

### Decision 1: A file's view is a list of windows

A **window** is a contiguous inclusive range `[start, end]` of new-side line numbers. Per file the
frontend state holds:

- `windows: Window[]` — sorted, disjoint, non-adjacent (adjacency merges; see Decision 3).
- `fileLines: Map<number, string>` — fetched head-revision lines by number (sparse; filled by grow
  and by default comment windows), plus `totalLines` once known.

Rendering a window walks its range: where the range overlaps a hunk, the hunk's own lines are
emitted — including removed (old-side) lines interleaved at their positions, exactly as today —
and elsewhere the fetched file line is emitted as an ordinary context row. A line in a window whose
content is not yet fetched renders nothing; the fetch that created the window supplies it, so this
is a transient state, not a UI state.

The existing pure expand arithmetic (`ts/core/expand.ts`) seeds the window arithmetic; frontier and
boundary become window edges and gaps. The `Affordance` DOM-closure struct is deleted — grow state
*is* the window list.

### Decision 2: Default windows = hunks + comment islands

On (re)opening a file, and after every refresh, the window list is recomputed from scratch by a pure
function of (hunks, comment lines, total lines):

- one window per hunk, spanning the hunk's natural new-side range;
- one window per comment whose line falls outside all hunk windows: `line ± diffContextSize` (3),
  clamped to the file;
- the resulting set merged by the rule in Decision 3.

This is the owner's "a comment left on an expanded section becomes a new window", and it replaces
the previous design's uncapped nearest-hunk expansion: a comment 800 lines from a hunk costs a
7-line island, not an 800-line reveal. Multiple comments in one gap may produce one merged island.
Untethered comments (adrift, or anchored to a line past the end of the file) create no window and
keep their existing top-of-file presentation.

Grown windows **persist across file switches within a session** — the state outlives the DOM now,
which is a concrete win over today's behaviour (switching files silently discards expansion) — and
**reset to defaults on diff refresh**, per the owner's explicit rule. Refresh already invalidates
line numbering wholesale, so collapse is also the correct-by-construction choice.

### Decision 3: Windows merge when the gap between them becomes small

Two windows merge into one when the hidden gap between them drops below `windowMergeThreshold` = 3
lines (the ordinary diff context size): the sliver is revealed (fetched if needed) and the ranges
join. Consequences, all deliberate:

- The existing converging between-hunk merge (gap reaches zero) is the threshold's degenerate case;
  the separate sibling-linking machinery goes away.
- A comment island near a hunk merges into it at creation time, which reproduces the replaced
  design's "expand the nearest hunk" behaviour precisely where it was cheap, and diverges from it
  precisely where it was expensive.
- Merging is applied as a normalisation pass (`normaliseWindows`) after every window mutation, so
  it is a property of the model, not of any particular affordance: sorted, disjoint, gaps ≥
  threshold — an invariant suited to property tests, alongside "grow never shrinks a window" and
  "normalisation is idempotent".

Rendered between-window gaps keep today's affordance UX: a grow-down control at a window's bottom
edge, grow-up at the top, step of 20, and the visible-but-disabled terminal state at exhausted file
boundaries (an owner UX decision that stands).

### Decision 4: Anchor semantics live only in Go; surfaces get derived fields

`AddComment` narrows to `(filePath, content, lineNumber)`. The backend captures the context window
from the head-revision file body (already cached for expansion), using the reconciler's own
`captureRadius` and line source — creation and re-anchor capture become identical by construction,
and capture windows can no longer include removed lines (the pre-existing orphaning bug next door,
made structurally impossible rather than patched; no migration for old anchors that span removed
lines, accepted at the previous gate).

Comment payloads to the GUI carry derived fields, the same shape the CLI already derives: current
`line` (0 when unplaced), `outdated`, `context` (last good, for untethered rendering), and the
file's aggregate status where the surface needs it. Anchor history stays server-side. The frontend
deletes its mirror: `anchorIsAdrift`, `currentLineNumber`, `isOutdated`, `lastGoodContext`,
`commentRootLine`, `fileCommentStatus` and the `Anchor` wire type; threading helpers
(`rootComments`, `getReplies`, `threadAuthors`, `authorLabel`) remain, as they are genuinely view
logic.

### Decision 5: Reconciliation matches against the full file at the head revision

`reanchorComments` builds its corpus from `fileCache.get(SourceBranch, path)` — every line, numbered
1..N — instead of `diffLinesByPath`. `RecomputeDiff` resets the cache immediately before, so bodies
are fresh. Unreadable and binary files are skipped for the pass, leaving comments untouched rather
than adrift on transient failure. `model.DiffLineRef` and the matching logic are unchanged; only
the corpus contract widens. Fuzzy matching now scans whole files — only after an exact miss on a
changed blob; bounded-neighbourhood search is the ready fix if profiling ever demands one.

### Decision 6: Regions re-render from state; the DOM is never patched

Any change to a file's view — comment mutation, window grow/merge, refresh — re-renders that file's
region (its section in the combined view, the whole pane in the single-file view) wholesale from
state. Pane scroll is captured and restored around the render. Deleted as a consequence: the
patching half of `mutate.ts` (`patchLineThread`, `patchTopLevelBlock`, `patchOutdatedBlock`), the
mid-flight navigation guards the replaced design needed (an async fetch updates state; if the file
is no longer shown there is no region to render), and the five callback-bag interfaces
(`CommentActions`, `DiffCallbacks`, `OverviewCallbacks`, `MutationContext`, `ExpandContext`) —
render modules call the modal/mutation handlers directly, and renders are triggered from one place.
The `currentFile` getter/setter shim over `selectedFiles` goes in the same sweep.

In-page find re-applies its highlights after any region render via a single post-render hook — find
is the one feature that must annotate rendered output it does not own.

**Cost, stated plainly:** a mutation re-highlights its whole region (per-line `highlightLine`
instead of one thread patch). Regions are window-sized, so this is bounded; if a pathological file
makes it visible, memoising `highlightLine` by (content, language) is the ready answer. In-region
text selection does not survive a re-render; comment threads and expansion state do, because they
are model state.

## Risks / Trade-offs

- **Scope: a restructure and a bug fix in one change** → mitigated by task staging: the backend
  stages (capture, corpus, derived fields) land and are testable before the frontend model swap;
  the window model lands as pure core with tests before any render change. Each stage leaves the
  app working.
- **Existing anchors whose window spans a removed line may fail one match** (Decision 4; carried
  from the replaced change) → fuzzy matching usually absorbs it; a miss orphans with context intact
  and recovers by blob reuse. No migration.
- **Windows change what "expanded" means on refresh** → collapse-to-defaults is the owner's rule
  and matches the mental model of refresh as a re-derivation; a reviewer's in-flight growth is
  in-session state, not review state.
- **Find highlights are wiped by region re-renders** → one post-render re-apply hook; find is
  already resilient to full-page changes (it re-scans on each search).
- **A merged island can widen a region beyond expectation** (many comments in one gap) → merging is
  bounded by the comments' own spread; the merge threshold only ever closes 3-line slivers.

## Open Questions

1. **Overview islands.** With derived `context` on every comment, rendering outside-hunk comments
   in the overview as read-only pseudo-hunks becomes cheap — deliberately excluded here; follow-up
   item.
2. **The architecture-review snapshot is uncommitted.** `docs/architecture-reviews/2026-07-29.md`
   informs this design but is not in git; the reconcile gate tears the worktree down, so it is lost
   unless committed. This design stands alone either way — committing the snapshot is the owner's
   call.
