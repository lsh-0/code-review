Staged so the app builds and passes tests after every group: backend first (derived fields arrive
alongside the existing payload, then the frontend swaps, then the old payload goes), pure window
arithmetic second, the render swap third, deletions last.

## 1. Backend: capture at creation, reconcile against the full file

- [x] 1.1 Add a helper in `backend/file_content.go` that captures a context window from a file body: given the body and a 1-based line number, return the window spanning `captureRadius` lines each side (clipped at the file's bounds) and the anchored line's offset. Empty window for a line outside the file. Unit-test interior, clipped-at-start, clipped-at-end, past-EOF, and single-line-file cases.
- [x] 1.2 Change `App.AddComment` to `(filePath, content string, lineNumber int)`: read the head-revision body via `a.fileCache`, capture with the 1.1 helper, pass to `AddCommentWithContext`. An unreadable body stores an empty context (existing adrift-on-creation behaviour). Update Go callers and tests; fixtures gain real file content.
- [x] 1.3 Replace `diffLinesByPath` with a full-file corpus: one `model.DiffLineRef` per line of `a.fileCache.get(SourceBranch, path)`, numbered from 1; skip unreadable paths and binary files. Update doc comments on the corpus builder, `reanchorComments`, and `model.DiffLineRef`.
- [x] 1.4 Reconcile tests in `backend/main_test.go`: a comment on an unchanged line outside every hunk survives a recompute that edits the file elsewhere; genuinely deleted content still goes adrift; an unreadable file leaves comments untouched. Add a larger-than-diff corpus case to `backend/model/reanchor_test.go`.
- [x] 1.5 Backend test pinning the bug: comment on a line outside every hunk via the new `AddComment`, assert the stored first anchor is good.

## 2. Backend: derived comment payloads

- [x] 2.1 Add derived fields to the comment payloads the GUI receives (`GetComments`, `GetCommentedFiles`, mutation results): current `line`, `outdated`, and `context` (last good), computed from the model as `backend/cli.go` already does — keeping the `anchors` field in place for now so the frontend keeps working until task 4.6 removes it.
- [x] 2.2 Go tests asserting the derived fields for an anchored, an outdated, a reply, and a top-level comment, and that GUI and CLI report the same `line`/`outdated` for the same comment.

## 3. Pure core: window arithmetic

- [x] 3.1 Add `ts/core/windows.ts`: a `Window` range type and pure functions — `defaultWindows(hunks, commentLines, totalLines)` (one window per hunk plus one per outside-hunk comment at ± `diffContextSize`, clamped), `growWindow(windows, index, direction, step)`, and `normaliseWindows` (sort, clamp, merge when gaps fall below the 3-line threshold). Reuse or absorb the arithmetic in `ts/core/expand.ts`.
- [x] 3.2 Add a pure function giving each window's rendered line plan: for a range, which sub-ranges come from which hunk (removed lines interleaved) and which from the fetched file lines, so the render layer only walks the plan.
- [x] 3.3 Unit tests in `ts/core/windows_test.ts`: hunk-only defaults, comment islands (mid-gap, near-hunk merging at creation, above first hunk, below last hunk, several comments merging into one island), growth (partial step, exhausting a boundary, converging merge), and untethered comments producing no window.
- [x] 3.4 Property tests over random hunk/comment/growth sequences: windows always sorted, disjoint, gaps ≥ threshold; growth never shrinks a window; `normaliseWindows` is idempotent.

## 4. Render layer: regions from state

- [x] 4.1 Extend `ts/render/state.ts` with per-file window state: `windows`, sparse fetched `fileLines`, `totalLines`. Defaults computed on first render of a file, kept across file switches, recomputed (collapsed) on refresh.
- [x] 4.2 Rewrite the file-region renderer (`ts/render/hunks.ts`) to walk the window plan from 3.2: windows, hunk headers where a window contains a hunk start, grow affordances at edges facing hidden lines (disabled at exhausted file boundaries — keep the visible-but-disabled UX), hidden lower-hunk headers inside merged windows, comment threads on their lines, untethered and top-level blocks as today.
- [x] 4.3 Wire growth: an affordance click computes the fetch range, calls `getFileLines`, stores lines and grown window in state, and re-renders the region. Default comment windows fetch their line ranges the same way on first render.
- [x] 4.4 Replace mutation patching with region re-render: `applyMutation` updates the comment cache and re-renders the affected file's region (overview rebuild unchanged), preserving pane scroll. Delete `patchLineThread`, `patchTopLevelBlock`, `patchOutdatedBlock`, and their DOM tests.
- [x] 4.5 Add a post-render hook so `ts/render/find.ts` re-applies active highlights after a region render.
- [x] 4.6 Swap the frontend to the derived comment fields: consume `line`/`outdated`/`context` from the payloads, delete the anchor-reading functions from `ts/core/comments.ts` (keep threading/authorship), delete the `Anchor` wire type, then remove `anchors` from the backend GUI payloads (completing 2.1) and delete `getLineContext`/`captureContextRadius` with the `AddComment` call-site update from task 1.2.
- [x] 4.7 Collapse the callback-bag interfaces (`CommentActions`, `DiffCallbacks`, `OverviewCallbacks`, `MutationContext`, `ExpandContext`) into direct imports of the modal/mutation handlers, and remove the `currentFile` getter/setter shim by fixing its call sites to use `selectedFiles`.
- [x] 4.8 DOM tests (`ts/render/render_dom_test.ts`): default windows render hunks with gaps; a comment island renders at its position with its thread; growth re-renders with the window grown; a converging grow merges windows and hides the lower header; a mutation re-render preserves window state; an async fetch for a no-longer-shown file changes nothing rendered; find highlights re-applied after a region render.

## 5. Verification

- [x] 5.1 `deno task test`, `go test ./...`, and both toolchains' formatter/linter.
- [x] 5.2 `openspec validate diff-file-windows`.
- [x] 5.3 By hand, the original report end-to-end: grow a window, comment on a revealed line, see it anchor in place; leave the file and return, see the comment in its own window (or merged into a near hunk) with context; refresh and see windows collapse to defaults with the comment island present.
- [x] 5.4 Regression sweep of the touched surfaces: existing hunk-line comments still place and mutate; outdated comments render untethered; the overview still renders; the agent CLI output is unchanged; multi-file combined view re-renders per section.
