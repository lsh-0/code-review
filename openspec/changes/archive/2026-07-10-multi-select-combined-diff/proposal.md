## Why

Reviewing a change often means reading several files together — a function and its
caller, an interface and its implementation, a test and the code it covers.
Today selection is single-file: the file list opens exactly one file's diff at a
time, so comparing two files means clicking back and forth and losing scroll
position each time. A reviewer cannot hold two related diffs on screen at once.

The render layer already composes for several files: per-file hunks load lazily
(`ensureFileDiff`) and the overview pane already stacks one section per file
(a header followed by that file's hunks via `renderFileHunks`). The missing piece
is a selection model that holds more than one file and a combined view that
renders the selected files as one stacked pane.

## What Changes

- The file list SHALL support selecting more than one file. A plain click selects
  a single file (today's behaviour); a ctrl/cmd-click toggles a file in or out of
  the selection without clearing the rest.
- The diff pane SHALL render every selected file as one stacked view: each file
  gets its own filename header followed by that file's full diff (hunks, expand
  affordances, and comment threads), in the order the files were selected.
- The per-file filename label moves from the single sticky `#current-file-name`
  element into each stacked section's header, so every section is labelled. The
  sticky header summarises the selection ("N files") when more than one file is
  selected.
- Comment mutations, expand affordances, and the per-file browse control SHALL act
  on the correct file's section when several are shown, rather than assuming a
  single file occupies the pane.

## Capabilities

### New Capabilities

- `multi-file-selection`: selecting multiple files into one combined, stacked diff
  pane, and the selection model (ordered set, plain vs ctrl/cmd-click) behind it.

### Modified Capabilities

None. The combined view reuses the existing diff-context-expansion and
comment-reanchoring rendering unchanged; this change adds the multi-file selection
capability on top of them.

## Impact

- `ts/render/state.ts` — selection state changes from a single `currentFile`
  string to an ordered `selectedFiles` list (with `currentFile` retained as the
  primary/anchor for the file-list highlight and refresh paths).
- `ts/main.ts` — `selectFile` gains a ctrl/cmd-toggle path; the active-item and
  header logic mark every selected file and summarise the selection.
- `ts/render/hunks.ts` — a combined renderer stacks one labelled section per
  selected file.
- `ts/render/filelist.ts` — the file-list item highlights every selected file and
  passes the modifier key through its click handler.
- `ts/render/mutate.ts` — comment-mutation patching is scoped to a file's section
  so a recurring line number across files patches the right one.
- `ts/render/render_dom_test.ts` — new DOM tests for the combined view and the
  selection model.
