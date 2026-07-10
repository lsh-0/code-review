## Context

Selection today is a single string. `state.currentFile` holds the open file;
`selectFile` clears every `.active` item, marks one, sets `#current-file-name`, and
calls `renderDiff(filePath)` which clears `#diff-content` and renders that one
file's hunks (`ts/main.ts:88`, `ts/render/hunks.ts:30`). Re-selecting the current
file is a no-op so scroll and expanded context survive.

The data side already composes for several files. Per-file hunks load lazily via
`ensureFileDiff`, and `ensureFileDiffs` already fetches many concurrently
(`ts/render/load.ts:64`, `:81`). The overview pane already stacks one section per
file: for each commented file it builds a `.overview-file` section with a header and
calls `renderFileHunks(section, path, cb, true)` (`ts/render/overview.ts:46`). The
combined multi-select view is that same stacked-section shape, but in full
(non-`overviewOnly`) render mode and over the *selected* files rather than the
commented ones.

## Goals / Non-Goals

**Goals:**
- An ordered, multi-file selection model: plain click selects one, ctrl/cmd-click
  toggles, the set never empties while a file is shown.
- A combined diff pane that stacks one labelled section per selected file, each
  rendering that file's full diff (hunks, expand affordances, comment threads).
- Per-file controls and comment mutations correctly scoped when several files are
  shown.
- A single-file selection renders and behaves exactly as today.

**Non-Goals:**
- Reordering the selection by drag, or any selection-persistence across reloads.
  Selection is in-session, like grouping and filter.
- Selecting files via the overview or via keyboard range-select (shift-click).
  Only plain and ctrl/cmd modifier clicks from the file list.
- A combined view in the overview pane. The overview keeps its comment-only layout.

## Decisions

### 1. Keep `currentFile` as the anchor; add an ordered `selectedFiles`

Rather than replace `currentFile` outright, `selectedFiles: string[]` becomes the
source of truth for what renders, and `currentFile` is kept as the *primary* (the
last-toggled file). This is the smallest change that preserves the two places that
legitimately need a single "current" notion:

- `filelist.ts` highlights the selection and falls back to the first file when the
  selection is empty (`ts/render/filelist.ts:98`, `:129`).
- `performFullRefresh` re-fetches and re-renders the open surface after a git
  recompute (`ts/main.ts:181`).

`currentFile` is derived as the last entry of `selectedFiles` (or `""` when empty).
The file-list active-mark logic changes from "equals `currentFile`" to "is in
`selectedFiles`". An empty `selectedFiles` with `overviewActive` false means "no
file yet", the same state `currentFile === ""` meant before.

### 2. A plain click resets; ctrl/cmd-click toggles

`selectFile(path, additive)` carries the modifier. `additive` false sets
`selectedFiles = [path]`. `additive` true toggles: append when absent, remove when
present — unless removing would empty the set, in which case it is a no-op (the spec
guarantees ≥1 file while a file is shown). The modifier is read from the click event
in `filelist.ts` (`ev.ctrlKey || ev.metaKey`) and passed through the callback.

The no-op fast-path that today skips re-rendering the already-current file
(`filePath === state.currentFile`) is kept for the plain-click single-file case so
scroll and expanded context survive a re-click; an additive click always re-renders.

### 3. Render the combined view by stacking per-file sections

A new `renderCombinedDiff(paths, cb)` clears `#diff-content` and, for each path,
appends a `.diff-file-section` containing a header (filename + browse button) and
that file's hunks via the existing `renderFileHunks(section, path, cb, false)`. The
single-file `renderDiff` becomes the one-path case of this (a single section with no
extra chrome, to keep the single-file DOM identical). The section header reuses the
overview's header structure so the existing `.overview-file-header` styling and the
per-file browse button carry over; a small amount of CSS aliases the combined
section to that look.

Because `renderFileHunks` already takes the parent element and the file path, no
change to the hunk/affordance/thread rendering is needed — it is called once per
section with that section as parent.

### 4. Scope comment-mutation patching to a file's section

`patchLineThread` and `patchOutdatedBlock` query `#diff-content` for
`.comment-thread[data-line="N"]` and `.diff-line[data-line="N"]`
(`ts/render/mutate.ts:85`, `:55`). With several files stacked, line number N can
recur, so these queries must be scoped to the mutated file's section. Each section
carries a `data-file` attribute; the patch functions resolve the section by
`[data-file="<path>"]` and query within it, falling back to `#diff-content` itself
when only the single-file (sectionless) view is present. This keeps the single-file
path byte-for-byte and makes the multi-file path unambiguous.

### 5. The sticky header summarises; per-file labels live in the sections

The single sticky `#current-file-name` no longer carries the only filename. It shows
the file's path for a single-file selection (today's behaviour) and a count
("N files") for a multi-file selection. The authoritative per-file label lives in
each section header, so every stacked file is labelled. The single top browse button
is removed in favour of the per-section browse buttons, matching the overview.

## Risks / Trade-offs

- **Re-render cost.** A combined view re-renders all selected sections on a refresh.
  Selections are small (a reviewer holds a handful of files), and rendering is the
  same per-file work already done singly, so the cost is bounded and acceptable.
- **Scroll preservation across an additive click.** Adding a file re-renders the
  pane, so the existing sections' scroll is reset. This matches the single-file
  behaviour on selection change and is acceptable; preserving per-section scroll
  across additions is a non-goal.

## Migration

No data or backend migration. Selection state is in-session only. The single-file
view is preserved exactly, so existing DOM tests for the single-file diff continue
to pass unchanged.
