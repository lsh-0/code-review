## 1. Selection state

- [x] 1.1 In `ts/render/state.ts`, replace the single `currentFile` source of truth
  with an ordered `selectedFiles: string[]`; expose `currentFile` as the primary
  (last entry, or `""` when empty) so the file-list highlight and refresh paths keep
  working. Update `dom_test_setup.ts` to reset `selectedFiles`.
- [x] 1.2 Add a pure helper for the toggle/replace decision (given the current
  selection, a path, and whether the click is additive, return the next selection)
  so the modifier logic is unit-testable without the DOM. Never empty the set.

## 2. Selection wiring

- [x] 2.1 In `ts/render/filelist.ts`, pass the modifier state (`ctrlKey || metaKey`)
  from the file-item click through `onSelectFile`, and mark every file in the
  selection active (not just one).
- [x] 2.2 In `ts/main.ts`, give `selectFile` an `additive` parameter that applies the
  pure toggle/replace helper, keep the no-op fast-path only for a plain re-click of a
  single already-selected file, and update `setActiveItem` to mark all selected
  files.

## 3. Combined rendering

- [x] 3.1 In `ts/render/hunks.ts`, add `renderCombinedDiff(paths, cb)` that clears
  `#diff-content` and appends one labelled `.diff-file-section` (with `data-file`)
  per path, each a filename header plus that file's hunks via `renderFileHunks`.
  Keep the single-file `renderDiff` rendering identically to today.
- [x] 3.2 In `ts/main.ts`, ensure every selected file's hunks and comments are loaded
  (`ensureFileDiffs` + per-file comments) before rendering the combined view, and set
  the sticky `#current-file-name` to the path (one file) or a count ("N files").
- [x] 3.3 Move the browse control into each section header (per-file), and remove the
  single top browse button; add the small CSS to give `.diff-file-section` the
  stacked-section look (reusing the overview header styling).

## 4. Scope per-file updates

- [x] 4.1 In `ts/render/mutate.ts`, scope `patchLineThread` and `patchOutdatedBlock`
  to the mutated file's `[data-file]` section, falling back to `#diff-content` for
  the single-file (sectionless) view, so a recurring line number patches the right
  file.
- [x] 4.2 In `ts/main.ts` `performFullRefresh`, re-fetch and re-render every selected
  file (not just one) after a recompute.

## 5. Tests

- [x] 5.1 Unit-test the toggle/replace helper: plain click replaces, ctrl/cmd adds,
  re-ctrl/cmd removes, removing the last file is a no-op.
- [x] 5.2 DOM-test the combined view: two selected files render two `data-file`
  sections in order, each with its filename header and its hunks; a single selection
  renders identically to the existing single-file test.
- [x] 5.3 DOM-test that a comment mutation on one file's line patches only that
  file's section when a line number recurs across two stacked files.

## 6. Validate

- [x] 6.1 `deno task check` and `deno task test` pass.
- [x] 6.2 `openspec validate multi-select-combined-diff --strict` passes.
