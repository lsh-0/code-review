## Why

Adding a comment to a line revealed by hunk expansion orphans the comment instantly: it renders
untethered at the top of the file with the outdated warning border instead of anchoring to the line.
The prior change (`anchor-comments-on-expanded-lines`, replaced by this one) traced the failure to
three independent layers and proposed patching each. The owner's review verdict, informed by the
architecture review of 2026-07-29 (`docs/architecture-reviews/`), is that the three layers share one
root: **the frontend's data model does not represent what is on screen.** Expanded lines exist only in
the DOM; the diff model, the re-anchor corpus, and the renderer each work from a picture that omits
them. Rather than compensating in three places, this change restructures the model so the bug cannot
be expressed.

## What Changes

- **Hunks become windows onto the file.** A file's view is a list of **windows** — contiguous ranges
  of new-side lines, each rendered from the diff's hunk lines where they overlap and from the fetched
  file body elsewhere. Windows can be **grown** (the expand affordances), **merge** when the gap
  between them becomes too small, and **collapse back to their default sizes on refresh**. The window
  list and the fetched lines live in frontend state, not in DOM closures.
- **A comment outside every hunk gets its own window.** Default windows are one per hunk plus one per
  such comment (the comment's line with the ordinary diff context around it). This replaces the prior
  design's uncapped contiguous expansion toward a distant comment: a comment far from any hunk becomes
  a small island, not a thousand-line reveal.
- **Anchor semantics move entirely behind the API.** The backend captures a comment's context window
  itself in `AddComment` (from the head-revision file body it already caches) and sends every surface
  *derived* placement — current line, outdated flag, last good context, file status — as the agent CLI
  already does. Anchor history stops crossing the bridge; the frontend's mirror of comment semantics
  (`ts/core/comments.ts` majority, the anchor wire types) is deleted along with the
  `captureContextRadius`/`captureRadius` two-language lockstep.
- **Re-anchoring matches against the whole file at the head revision**, not just the diff's lines, so
  a comment on unchanged code outside every hunk survives reconciliation. "Adrift" keeps its meaning:
  content genuinely gone from the file still orphans, and still renders untethered.
- **The render layer re-renders regions instead of patching the DOM.** Rendering a file section
  becomes a function of (windows, comments, fetched lines) → DOM, re-run per region on any change.
  The patching half of `ts/render/mutate.ts`, the DOM-closure affordance state, and the five
  callback-bag interfaces go; in-page find re-applies its highlights after a region render.

**This is a restructure folded with a bug fix, and materially larger than the change it replaces.**
The trade accepted by the owner: one coherent model instead of three coordinated patches, with the
bug and its whole class (model/DOM drift) removed rather than compensated for.

## Capabilities

### New Capabilities

None. The window model replaces the mechanics of existing capabilities rather than adding a surface.

### Modified Capabilities

- `diff-context-expansion`: rewritten around windows. Expansion affordances become window-edge grow
  controls; converging between-hunk expansion becomes window merging; a new requirement gives
  comments outside hunk windows their own default window; grown windows persist across file switches
  within a session and collapse to defaults on refresh. The binary-file, pure-rename, and browse
  requirements are untouched.
- `comment-reanchoring`: creation-time capture moves to the backend and draws from the file body;
  the re-anchor corpus becomes the file's full content at the head revision (modifying the scenario
  "Content outside the diff cannot be re-anchored", which states the opposite); a new requirement
  makes anchor history private to the backend, with all surfaces receiving derived placement fields.
- `frontend-architecture`: the view model becomes the single source of truth for what is on screen —
  the DOM is disposable output, regions re-render from state, and no view state lives only in the
  rendered tree.

## Impact

- **Frontend (TypeScript/Deno).** New pure window arithmetic in `ts/core` (seeded by the existing
  `expand.ts` logic); `ts/render/state.ts` gains per-file windows and fetched-line cache;
  `ts/render/hunks.ts` and `ts/render/expand.ts` are rewritten as window rendering;
  `ts/render/mutate.ts` patching, most of `ts/core/comments.ts`, the anchor types in
  `ts/core/types.ts`, the callback-bag interfaces, and the `currentFile` accessor shim are deleted;
  `ts/render/find.ts` gains a re-apply hook.
- **Backend (Go).** `AddComment` signature narrows to `(file, content, line)`; comment payloads to
  the GUI carry derived fields (as `backend/cli.go` already derives); `reanchorComments` builds its
  corpus from the cached full file bodies. `backend/model` logic is unchanged except doc contracts.
- **No storage migration.** The review-state JSON and anchor shapes on disk are untouched; existing
  anchors whose captured window spans a removed line are left to re-match fuzzily (accepted, no
  migration — carried from the replaced change).
- **Tests.** Pure window arithmetic gets unit and property tests; DOM tests simplify to
  state-in/tree-out; Go tests cover backend capture, derived payloads, and full-file reconciliation.
