## Why

A binary file cannot be diffed line-by-line, so the diff body for one is empty. An
empty pane reads as a bug — the reviewer cannot tell whether the file is unchanged,
failed to load, or simply has no textual diff. The placeholder should state the
reason plainly so the empty body is understood as expected, not broken.

The existing `diff-context-expansion` spec already records that binary files are
listed but not rendered, but it specifies only "a plain placeholder (for example
'binary file')" — a bare label that does not explain *why* there is no diff. This
change tightens that requirement to mandate an explanatory placeholder.

## What Changes

- The binary-file placeholder SHALL explain that the file cannot be diffed, rather
  than merely labelling it as binary. The rendered text is "binary file, cannot
  diff".
- No change to detection (git's `Binary files ... differ` marker), navigation
  (binary files still listed), or the suppression of hunks, expansion affordances,
  and blob fetches. Those behaviours are already specified and already implemented.

This is a wording/contract refinement of an existing requirement; there is no new
runtime capability and no breaking change.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `diff-context-expansion`: the "Binary files are listed but not rendered"
  requirement changes from permitting any plain label to requiring an explanatory
  placeholder that states the file cannot be diffed.

## Impact

- Code already on `master` and unchanged by this proposal:
  - `backend/diff_parser.go` — detects the `Binary files ... differ` marker and
    sets `DiffFile.Binary`.
  - `ts/render/hunks.ts` — renders the `.binary-placeholder` with the text
    "binary file, cannot diff".
  - `ts/render/render_dom_test.ts` — asserts the exact placeholder text.
- This change records the decision in the spec; it does not require further code
  changes, since the implementation already matches the tightened requirement.
